package tech.easyp.mcp.runtime.oauth;

import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.crypto.RSASSAVerifier;
import com.nimbusds.jose.jwk.JWK;
import com.nimbusds.jose.jwk.KeyUse;
import com.nimbusds.jose.jwk.RSAKey;
import com.nimbusds.jwt.JWTClaimsSet;
import com.nimbusds.jwt.SignedJWT;
import java.io.IOException;
import java.io.InputStream;
import java.math.BigInteger;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.security.interfaces.RSAPublicKey;
import java.time.Duration;
import java.time.Instant;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CompletionException;
import java.util.concurrent.TimeUnit;
import java.util.function.Supplier;

/**
 * RS256 verifier ported from mcpruntime/jwks.go: HTTPS-only JWKS (redirects
 * included) with a five-minute cache, bounded staleness, unknown key ids
 * refreshed at most once a minute, exponential backoff and a single refresh in
 * flight. Signatures and keys are handled by nimbus-jose-jwt.
 */
public final class JWKSTokenVerifier implements TokenVerifier {
  static final Duration CACHE_TTL = Duration.ofMinutes(5);
  static final Duration REFRESH_TIMEOUT = Duration.ofSeconds(5);
  static final Duration UNKNOWN_KEY_INTERVAL = Duration.ofMinutes(1);
  static final Duration RETRY_MIN = Duration.ofSeconds(1);
  static final Duration RETRY_MAX = Duration.ofMinutes(1);
  static final Duration DEFAULT_MAX_STALE = Duration.ofHours(1);
  static final int MAX_JWKS_BYTES = 1 << 20;
  static final int MAX_REDIRECTS = 10;
  static final Duration LEEWAY = Duration.ofSeconds(10);

  /**
   * Verifier settings. maxStale is how long a stale key set stays usable while
   * refreshes fail: negative means the TTL only, zero one hour.
   */
  public record Config(String jwksUrl, String issuer, String audience, Duration maxStale, HttpClient httpClient, Supplier<Instant> clock) {
  }

  public static Config config(String jwksUrl, String issuer, String audience) {
    return new Config(jwksUrl, issuer, audience, Duration.ZERO, null, null);
  }

  private final URI jwksUrl;
  private final String issuer;
  private final String audience;
  private final Duration trustWindow;
  private final HttpClient httpClient;
  private final Supplier<Instant> clock;

  private final Object lock = new Object();
  private Map<String, RSAPublicKey> keys = Map.of();
  private Instant expires = Instant.MIN;
  private Instant hardExpires = Instant.MIN;
  private Instant retryAt = Instant.MIN;
  private Duration retryDelay = Duration.ZERO;
  private Exception lastError;
  private Instant lastUnknownRefresh;
  private CompletableFuture<Void> refreshing;
  private int fetches;

  public JWKSTokenVerifier(Config config) {
    this.jwksUrl = requireHttps(config.jwksUrl(), "JWKS URL");
    requireHttps(config.issuer(), "issuer");
    if (config.audience() == null || config.audience().isEmpty()) {
      throw new IllegalArgumentException("mcpruntime: JWT audience is required");
    }
    this.issuer = config.issuer();
    this.audience = config.audience();
    Duration maxStale = config.maxStale() == null ? Duration.ZERO : config.maxStale();
    if (maxStale.isNegative()) {
      this.trustWindow = CACHE_TTL;
    }
    else if (maxStale.isZero()) {
      this.trustWindow = DEFAULT_MAX_STALE;
    }
    else {
      this.trustWindow = maxStale.compareTo(CACHE_TTL) < 0 ? CACHE_TTL : maxStale;
    }
    this.httpClient = config.httpClient() != null
        ? config.httpClient()
        : HttpClient.newBuilder().connectTimeout(REFRESH_TIMEOUT).followRedirects(HttpClient.Redirect.NEVER).build();
    this.clock = config.clock() == null ? Instant::now : config.clock();
  }

  private static URI requireHttps(String raw, String what) {
    try {
      URI uri = new URI(raw);
      if ("https".equals(uri.getScheme()) && uri.getHost() != null && !uri.getHost().isEmpty()
          && uri.getRawFragment() == null && uri.getRawUserInfo() == null) {
        return uri;
      }
    }
    catch (Exception ignored) {
      // Reported below with the field name.
    }
    throw new IllegalArgumentException("mcpruntime: " + what + " must be a valid HTTPS URL");
  }

