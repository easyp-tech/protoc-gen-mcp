# Go MCP Apps and OAuth

The Go target is built on the official
[Model Context Protocol Go SDK](https://github.com/modelcontextprotocol/go-sdk).
MCP transport, protocol negotiation, sessions, prompts, resource templates,
tools and HTTP handling belong to that SDK. The generated adapter retains
ProtoJSON conversion and JSON Schema validation.

## Attach an MCP App to an RPC

The `app_ui` option is attached to an ordinary unary RPC:

~~~proto
rpc CreateReport(CreateReportRequest) returns (CreateReportResponse) {
  option (mcp.options.v1.method) = {
    title: "Create report"
    app_ui: {
      resource_uri: "ui://reporting/composer"
      visibility: "model"
      visibility: "app"
    }
  };
}
~~~

The generated tool includes:

~~~json
{
  "_meta": {
    "ui": {
      "resourceUri": "ui://reporting/composer",
      "visibility": ["model", "app"]
    }
  }
}
~~~

The MCP Apps UI extension is `io.modelcontextprotocol/ui`; its HTML MIME
type is `text/html;profile=mcp-app`. The generator does not generate frontend
code. Build your UI separately (e.g., Vite) and embed the resulting HTML in
your Go binary. A resource URI declared by an RPC must be registered by the
application.

~~~go
//go:embed web/dist/index.html
var appHTML string

server := mcp.NewServer(
    &mcp.Implementation{Name: "reporting", Version: "v1.0.0"},
    &mcp.ServerOptions{Capabilities: mcpruntime.AppCapabilities()},
)
if err := reportingv1.RegisterReportingAPITools(server, handler{}); err != nil {
    log.Fatal(err)
}
if err := mcpruntime.RegisterAppResource(server, mcpruntime.AppResource{
    URI:  "ui://reporting/composer",
    Name: "Report composer",
    HTML: appHTML,
}); err != nil {
    log.Fatal(err)
}
~~~

A tool can instead be linked to a resource at registration time without
changing its protobuf method option, using
`mcpruntime.WithAppUI("CreateReport", "ui://reporting/composer", "model", "app")`.

The UI is optional: clients without MCP Apps support still get ordinary
`structuredContent` plus text. Do not put secrets or access tokens in HTML,
tool descriptions or `_meta`.

## Protect a remote MCP endpoint with OAuth

OAuth is independent of the UI. The Go SDK supports OAuth *resource-server*
behavior: bearer validation, public Protected Resource Metadata and
authorization scopes. It is not a complete Authorization Server.

`mcpruntime.NewOAuthResourceHandler` serves the public discovery URL
`/.well-known/oauth-protected-resource` and protects the MCP endpoint.

~~~go
httpHandler, err := mcpruntime.NewOAuthResourceHandler(server, mcpruntime.OAuthResourceServer{
    ResourceURL:          "https://api.example.com/mcp",
    AuthorizationServers: []string{"https://auth.example.com"},
    Scopes:               []string{"reports:read"},
    Verifier:             verifyToken,
})
if err != nil {
    log.Fatal(err)
}
log.Fatal(http.ListenAndServeTLS(":443", "cert.pem", "key.pem", httpHandler))
~~~

The `verifyToken` function must satisfy `auth.TokenVerifier` from
`github.com/modelcontextprotocol/go-sdk/auth` and validate signature, issuer,
audience, resource binding and token status with your identity provider. It
must return `auth.TokenInfo` including expiration, scopes and an appropriate
user identity. The public metadata route is not authenticated; MCP requests
are. Keep identity provider login, consent, code/PKCE and token issuance
outside the generator and MCP resource server.

The helper defaults to stateless Streamable HTTP, making newer MCP protocol
versions available while the official SDK handles compatibility with legacy
clients.

## Validation

- `go test ./mcpruntime` verifies tool registration, in-memory SDK
  connections, UI tool metadata, resource contents and bearer middleware.
- `go test ./internal/examplemcp` checks the generated tools/prompts/resources
  over MCP transports.
- `go test ./internal/codegen` verifies generator contracts and goldens.
- `go test ./...` is the repository-wide gate.

For production, serve HTTPS, maintain strict verifier checks, and do not
expose app-only destructive tools without independent authorization checks.
