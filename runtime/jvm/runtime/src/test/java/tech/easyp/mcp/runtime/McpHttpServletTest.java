package tech.easyp.mcp.runtime;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.google.protobuf.Struct;
import com.google.protobuf.Value;
import io.modelcontextprotocol.json.McpJsonDefaults;
import io.modelcontextprotocol.json.TypeRef;
import jakarta.servlet.http.HttpServlet;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import org.eclipse.jetty.ee10.servlet.ServletContextHandler;
import org.eclipse.jetty.ee10.servlet.ServletHolder;
import org.eclipse.jetty.server.Server;
import org.eclipse.jetty.server.ServerConnector;
import org.junit.jupiter.api.Test;
import tech.easyp.mcp.runtime.oauth.AccessToken;
import tech.easyp.mcp.runtime.oauth.OAuthResourceServer;

class McpHttpServletTest {
  private static final HttpClient CLIENT = HttpClient.newHttpClient();

  private static McpHttpServer server(boolean apps) {
    McpHttpServer server = new McpHttpServer();
    McpRuntime.ServerToolRegistry registry = McpRuntime.registryFor(server);
    registry.configureServer("servlet-test", "v1.2.3", apps);
    registry.registerTool(new McpRuntime.RegisteredTool(
        "whoami", "whoami", "", "{\"type\":\"object\"}", "{\"type\":\"object\"}",
        Struct::newBuilder, Struct::newBuilder,
        (exchange, request) -> {
          AccessToken token = McpRuntime.currentAccessToken();
          return Struct.newBuilder()
              .putFields("token", Value.newBuilder().setStringValue(token == null ? "" : token.token()).build())
              .build();
        },
        null, null, null, null, List.of("reports:write")));
    McpRuntime.installSessionFactory(server, registry);
    return server;
  }

  private static Server start(HttpServlet servlet) throws Exception {
    Server jetty = new Server();
    ServerConnector connector = new ServerConnector(jetty);
    connector.setHost("127.0.0.1");
    connector.setPort(0);
    jetty.addConnector(connector);
    ServletContextHandler context = new ServletContextHandler();
    context.addServlet(new ServletHolder(servlet), "/*");
    jetty.setHandler(context);
    jetty.start();
    return jetty;
  }

  private static String base(Server jetty) {
    return "http://127.0.0.1:" + ((ServerConnector) jetty.getConnectors()[0]).getLocalPort();
  }

  private static HttpResponse<String> post(Server jetty, String token, String body) throws Exception {
    HttpRequest.Builder request = HttpRequest.newBuilder(URI.create(base(jetty) + "/mcp"))
        .header("Content-Type", "application/json")
        .header("Accept", "application/json, text/event-stream")
        .POST(HttpRequest.BodyPublishers.ofString(body));
    if (token != null) {
      request.header("Authorization", "Bearer " + token);
    }
    return CLIENT.send(request.build(), HttpResponse.BodyHandlers.ofString());
  }

  private static Map<String, Object> result(HttpResponse<String> response) throws Exception {
    assertEquals(200, response.statusCode(), response.body());
    Map<String, Object> envelope = McpJsonDefaults.getMapper().readValue(response.body(), new TypeRef<Map<String, Object>>() {
    });
    @SuppressWarnings("unchecked")
    Map<String, Object> result = (Map<String, Object>) envelope.get("result");
    return result;
  }

  private static final String INITIALIZE = "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":"
      + "{\"protocolVersion\":\"2099-01-01\",\"capabilities\":{},\"clientInfo\":{\"name\":\"t\",\"version\":\"0\"}}}";
  private static final String CALL = "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"whoami\",\"arguments\":{}}}";

  @Test
  void protectedServerAdvertisesAppsAndPassesTheToken() throws Exception {
    OAuthResourceServer oauth = new OAuthResourceServer(
        "https://mcp.example.com/mcp", List.of("https://auth.example.com"), List.of(), "", "", "", "");
    HttpServlet servlet = McpHttpServlet.protect(server(true), oauth,
        token -> AccessToken.of(token, List.of("reports:write"), Instant.now().plusSeconds(60)));
    Server jetty = start(servlet);
    try {
      Map<String, Object> initialize = result(post(jetty, "writer", INITIALIZE));
      assertEquals("2025-11-25", initialize.get("protocolVersion"));
      assertEquals(Map.of("name", "servlet-test", "title", "servlet-test", "version", "v1.2.3"), initialize.get("serverInfo"));
      @SuppressWarnings("unchecked")
      Map<String, Object> capabilities = (Map<String, Object>) initialize.get("capabilities");
      assertEquals(Map.of(McpRuntime.APP_EXTENSION_ID, Map.of("mimeTypes", List.of(McpRuntime.APP_HTML_MIME_TYPE))), capabilities.get("extensions"));
      assertTrue(capabilities.containsKey("tools"));

      Map<String, Object> call = result(post(jetty, "writer", CALL));
      assertEquals(Map.of("token", "writer"), call.get("structuredContent"));

      HttpResponse<String> expired = post(jetty, null, CALL);
      assertEquals(401, expired.statusCode());
    }
    finally {
      jetty.stop();
    }
  }

  @Test
  void openServerStillDeniesScopedToolsWithoutAToken() throws Exception {
    Server jetty = start(McpHttpServlet.open(server(false)));
    try {
      Map<String, Object> call = result(post(jetty, null, CALL));
      assertEquals(Boolean.TRUE, call.get("isError"));
      assertEquals(List.of(Map.of("type", "text", "text", "permission denied: an authenticated token is required")), call.get("content"));

      HttpResponse<String> metadata = CLIENT.send(
          HttpRequest.newBuilder(URI.create(base(jetty) + OAuthResourceServer.METADATA_PATH)).build(),
          HttpResponse.BodyHandlers.ofString());
      assertEquals(404, metadata.statusCode());
      HttpResponse<String> unknownMethod = post(jetty, null, "{\"jsonrpc\":\"2.0\",\"id\":3,\"method\":\"nope\"}");
      assertTrue(unknownMethod.body().contains("-32601"), unknownMethod.body());
    }
    finally {
      jetty.stop();
    }
  }
}
