package tech.easyp.mcp.runtime.oauth;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;

import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.Set;
import org.junit.jupiter.api.Test;

class ScopeGateTest {
  static final Map<String, List<String>> TOOLS = Map.of("secure", List.of("reports:write"), "open", List.of());

  static Set<String> scopes(String body) {
    return ScopeGate.requestedToolScopes(body, TOOLS::get);
  }

  @Test
  void collectsScopesOfEveryCall() {
    assertEquals(Set.of("reports:write"), scopes("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"secure\"}}"));
    assertEquals(Set.of(), scopes("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"open\"}}"));
    assertEquals(Set.of(), scopes("{\"jsonrpc\":\"2.0\",\"method\":\"tools/call\",\"params\":{\"name\":\"secure\"}}"));
    assertEquals(Set.of("reports:write"), scopes("[{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/list\"},"
        + "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"secure\"}}]"));
    assertEquals(Set.of(), scopes("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"Name\":\"secure\",\"name\":\"open\"}}"));
  }

  @Test
  void rejectsAmbiguousRequests() {
    for (String body : List.of(
        "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"open\",\"name\":\"secure\"}}",
        "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"open\"}} {}",
        "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":[]}",
        "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":1}}",
        "{")) {
      assertThrows(IllegalArgumentException.class, () -> scopes(body), body);
    }
  }

  @Test
  void toolGuardMatchesGo() {
    assertNull(ScopeGate.toolGuardError(List.of(), null));
    assertEquals("permission denied: an authenticated token is required", ScopeGate.toolGuardError(List.of("reports:write"), null));
    AccessToken reader = AccessToken.of("t", List.of("reports:read"), Instant.now().plusSeconds(60));
    assertEquals("permission denied: missing required scope \"reports:write\"", ScopeGate.toolGuardError(List.of("reports:write"), reader));
    AccessToken writer = AccessToken.of("t", List.of("reports:write"), Instant.now().plusSeconds(60));
    assertNull(ScopeGate.toolGuardError(List.of("reports:write"), writer));
  }

  @Test
  void resourceServerMetadataAndChallenges() {
    OAuthResourceServer oauth = new OAuthResourceServer(
        "https://mcp.example.com/mcp", List.of("https://auth.example.com"), List.of("reports:read"), "", "", "", "");
    assertEquals("/mcp", oauth.mcpPath());
    assertEquals("/.well-known/oauth-protected-resource/mcp", oauth.metadataPath());
    assertEquals("https://mcp.example.com/.well-known/oauth-protected-resource/mcp", oauth.metadataUrl());
    assertEquals("{\"resource\":\"https://mcp.example.com/mcp\",\"authorization_servers\":[\"https://auth.example.com\"],"
        + "\"scopes_supported\":[\"reports:read\",\"reports:write\"]}", oauth.metadataJson(List.of("reports:write")));
    assertEquals("Bearer error=\"insufficient_scope\", scope=\"reports:read reports:write\", "
        + "resource_metadata=\"https://mcp.example.com/.well-known/oauth-protected-resource/mcp\"",
        oauth.insufficientScopeChallenge(List.of("reports:write", "reports:read", "reports:write")));
    assertThrows(IllegalArgumentException.class, () -> new OAuthResourceServer("http://mcp.example.com", List.of("https://a"), null, "", "", "", ""));
    assertThrows(IllegalArgumentException.class, () -> new OAuthResourceServer("https://mcp.example.com", List.of(), null, "", "", "", ""));
    assertThrows(IllegalArgumentException.class, () -> new OAuthResourceServer("https://mcp.example.com", List.of("https://a"), null, "", "", "", "/.well-known/x"));
    assertThrows(IllegalArgumentException.class, () -> oauth.jwksVerifier());
  }
}
