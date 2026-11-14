package tech.easyp.mcp.runtime;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;

import io.modelcontextprotocol.spec.McpError;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;

class McpRuntimeTest {
  private static McpRuntime.RegisteredResourceTemplate template(String uriTemplate, String... names) {
    return new McpRuntime.RegisteredResourceTemplate(
        Map.of("uriTemplate", uriTemplate),
        McpRuntime.compileUriTemplate(uriTemplate),
        List.of(names),
        List::of,
        (uri, params) -> Map.of());
  }

  @Test
  void uriTemplatesMatchAndPercentDecodeLikeGo() {
    McpRuntime.RegisteredResourceTemplate users = template("users://{user_id}/profile", "user_id");
    assertEquals(Map.of("user_id", "ada l"), McpRuntime.matchUriTemplate(users, "users://ada%20l/profile"));
    assertEquals(Map.of("user_id", "a+b"), McpRuntime.matchUriTemplate(users, "users://a+b/profile"));
    assertNull(McpRuntime.matchUriTemplate(users, "users://ada/settings"));
    assertNull(McpRuntime.matchUriTemplate(users, "users://a/b/profile"));
    assertThrows(McpError.class, () -> McpRuntime.matchUriTemplate(users, "users://bad%zz/profile"));

    McpRuntime.RegisteredResourceTemplate dotted = template("docs://v1.0/{name}", "name");
    assertNull(McpRuntime.matchUriTemplate(dotted, "docs://v1x0/a"));
  }

  @Test
  void toolNamesJoinNamespacesWithoutDots() {
    assertEquals("raw_pb_Create", McpRuntime.toolName("raw.pb", "Create"));
    assertEquals("Create", McpRuntime.toolName("", "Create"));
    assertEquals("example_default", McpRuntime.normalizeNamespace(null, "example.default"));
  }

  @Test
  void gencodeVersionMismatchFails() {
    McpRuntime.checkGencodeVersion(McpRuntime.GENCODE_VERSION);
    assertThrows(IllegalStateException.class, () -> McpRuntime.checkGencodeVersion(McpRuntime.GENCODE_VERSION + 1));
  }
}
