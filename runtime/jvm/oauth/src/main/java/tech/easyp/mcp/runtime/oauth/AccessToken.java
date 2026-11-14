package tech.easyp.mcp.runtime.oauth;

import java.time.Instant;
import java.util.List;
import java.util.Map;

/**
 * A validated bearer token: the scopes it grants and when it expires. The
 * HTTP endpoint rejects tokens without an expiry, as the Go runtime does.
 */
public record AccessToken(
    String token,
    String clientId,
    String subject,
    List<String> scopes,
    Instant expiresAt,
    Map<String, Object> claims) {

  public AccessToken {
    scopes = scopes == null ? List.of() : List.copyOf(scopes);
    claims = claims == null ? Map.of() : Map.copyOf(claims);
  }

  /** A token with only scopes and an expiry, convenient for custom verifiers. */
  public static AccessToken of(String token, List<String> scopes, Instant expiresAt) {
    return new AccessToken(token, "", "", scopes, expiresAt, Map.of());
  }
}
