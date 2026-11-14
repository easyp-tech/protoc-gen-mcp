package tech.easyp.mcp.runtime;

import io.modelcontextprotocol.common.McpTransportContext;
import io.modelcontextprotocol.json.McpJsonDefaults;
import io.modelcontextprotocol.server.transport.HttpServletStatelessServerTransport;
import jakarta.servlet.ReadListener;
import jakarta.servlet.ServletConfig;
import jakarta.servlet.ServletException;
import jakarta.servlet.ServletInputStream;
import jakarta.servlet.http.HttpServlet;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletRequestWrapper;
import jakarta.servlet.http.HttpServletResponse;
import java.io.BufferedReader;
import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.io.InputStreamReader;
import java.net.URI;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;
import tech.easyp.mcp.runtime.oauth.AccessToken;
import tech.easyp.mcp.runtime.oauth.OAuthResourceServer;
import tech.easyp.mcp.runtime.oauth.ScopeGate;
import tech.easyp.mcp.runtime.oauth.TokenVerifier;

/**
 * Streamable HTTP endpoint for a generated server, ported from
 * mcpruntime.NewConfiguredHTTPHandler and NewOAuthResourceHandler. MCP itself
 * is served by the SDK {@link HttpServletStatelessServerTransport}; this
 * servlet adds what the SDK lacks: RFC 9728 metadata with live
 * scopes_supported, bearer authentication, and a per-tool scope check that
 * answers 403 insufficient_scope before any tool runs. Mount it at "/*".
 */
public final class McpHttpServlet extends HttpServlet {
  private static final long serialVersionUID = 1L;

  private final transient McpHttpServer server;
  private final transient OAuthResourceServer oauth;
  private final transient TokenVerifier verifier;
  private final transient HttpServletStatelessServerTransport transport;
  private final String mcpPath;

  private McpHttpServlet(McpHttpServer server, OAuthResourceServer oauth, TokenVerifier verifier) {
    this.server = server;
    this.oauth = oauth;
    this.verifier = verifier;
    this.mcpPath = oauth == null ? "/mcp" : oauth.mcpPath();
    this.transport = HttpServletStatelessServerTransport.builder()
        .jsonMapper(McpJsonDefaults.getMapper())
        .messageEndpoint(this.mcpPath)
        .contextExtractor(request -> {
          Object token = request.getAttribute(McpRuntime.ACCESS_TOKEN_KEY);
          return token == null ? McpTransportContext.EMPTY : McpTransportContext.create(Map.of(McpRuntime.ACCESS_TOKEN_KEY, token));
        })
        .build();
    this.transport.setMcpHandler(McpRuntime.newStatelessHandler(server, this.transport.protocolVersions()));
  }

  /** Serves server on /mcp without authentication. */
  public static HttpServlet open(McpHttpServer server) {
    if (server == null) {
      throw new IllegalArgumentException("mcpruntime: server is null");
    }
    return new McpHttpServlet(server, null, null);
  }

  /**
   * Serves server as an OAuth protected resource. verifier validates bearer
   * tokens; when it is null the built-in RS256 JWKS verifier is built from the
   * policy's issuer, audience and JWKS URL.
   */
  public static HttpServlet protect(McpHttpServer server, OAuthResourceServer oauth, TokenVerifier verifier) {
    if (server == null) {
      throw new IllegalArgumentException("mcpruntime: server is null");
    }
    if (oauth == null) {
      throw new IllegalArgumentException("mcpruntime: OAuth policy is null");
    }
    return new McpHttpServlet(server, oauth, verifier == null ? oauth.jwksVerifier() : verifier);
  }

  @Override
  public void init(ServletConfig config) throws ServletException {
    super.init(config);
    transport.init(config);
  }

  @Override
  public void destroy() {
    transport.destroy();
    super.destroy();
  }

  @Override
  protected void service(HttpServletRequest request, HttpServletResponse response) throws ServletException, IOException {
    String path = request.getRequestURI().substring(request.getContextPath().length());
    if (oauth != null && OAuthResourceServer.isUnderMetadataPrefix(path)) {
      metadata(request, response, path);
      return;
    }
    if (!path.equals(mcpPath)) {
      text(response, HttpServletResponse.SC_NOT_FOUND, "404 page not found\n");
      return;
    }
    if (!sameOrigin(request)) {
      text(response, HttpServletResponse.SC_FORBIDDEN, "cross-origin request detected\n");
      return;
    }
    AccessToken token = null;
    if (oauth != null) {
      token = authenticate(request);
      if (token == null) {
        response.setHeader("WWW-Authenticate", oauth.bearerChallenge());
        text(response, HttpServletResponse.SC_UNAUTHORIZED, "invalid token\n");
        return;
      }
    }
    HttpServletRequest forwarded = request;
    if ("POST".equals(request.getMethod())) {
      byte[] body = request.getInputStream().readNBytes(ScopeGate.MAX_REQUEST_BODY_BYTES + 1);
      if (body.length > ScopeGate.MAX_REQUEST_BODY_BYTES) {
        text(response, HttpServletResponse.SC_REQUEST_ENTITY_TOO_LARGE, "MCP request body too large\n");
        return;
      }
      if (oauth != null) {
        Set<String> required = new TreeSet<>(oauth.scopes());
        try {
          required.addAll(ScopeGate.requestedToolScopes(
              new String(body, StandardCharsets.UTF_8), name -> McpRuntime.toolScopes(server, name)));
        }
        catch (IllegalArgumentException error) {
          text(response, HttpServletResponse.SC_BAD_REQUEST, "invalid MCP request\n");
          return;
        }
        if (!ScopeGate.missing(required, token.scopes()).isEmpty()) {
          response.setHeader("WWW-Authenticate", oauth.insufficientScopeChallenge(required));
          response.setHeader("Cache-Control", "no-store");
          text(response, HttpServletResponse.SC_FORBIDDEN, "insufficient scope\n");
          return;
        }
      }
      forwarded = new BufferedRequest(request, body);
    }
    else if (oauth != null && !ScopeGate.missing(oauth.scopes(), token.scopes()).isEmpty()) {
      response.setHeader("WWW-Authenticate", oauth.insufficientScopeChallenge(oauth.scopes()));
      response.setHeader("Cache-Control", "no-store");
      text(response, HttpServletResponse.SC_FORBIDDEN, "insufficient scope\n");
      return;
    }
    if (token != null) {
      forwarded.setAttribute(McpRuntime.ACCESS_TOKEN_KEY, token);
    }
    transport.service(forwarded, response);
  }

