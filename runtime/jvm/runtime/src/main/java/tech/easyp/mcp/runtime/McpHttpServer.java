package tech.easyp.mcp.runtime;

import io.modelcontextprotocol.spec.McpServerSession;
import io.modelcontextprotocol.spec.McpServerTransportProvider;
import reactor.core.publisher.Mono;

/**
 * A server served over stateless Streamable HTTP. Generated
 * {@code install<File>MCPServer} methods register primitives on it like on any
 * transport provider, and {@link McpHttpServlet} serves them. Stateless HTTP
 * has no sessions, so session factories and notifications are ignored.
 */
public final class McpHttpServer implements McpServerTransportProvider {
  @Override
  public void setSessionFactory(McpServerSession.Factory sessionFactory) {
  }

  @Override
  public Mono<Void> notifyClients(String method, Object params) {
    return Mono.empty();
  }

  @Override
  public Mono<Void> closeGracefully() {
    return Mono.empty();
  }
}
