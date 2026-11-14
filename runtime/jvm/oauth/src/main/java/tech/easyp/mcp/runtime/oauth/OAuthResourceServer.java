package tech.easyp.mcp.runtime.oauth;

import java.net.URI;
import java.util.ArrayList;
import java.util.Collection;
import java.util.List;
import java.util.TreeSet;

/**
 * Public OAuth policy from {@code option (mcp.options.v1.server).oauth}: the
 * RFC 9728 protected resource, its authorization servers and global scopes,
 * plus what the built-in JWKS verifier needs.
 */
public final class OAuthResourceServer {
  /** RFC 9728 well-known prefix for protected resource metadata. */
  public static final String METADATA_PATH = "/.well-known/oauth-protected-resource";

  private final String resourceUrl;
  private final List<String> authorizationServers;
  private final List<String> scopes;
  private final String issuer;
  private final String audience;
  private final String jwksUrl;
  private final String mcpPath;
  private final String metadataPath;
  private final String metadataUrl;

  public OAuthResourceServer(
      String resourceUrl,
      List<String> authorizationServers,
      List<String> scopes,
      String issuer,
      String audience,
      String jwksUrl,
      String mcpPath) {
    URI resource = httpsUrl(resourceUrl, "resource URL");
    if (authorizationServers == null || authorizationServers.isEmpty()) {
      throw new IllegalArgumentException("mcpruntime: at least one authorization server is required");
    }
    for (String server : authorizationServers) {
      httpsUrl(server, "authorization server");
    }
    String path = mcpPath == null || mcpPath.isEmpty() ? "/mcp" : mcpPath;
    if (!path.startsWith("/") || path.equals("/") || path.startsWith("/.well-known/")) {
      throw new IllegalArgumentException("mcpruntime: invalid MCP path " + Json.quote(path));
    }
    this.resourceUrl = resourceUrl;
    this.authorizationServers = List.copyOf(authorizationServers);
    this.scopes = scopes == null ? List.of() : List.copyOf(scopes);
    this.issuer = issuer == null ? "" : issuer;
    this.audience = audience == null ? "" : audience;
    this.jwksUrl = jwksUrl == null ? "" : jwksUrl;
    this.mcpPath = path;
    String resourcePath = resource.getRawPath() == null ? "" : resource.getRawPath();
    while (resourcePath.endsWith("/")) {
      resourcePath = resourcePath.substring(0, resourcePath.length() - 1);
    }
    this.metadataPath = METADATA_PATH + resourcePath;
    this.metadataUrl = resource.getScheme() + "://" + resource.getRawAuthority() + this.metadataPath;
  }

  private static URI httpsUrl(String raw, String what) {
    try {
      URI uri = new URI(raw);
      if ("https".equals(uri.getScheme()) && uri.getRawAuthority() != null && !uri.getRawAuthority().isEmpty()) {
        return uri;
      }
    }
    catch (Exception ignored) {
      // Reported below with the field name.
    }
    throw new IllegalArgumentException("mcpruntime: invalid HTTPS " + what + " " + Json.quote(String.valueOf(raw)));
  }

  public String resourceUrl() {
    return resourceUrl;
  }

  public List<String> authorizationServers() {
    return authorizationServers;
  }

  public List<String> scopes() {
    return scopes;
  }

  public String issuer() {
    return issuer;
  }

  public String audience() {
    return audience;
  }

  public String jwksUrl() {
    return jwksUrl;
  }

  public String mcpPath() {
    return mcpPath;
  }

  /** Path-scoped metadata path, for example /.well-known/oauth-protected-resource/mcp. */
  public String metadataPath() {
    return metadataPath;
  }

  /** Absolute URL of the path-scoped metadata, advertised in every challenge. */
  public String metadataUrl() {
    return metadataUrl;
  }

  /** Whether path is under the well-known metadata prefix and must not reach MCP. */
  public static boolean isUnderMetadataPrefix(String path) {
    return path.equals(METADATA_PATH) || path.startsWith(METADATA_PATH + "/");
  }

  /** Whether path serves this resource's metadata: at the root or path-scoped. */
  public boolean servesMetadata(String path) {
    return path.equals(METADATA_PATH) || path.equals(metadataPath);
  }

  /** The built-in RS256 JWKS verifier for this policy. */
  public TokenVerifier jwksVerifier() {
    if (jwksUrl.isEmpty()) {
      throw new IllegalArgumentException("mcpruntime: OAuth needs a token verifier or a JWKS URL");
    }
    return new JWKSTokenVerifier(JWKSTokenVerifier.config(jwksUrl, issuer, audience));
  }

  /** RFC 9728 document whose scopes_supported adds every tool's required scopes. */
  public String metadataJson(Collection<String> toolScopes) {
    TreeSet<String> supported = new TreeSet<>(scopes);
    supported.addAll(toolScopes);
    StringBuilder out = new StringBuilder("{\"resource\":").append(Json.quote(resourceUrl));
    out.append(",\"authorization_servers\":").append(jsonArray(authorizationServers));
    if (!supported.isEmpty()) {
      out.append(",\"scopes_supported\":").append(jsonArray(new ArrayList<>(supported)));
    }
    return out.append('}').toString();
  }

  /** WWW-Authenticate value for a missing or invalid token. */
  public String bearerChallenge() {
    return "Bearer resource_metadata=" + Json.quote(metadataUrl);
  }

  /** WWW-Authenticate value for 403 listing every scope the request needs. */
  public String insufficientScopeChallenge(Collection<String> required) {
    return "Bearer error=\"insufficient_scope\", scope=" + Json.quote(String.join(" ", new TreeSet<>(required)))
        + ", resource_metadata=" + Json.quote(metadataUrl);
  }

  private static String jsonArray(List<String> values) {
    List<String> quoted = new ArrayList<>();
    for (String value : values) {
      quoted.add(Json.quote(value));
    }
    return "[" + String.join(",", quoted) + "]";
  }
}