  private AccessToken authenticate(HttpServletRequest request) {
    String header = request.getHeader("Authorization");
    if (header == null || header.length() < 7 || !header.regionMatches(true, 0, "Bearer ", 0, 7)) {
      return null;
    }
    String raw = header.substring(7).trim();
    if (raw.isEmpty()) {
      return null;
    }
    try {
      AccessToken token = verifier.verify(raw);
      if (token == null || token.expiresAt() == null || !token.expiresAt().isAfter(Instant.now())) {
        return null;
      }
      return token;
    }
    catch (Exception error) {
      return null;
    }
  }

  private void metadata(HttpServletRequest request, HttpServletResponse response, String path) throws IOException {
    if (!oauth.servesMetadata(path)) {
      text(response, HttpServletResponse.SC_NOT_FOUND, "404 page not found\n");
      return;
    }
    response.setHeader("Access-Control-Allow-Origin", "*");
    response.setHeader("Access-Control-Allow-Methods", "GET, OPTIONS");
    response.setHeader("Access-Control-Allow-Headers", "Content-Type");
    switch (request.getMethod()) {
      case "OPTIONS" -> response.setStatus(HttpServletResponse.SC_NO_CONTENT);
      case "GET" -> {
        byte[] body = oauth.metadataJson(McpRuntime.allToolScopes(server)).getBytes(StandardCharsets.UTF_8);
        response.setStatus(HttpServletResponse.SC_OK);
        response.setContentType("application/json");
        response.setContentLength(body.length);
        response.getOutputStream().write(body);
      }
      default -> text(response, HttpServletResponse.SC_METHOD_NOT_ALLOWED, "Method not allowed\n");
    }
  }

  /** Mirrors net/http CrossOriginProtection for state-changing requests. */
  private static boolean sameOrigin(HttpServletRequest request) {
    if (List.of("GET", "HEAD", "OPTIONS").contains(request.getMethod())) {
      return true;
    }
    String site = request.getHeader("Sec-Fetch-Site");
    if (site != null) {
      return site.equals("same-origin") || site.equals("none");
    }
    String origin = request.getHeader("Origin");
    if (origin == null) {
      return true;
    }
    try {
      return String.valueOf(URI.create(origin).getRawAuthority()).equals(request.getHeader("Host"));
    }
    catch (IllegalArgumentException error) {
      return false;
    }
  }

  private static void text(HttpServletResponse response, int status, String message) throws IOException {
    byte[] body = message.getBytes(StandardCharsets.UTF_8);
    response.setStatus(status);
    response.setContentType("text/plain; charset=utf-8");
    response.setHeader("X-Content-Type-Options", "nosniff");
    response.setContentLength(body.length);
    response.getOutputStream().write(body);
  }

  /** Replays a body already read for the scope check; JSON is always UTF-8. */
  private static final class BufferedRequest extends HttpServletRequestWrapper {
    private final byte[] body;

    BufferedRequest(HttpServletRequest request, byte[] body) {
      super(request);
      this.body = body;
    }

    @Override
    public ServletInputStream getInputStream() {
      ByteArrayInputStream in = new ByteArrayInputStream(body);
      return new ServletInputStream() {
        @Override
        public int read() {
          return in.read();
        }

        @Override
        public int read(byte[] buffer, int offset, int length) {
          return in.read(buffer, offset, length);
        }

        @Override
        public boolean isFinished() {
          return in.available() == 0;
        }

        @Override
        public boolean isReady() {
          return true;
        }

        @Override
        public void setReadListener(ReadListener listener) {
          throw new IllegalStateException("mcpruntime: the request body is already buffered");
        }
      };
    }

    @Override
    public BufferedReader getReader() {
      return new BufferedReader(new InputStreamReader(new ByteArrayInputStream(body), StandardCharsets.UTF_8));
    }

    @Override
    public int getContentLength() {
      return body.length;
    }

    @Override
    public long getContentLengthLong() {
      return body.length;
    }

    @Override
    public String getCharacterEncoding() {
      return StandardCharsets.UTF_8.name();
    }
  }
}
