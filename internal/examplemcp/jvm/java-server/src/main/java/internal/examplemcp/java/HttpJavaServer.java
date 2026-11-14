package internal.examplemcp.java;

import internal.testproto.config.v1.ServerConfig;
import internal.testproto.config.v1.ServerConfigMcp;
import io.modelcontextprotocol.server.McpAsyncServerExchange;
import jakarta.servlet.http.HttpServlet;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import org.eclipse.jetty.ee10.servlet.ServletContextHandler;
import org.eclipse.jetty.ee10.servlet.ServletHolder;
import org.eclipse.jetty.server.Server;
import org.eclipse.jetty.server.ServerConnector;
import tech.easyp.mcp.runtime.McpHttpServer;
import tech.easyp.mcp.runtime.oauth.AccessToken;

/** Java counterpart of the Go HTTP fixture server for the shared HTTP contract. */
final class HttpJavaServer {
  private static final Map<String, List<String>> DEMO_SCOPES = Map.of(
      "reader", List.of("reports:read"),
      "writer", List.of("reports:read", "reports:write"));

  private HttpJavaServer() {
  }

  static void run() throws Exception {
    McpHttpServer server = new McpHttpServer();
    ServerConfigMcp.installServerConfigMCPServer(server, new ServerConfigMcp.ServerConfigMCPHandlers(new SecureHandler()));
    HttpServlet servlet = ServerConfigMcp.newServerConfigMCPHttpServlet(server, token -> {
      List<String> scopes = DEMO_SCOPES.get(token);
      return scopes == null ? null : AccessToken.of(token, scopes, Instant.now().plusSeconds(3600));
    });

    Server jetty = new Server();
    ServerConnector connector = new ServerConnector(jetty);
    connector.setHost("127.0.0.1");
    connector.setPort(0);
    jetty.addConnector(connector);
    ServletContextHandler context = new ServletContextHandler();
    context.addServlet(new ServletHolder(servlet), "/*");
    jetty.setHandler(context);
    jetty.start();
    System.out.println("listening on http://127.0.0.1:" + connector.getLocalPort());
    System.out.flush();
    jetty.join();
  }

  private static final class SecureHandler implements ServerConfigMcp.SecureAPIToolHandler {
    @Override
    public ServerConfig.CheckAccessResponse checkAccess(McpAsyncServerExchange ctx, ServerConfig.CheckAccessRequest request) {
      return ServerConfig.CheckAccessResponse.newBuilder().setAllowed(true).build();
    }
  }
}