  @Override
  public AccessToken verify(String token) {
    try {
      SignedJWT jwt = SignedJWT.parse(token);
      if (!JWSAlgorithm.RS256.equals(jwt.getHeader().getAlgorithm())) {
        return null;
      }
      String kid = jwt.getHeader().getKeyID();
      if (kid == null || kid.isEmpty()) {
        return null;
      }
      if (!jwt.verify(new RSASSAVerifier(key(kid)))) {
        return null;
      }
      JWTClaimsSet claims = jwt.getJWTClaimsSet();
      Instant now = clock.get();
      if (!issuer.equals(claims.getIssuer()) || claims.getAudience() == null || !claims.getAudience().contains(audience)) {
        return null;
      }
      if (claims.getExpirationTime() == null || !now.minus(LEEWAY).isBefore(claims.getExpirationTime().toInstant())) {
        return null;
      }
      if (claims.getNotBeforeTime() != null && now.plus(LEEWAY).isBefore(claims.getNotBeforeTime().toInstant())) {
        return null;
      }
      if (claims.getIssueTime() != null && now.plus(LEEWAY).isBefore(claims.getIssueTime().toInstant())) {
        return null;
      }
      List<String> scopes = scopes(claims.getClaims());
      if (scopes == null) {
        return null;
      }
      String subject = claims.getSubject() == null ? "" : claims.getSubject();
      Object client = claims.getClaim("client_id") != null ? claims.getClaim("client_id") : claims.getClaim("azp");
      return new AccessToken(
          token,
          client == null ? subject : String.valueOf(client),
          subject,
          scopes,
          claims.getExpirationTime().toInstant(),
          nonNull(claims.getClaims()));
    }
    catch (Exception error) {
      return null;
    }
  }

  private static Map<String, Object> nonNull(Map<String, Object> claims) {
    Map<String, Object> out = new HashMap<>();
    claims.forEach((name, value) -> {
      if (value != null) {
        out.put(name, value);
      }
    });
    return out;
  }

  private static List<String> scopes(Map<String, Object> claims) {
    if (claims.get("scope") instanceof String scope) {
      return fields(scope);
    }
    Object scp = claims.get("scp");
    if (scp instanceof List<?> list) {
      List<String> scopes = new ArrayList<>();
      for (Object value : list) {
        if (!(value instanceof String text)) {
          return null;
        }
        scopes.add(text);
      }
      return scopes;
    }
    if (scp instanceof String text) {
      return fields(text);
    }
    return List.of();
  }

  private static List<String> fields(String value) {
    List<String> out = new ArrayList<>();
    for (String field : value.trim().split("\\s+")) {
      if (!field.isEmpty()) {
        out.add(field);
      }
    }
    return out;
  }

  RSAPublicKey key(String kid) throws Exception {
    while (true) {
      CompletableFuture<Void> wait;
      synchronized (lock) {
        Instant now = clock.get();
        RSAPublicKey key = keys.get(kid);
        if (key != null && now.isBefore(hardExpires)) {
          if (!now.isBefore(expires) && refreshing == null && !now.isBefore(retryAt)) {
            startRefreshLocked();
          }
          return key;
        }
        if (refreshing == null) {
          if (now.isBefore(retryAt) && lastError != null) {
            throw lastError;
          }
          if (key == null && now.isBefore(expires)) {
            boolean fresh = Duration.between(now, expires).compareTo(CACHE_TTL.minus(UNKNOWN_KEY_INTERVAL)) > 0;
            boolean throttled = lastUnknownRefresh != null && Duration.between(lastUnknownRefresh, now).compareTo(UNKNOWN_KEY_INTERVAL) < 0;
            if (fresh || throttled) {
              throw new IllegalStateException("unknown JWT key id");
            }
          }
          if (key == null) {
            lastUnknownRefresh = now;
          }
          startRefreshLocked();
        }
        wait = refreshing;
      }
      if (wait == null) {
        continue;
      }
      try {
        wait.join();
      }
      catch (CompletionException ignored) {
        // The refresh records its own failure.
      }
    }
  }

  private void startRefreshLocked() {
    // The timeout bounds the whole refresh, body included, as http.Client.Timeout does in Go.
    CompletableFuture<Void> refresh = CompletableFuture.supplyAsync(() -> {
          try {
            return fetch();
          }
          catch (Exception error) {
            throw new CompletionException(error);
          }
        })
        .orTimeout(REFRESH_TIMEOUT.toMillis(), TimeUnit.MILLISECONDS)
        .handle((fetched, error) -> {
          finishRefresh(fetched, error);
          return null;
        });
    // A refresh that already finished on this thread has cleared the state.
    refreshing = refresh.isDone() ? null : refresh;
  }

