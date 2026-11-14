package tech.easyp.mcp.runtime.oauth;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;

import com.nimbusds.jose.JOSEObjectType;
import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.JWSHeader;
import com.nimbusds.jose.crypto.RSASSASigner;
import com.nimbusds.jose.jwk.RSAKey;
import com.nimbusds.jose.jwk.gen.RSAKeyGenerator;
import com.nimbusds.jwt.JWTClaimsSet;
import com.nimbusds.jwt.SignedJWT;
import com.sun.net.httpserver.HttpsConfigurator;
import com.sun.net.httpserver.HttpsServer;
import java.io.FileInputStream;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.net.http.HttpClient;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.KeyStore;
import java.time.Duration;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Date;
import java.util.List;
import java.util.Map;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;
import java.util.function.Consumer;
import javax.net.ssl.KeyManagerFactory;
import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManagerFactory;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class JWKSTokenVerifierTest {
  static final String ISSUER = "https://issuer.example.com";
  static final String AUDIENCE = "https://mcp.example.com/mcp";

  @TempDir
  static Path directory;

  static HttpsServer server;
  static SSLContext clientContext;
  static String url;
  static RSAKey key;
  static final AtomicReference<String> document = new AtomicReference<>();
  static final AtomicInteger status = new AtomicInteger(200);
  static final AtomicReference<String> redirect = new AtomicReference<>();
  static final AtomicInteger requests = new AtomicInteger();

  Instant[] clock;

  @BeforeAll
  static void startServer() throws Exception {
    Path keystore = directory.resolve("server.p12");
    String keytool = Path.of(System.getProperty("java.home"), "bin", "keytool").toString();
    Process process = new ProcessBuilder(
        keytool, "-genkeypair", "-alias", "server", "-keyalg", "RSA", "-keysize", "2048",
        "-dname", "CN=localhost", "-ext", "san=dns:localhost", "-validity", "1",
        "-storetype", "PKCS12", "-keystore", keystore.toString(), "-storepass", "changeit", "-keypass", "changeit")
        .redirectErrorStream(true)
        .start();
    process.getInputStream().readAllBytes();
    assertEquals(0, process.waitFor());

    KeyStore store = KeyStore.getInstance("PKCS12");
    try (FileInputStream in = new FileInputStream(keystore.toFile())) {
      store.load(in, "changeit".toCharArray());
    }
    KeyManagerFactory keys = KeyManagerFactory.getInstance(KeyManagerFactory.getDefaultAlgorithm());
    keys.init(store, "changeit".toCharArray());
    SSLContext serverContext = SSLContext.getInstance("TLS");
    serverContext.init(keys.getKeyManagers(), null, null);
    TrustManagerFactory trust = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());
    trust.init(store);
    clientContext = SSLContext.getInstance("TLS");
    clientContext.init(null, trust.getTrustManagers(), null);

    server = HttpsServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
    server.setHttpsConfigurator(new HttpsConfigurator(serverContext));
    server.createContext("/jwks.json", exchange -> {
      requests.incrementAndGet();
      if (redirect.get() != null) {
        exchange.getResponseHeaders().add("Location", redirect.get());
        exchange.sendResponseHeaders(302, -1);
        exchange.close();
        return;
      }
      byte[] body = document.get().getBytes(StandardCharsets.UTF_8);
      exchange.getResponseHeaders().add("Content-Type", "application/json");
      exchange.sendResponseHeaders(status.get(), body.length);
      try (OutputStream out = exchange.getResponseBody()) {
        out.write(body);
      }
    });
    server.start();
    url = "https://localhost:" + server.getAddress().getPort() + "/jwks.json";
    key = new RSAKeyGenerator(2048).keyID("k1").generate();
  }

  @AfterAll
  static void stopServer() {
    server.stop(0);
  }

  @BeforeEach
  void reset() {
    document.set(jwks(key.toPublicJWK().toJSONObject()));
    status.set(200);
    redirect.set(null);
    requests.set(0);
    clock = new Instant[] {Instant.now()};
  }

  @SafeVarargs
  static String jwks(Map<String, Object>... keys) {
    List<String> entries = new ArrayList<>();
    for (Map<String, Object> jwk : keys) {
      List<String> fields = new ArrayList<>();
      jwk.forEach((name, value) -> fields.add(Json.quote(name) + ":" + Json.quote(String.valueOf(value))));
      entries.add("{" + String.join(",", fields) + "}");
    }
    return "{\"keys\":[" + String.join(",", entries) + "]}";
  }

  JWKSTokenVerifier verifier(Duration maxStale) {
    HttpClient client = HttpClient.newBuilder().sslContext(clientContext).followRedirects(HttpClient.Redirect.NEVER).build();
    return new JWKSTokenVerifier(new JWKSTokenVerifier.Config(url, ISSUER, AUDIENCE, maxStale, client, () -> clock[0]));
  }

  static String token(RSAKey signer, String kid, Consumer<JWTClaimsSet.Builder> customize) throws Exception {
    JWTClaimsSet.Builder claims = new JWTClaimsSet.Builder()
        .issuer(ISSUER)
        .audience(AUDIENCE)
        .subject("user-123")
        .expirationTime(Date.from(Instant.now().plusSeconds(600)));
    customize.accept(claims);
    SignedJWT jwt = new SignedJWT(new JWSHeader.Builder(JWSAlgorithm.RS256).keyID(kid).type(JOSEObjectType.JWT).build(), claims.build());
    jwt.sign(new RSASSASigner(signer));
    return jwt.serialize();
  }

  static String token(Consumer<JWTClaimsSet.Builder> customize) throws Exception {
    return token(key, "k1", customize);
  }

  @Test
  void validTokenYieldsScopesAndExpiry() throws Exception {
    JWKSTokenVerifier verifier = verifier(Duration.ZERO);
    AccessToken access = verifier.verify(token(claims -> claims.claim("scope", "reports:read reports:write")));
    assertNotNull(access);
    assertEquals(List.of("reports:read", "reports:write"), access.scopes());
    assertEquals("user-123", access.subject());
    assertNotNull(access.expiresAt());
    assertEquals(List.of("a", "b"), verifier.verify(token(claims -> claims.claim("scp", List.of("a", "b")))).scopes());
  }

  @Test
  void rejectsWrongClaimsAndKeys() throws Exception {
    JWKSTokenVerifier verifier = verifier(Duration.ZERO);
    RSAKey other = new RSAKeyGenerator(2048).keyID("k1").generate();
    List<String> tokens = List.of(
        token(claims -> claims.issuer("https://evil.example.com")),
        token(claims -> claims.audience("https://other.example.com")),
        token(claims -> claims.expirationTime(Date.from(Instant.now().minusSeconds(600)))),
        token(claims -> claims.expirationTime(null)),
        token(claims -> claims.issueTime(Date.from(Instant.now().plusSeconds(600)))),
        token(claims -> claims.notBeforeTime(Date.from(Instant.now().plusSeconds(600)))),
        token(other, "k1", claims -> { }),
        token(key, "", claims -> { }),
        token(claims -> claims.claim("scp", List.of(1))),
        "not-a-jwt");
    for (String token : tokens) {
      assertNull(verifier.verify(token), token);
    }
  }

  @Test
  void rejectsInvalidConfiguration() {
    for (String[] config : new String[][] {
        {"http://issuer.example.com/jwks", ISSUER, AUDIENCE},
        {url, "http://issuer.example.com", AUDIENCE},
        {url, ISSUER, ""},
        {"https://user:pass@issuer.example.com/jwks", ISSUER, AUDIENCE},
    }) {
      assertThrows(IllegalArgumentException.class, () -> new JWKSTokenVerifier(JWKSTokenVerifier.config(config[0], config[1], config[2])));
    }
  }

  @Test
  void rejectsUnusableDocuments() throws Exception {
    RSAKey small = new RSAKeyGenerator(1024, true).keyID("k1").generate();
    Map<String, Object> hs256 = new java.util.HashMap<>(key.toPublicJWK().toJSONObject());
    hs256.put("alg", "HS256");
    Map<String, Object> encryption = new java.util.HashMap<>(key.toPublicJWK().toJSONObject());
    encryption.put("use", "enc");
    for (String doc : List.of("{\"keys\":[]}", jwks(small.toPublicJWK().toJSONObject()), jwks(hs256), jwks(encryption), "{\"keys\":[]} trailing")) {
      document.set(doc);
      assertNull(verifier(Duration.ZERO).verify(token(claims -> { })), doc);
    }
    reset();
    status.set(502);
    assertNull(verifier(Duration.ZERO).verify(token(claims -> { })));
  }

  @Test
  void refusesRedirectToHttp() throws Exception {
    redirect.set("http://127.0.0.1:9/jwks.json");
    assertNull(verifier(Duration.ZERO).verify(token(claims -> { })));
    assertEquals(1, requests.get());
  }

  @Test
  void unknownKidRefreshIsThrottled() throws Exception {
    JWKSTokenVerifier verifier = verifier(Duration.ZERO);
    assertNotNull(verifier.verify(token(claims -> { })));
    clock[0] = clock[0].plus(Duration.ofMinutes(2));
    for (int index = 0; index < 8; index++) {
      verifier.verify(token(key, "unknown-" + index, claims -> { }));
    }
    assertEquals(2, requests.get());
  }

  @Test
  void staleKeysExpireAfterTrustWindow() throws Exception {
    JWKSTokenVerifier verifier = verifier(Duration.ofMinutes(10));
    String token = token(claims -> { });
    assertNotNull(verifier.verify(token));
    status.set(500);
    clock[0] = clock[0].plus(Duration.ofMinutes(9));
    assertNotNull(verifier.verify(token));
    clock[0] = clock[0].plus(Duration.ofMinutes(2));
    assertNull(verifier.verify(token));
  }

  @Test
  void failedRefreshBacksOffExponentially() throws Exception {
    status.set(500);
    JWKSTokenVerifier verifier = verifier(Duration.ofSeconds(-1));
    String token = token(claims -> { });
    List<Long> delays = new ArrayList<>();
    for (int attempt = 0; attempt < 8; attempt++) {
      verifier.verify(token);
      delays.add(verifier.retryDelay().toSeconds());
      clock[0] = verifier.retryAt();
    }
    assertEquals(List.of(1L, 2L, 4L, 8L, 16L, 32L, 60L, 60L), delays);
    assertEquals(8, verifier.fetches());
  }
}
