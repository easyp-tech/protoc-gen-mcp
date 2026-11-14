package tech.easyp.mcp.runtime.oauth;

/**
 * Validates bearer tokens for the protected MCP endpoint, like auth.TokenVerifier
 * in the Go runtime. Returning null or throwing rejects the token with 401.
 */
@FunctionalInterface
public interface TokenVerifier {
  AccessToken verify(String token) throws Exception;
}