  private void finishRefresh(Map<String, RSAPublicKey> fetched, Throwable error) {
    while (error instanceof CompletionException && error.getCause() != null) {
      error = error.getCause();
    }
    synchronized (lock) {
      Instant now = clock.get();
      if (error == null) {
        keys = fetched;
        expires = now.plus(CACHE_TTL);
        hardExpires = now.plus(trustWindow);
        retryDelay = Duration.ZERO;
        retryAt = Instant.MIN;
        lastError = null;
      }
      else {
        retryDelay = retryDelay.isZero() ? RETRY_MIN : min(retryDelay.multipliedBy(2), RETRY_MAX);
        retryAt = now.plus(retryDelay);
        lastError = error instanceof Exception exception ? exception : new IOException(error);
      }
      refreshing = null;
    }
  }

  private static Duration min(Duration a, Duration b) {
    return a.compareTo(b) <= 0 ? a : b;
  }

  private Map<String, RSAPublicKey> fetch() throws Exception {
    synchronized (lock) {
      fetches++;
    }
    URI url = jwksUrl;
    HttpResponse<InputStream> response = null;
    for (int redirects = 0; ; redirects++) {
      HttpRequest request = HttpRequest.newBuilder(url).timeout(REFRESH_TIMEOUT).header("Accept", "application/json").GET().build();
      response = httpClient.send(request, HttpResponse.BodyHandlers.ofInputStream());
      int status = response.statusCode();
      if (status < 300 || status >= 400 || status == 304) {
        break;
      }
      response.body().close();
      String location = response.headers().firstValue("Location").orElse(null);
      if (location == null || redirects >= MAX_REDIRECTS) {
        throw new IOException("JWKS endpoint returned HTTP " + status);
      }
      url = url.resolve(location);
      if (!"https".equals(url.getScheme())) {
        throw new IOException("JWKS redirect to an insecure scheme is forbidden");
      }
    }
    try (InputStream body = response.body()) {
      if (response.statusCode() != 200) {
        throw new IOException("JWKS endpoint returned HTTP " + response.statusCode());
      }
      byte[] bytes = body.readNBytes(MAX_JWKS_BYTES + 1);
      if (bytes.length > MAX_JWKS_BYTES) {
        throw new IOException("JWKS response exceeds 1 MiB");
      }
      return usableKeys(new String(bytes, StandardCharsets.UTF_8));
    }
  }

  static Map<String, RSAPublicKey> usableKeys(String document) throws IOException {
    Object parsed = Json.parse(document, false);
    Map<String, RSAPublicKey> keys = new HashMap<>();
    if (parsed instanceof Map<?, ?> root && root.get("keys") instanceof List<?> list) {
      for (Object entry : list) {
        RSAPublicKey key = usableKey(entry);
        if (key != null) {
          keys.put((String) ((Map<?, ?>) entry).get("kid"), key);
        }
      }
    }
    if (keys.isEmpty()) {
      throw new IOException("JWKS response contains no usable RS256 signing keys");
    }
    return keys;
  }

  private static RSAPublicKey usableKey(Object raw) {
    if (!(raw instanceof Map<?, ?> jwk)) {
      return null;
    }
    if (!(jwk.get("kid") instanceof String kid) || kid.isEmpty() || !"RSA".equals(jwk.get("kty"))) {
      return null;
    }
    Object alg = jwk.get("alg");
    Object use = jwk.get("use");
    if (!(alg == null || "".equals(alg) || "RS256".equals(alg)) || !(use == null || "".equals(use) || "sig".equals(use))) {
      return null;
    }
    if (!(jwk.get("n") instanceof String n) || !(jwk.get("e") instanceof String e) || n.contains("=") || e.contains("=")) {
      return null;
    }
    try {
      Map<String, Object> plain = new HashMap<>();
      plain.put("kty", "RSA");
      plain.put("kid", kid);
      plain.put("n", n);
      plain.put("e", e);
      RSAKey key = JWK.parse(plain).toRSAKey();
      byte[] exponent = key.getPublicExponent().decode();
      if (exponent.length < 1 || exponent.length > 4) {
        return null;
      }
      BigInteger publicExponent = new BigInteger(1, exponent);
      if (publicExponent.compareTo(BigInteger.valueOf(3)) < 0 || !publicExponent.testBit(0) || key.size() < 2048) {
        return null;
      }
      if (key.getKeyUse() != null && key.getKeyUse() != KeyUse.SIGNATURE) {
        return null;
      }
      return key.toRSAPublicKey();
    }
    catch (Exception error) {
      return null;
    }
  }

  // Test hooks mirroring the Go cache tests.
  int fetches() {
    synchronized (lock) {
      return fetches;
    }
  }

  Duration retryDelay() {
    synchronized (lock) {
      return retryDelay;
    }
  }

  Instant retryAt() {
    synchronized (lock) {
      return retryAt;
    }
  }

}
