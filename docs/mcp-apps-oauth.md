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
your Go binary.

**You can also declare the HTML resource in protobuf**, not just the tool
metadata:

~~~proto
message ReportApp {
  option (mcp.options.v1.resource) = {
    name: "report_app"
    uri: "ui://reporting/composer"
    mime_type: "text/html;profile=mcp-app"
    content_field: "html"
  };
  string html = 1;
}
~~~

Implement the generated `ReadReportApp(ctx)` handler to return
`&ReportApp{Html: appHTML}`, register it with the generated
`Register<File>Resources(ctx, server, handler)` function and create the server
with `&mcp.ServerOptions{Capabilities: mcpruntime.AppCapabilities()}`.
The generator handles resource registration, MIME and unescaped HTML.
Constructing and serving the frontend remains the application's responsibility.

When protobuf resource annotations are not convenient, the original
`mcpruntime.RegisterAppResource(server, AppResource{...})` helper is also
available. The standard SDK's app extension capability must still be enabled
at server construction; the protobuf annotations do not create a server.

**Resource URIs in RPC `app_ui` and resource `uri` must match.** The
resource may be declared in a different protobuf file or registered manually.

A tool can instead be linked to a resource at registration time without
changing its protobuf method option, using
`mcpruntime.WithAppUI("CreateReport", "ui://reporting/composer", "model", "app")`.

The UI is optional: clients without MCP Apps support still get ordinary
`structuredContent` plus text. Do not put secrets or access tokens in HTML,
tool descriptions or `_meta`.

## Protobuf-driven Markdown, text and binary resources

Resources default to JSON. Without `content_field`, the generator serializes
the entire protobuf response using ProtoJSON; `mime_type` must be JSON-like
(`application/json` or `application/*+json`). A non-JSON MIME without a raw
content field is rejected at code-generation time.

Use `content_field` to select a **singular string or bytes protobuf field** as
the resource body. The selector is the *protobuf field name*, not its JSON
name. A string field is emitted as MCP `text`; a bytes field as MCP `blob`
(base64 on the wire). Both static and URI-template resources are supported.

~~~proto
message SkillDocument {
  option (mcp.options.v1.resource) = {
    name: "skill"
    uri: "skill://easyp/SKILL.md"
    mime_type: "text/markdown"
    content_field: "markdown"
  };
  string markdown = 1;
}
~~~

A real file can be embedded by the implementing Go application:

~~~go
//go:embed skills/SKILL.md
var skillMarkdown string

func (Handler) ReadSkillDocument(_ context.Context) (*v1.SkillDocument, error) {
    return &v1.SkillDocument{Markdown: skillMarkdown}, nil
}
~~~

The resulting `resources/read` returns the actual Markdown, **not** the JSON
object `{"markdown":"..."}`. `text/plain`, raw HTML, XML and other text
formats work the same way. With a `bytes` field and
`application/octet-stream`, binary attachments are returned as MCP blobs.
Invalid field selectors, repeated/map fields, and byte fields advertised as
text MIME fail at generation time.

Exposing a `SKILL.md` as an MCP resource does not automatically install it as
a native Skill in every host. The host must discover/read the resource, or an
MCP tool/prompt must explicitly refer to it.

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
