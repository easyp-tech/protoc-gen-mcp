# Protobuf-first Go MCP server, MCP Apps and OAuth

The Go target uses the official
[Model Context Protocol Go SDK](https://github.com/modelcontextprotocol/go-sdk).
Protocol negotiation, Streamable HTTP, sessions, tools, prompts and resources
are SDK-owned. `protoc-gen-mcp` owns only protobuf descriptions, generated
bindings, ProtoJSON/JSON Schema adapters and declarative server wiring.

## Declarative server configuration

The `(mcp.options.v1.server)` **file option** configures a generated server
factory. The generated factory registers tools/prompts/resources from that
protobuf file, chooses MCP Apps capabilities and exposes a configured HTTP
handler. The original `Register<Service>Tools` and
`Register<File>Resources` functions are still available for composing
several proto files into one server.

```proto
syntax = "proto3";
package example.v1;
import "mcp/options/v1/options.proto";

option (mcp.options.v1.server) = {
  name: "example-mcp"
  version: "v1.0.0"
  // Optional: if omitted, the generator detects MCP Apps tools/resources.
  apps: { enabled: true }
};
```

The generator emits:

- `<File>MCPHandlers` — typed business handlers, one field per tool service and
  optional fields for dynamic resources/prompts.
- `New<File>MCPServer(ctx, handlers) (*mcp.Server, error)` — constructs the
  official SDK server, enables the extension and registers this file's MCP
  features. File names use the descriptor-safe Go symbol name (for example,
  `NewFile_internal_testproto_resources_v1_resources_protoMCPServer`).
- `New<File>MCPHTTPHandler(server, verifier)` when a server option exists;
  the verifier argument may be nil if the configured OAuth provider supports
  the built-in JWKS flow.

Omitting the server option preserves the existing generator API. When a file
declares an MCP Apps tool or resource, the factory is also generated
automatically; the Apps extension is enabled unless explicitly disabled.
`apps.enabled: false` alongside MCP Apps features is a generation error.

**Composition:** a factory handles its own protobuf file only. Register
additional generated packages onto the returned `*mcp.Server` through their
`Register...Tools/Resources/Prompts` APIs. This avoids duplicate factories
with conflicting global server policies.

## Tools and MCP Apps UI

```proto
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
```

This generates the standard MCP Apps tool metadata
`_meta.ui.resourceUri` and `visibility`, without replacing the ordinary
text or structured tool outputs.

### Auto-embed a built frontend

```proto
// A resource can be static, with no Go handler and no body field.
message ReportDashboard {
  option (mcp.options.v1.resource) = {
    name: "report_ui"
    uri: "ui://reporting/composer"
    mime_type: "text/html;profile=mcp-app"
    source_file: "assets/index.html"
    app_ui: {
      csp: {
        connect_domains: "https://api.example.com"
        resource_domains: "https://cdn.example.com"
      }
      permissions: { clipboard_write: true }
      prefers_border: true
    }
  };
}
```

Build the React/Vite frontend **before** generating/compiling Go. Place the
compiled single HTML file at `assets/index.html` relative to the **generated
Go package directory** (not the root of the project). The generator emits a
`//go:embed` directive and registers the resource through the SDK. No manual
`ReadReportDashboard` handler is required.

The generated resource descriptor and `resources/read` body both carry
`_meta.ui`. The supported CSP fields are `connect_domains`,
`resource_domains`, `frame_domains` and `base_uri_domains`. Browser
permissions include camera, microphone, geolocation and clipboard write;
hosts may decline requested permissions. No attempt is made to compile JS in
the protoc plugin.

`source_file` supports other static types too: Markdown, plain text, XML
and binary assets. Go embed requires paths to remain inside the generated
package and to avoid parent traversal or hidden segments. The path must be a
literal file, not an external URL. `source_file` cannot be combined with
`content_field` or a dynamic URI template.

For runtime-provided HTML there is also the existing
`mcpruntime.RegisterAppResource` helper. Whichever approach is used,
`Tool.app_ui.resource_uri` must resolve to a registered resource URI.

## Protobuf-defined Markdown, text and binary

Without `content_field`, the generator encodes the whole protobuf result
as ProtoJSON and requires a JSON-compatible MIME type. To return text or
binary without JSON wrapping, use `content_field` selecting a singular
protobuf `string` or `bytes` field.

```proto
message SkillDocument {
  option (mcp.options.v1.resource) = {
    name: "skill"
    uri: "skill://easyp/SKILL.md"
    mime_type: "text/markdown"
    content_field: "markdown"
  };
  string markdown = 1;
}
```

```go
//go:embed skills/SKILL.md
var markdown string

func (Handler) ReadSkillDocument(_ context.Context) (*v1.SkillDocument, error) {
    return &v1.SkillDocument{Markdown: markdown}, nil
}
```

The MCP `resources/read` response contains the original Markdown, not a
JSON object. Static or template URIs are supported. A `bytes` field is
returned as an MCP blob, base64-encoded on the wire. The generator fails
early on unknown fields, repeated/map fields, or mismatched text/bytes MIME
types.

Serving `SKILL.md` as an MCP resource does *not* cause every host to install
it as a native Skill. The host must retrieve it or a tool/prompt must
reference it.

## OAuth resource-server policy via protobuf

```proto
option (mcp.options.v1.server) = {
  name: "secured-mcp"
  oauth: {
    resource_url: "https://mcp.example.com/mcp"
    authorization_servers: "https://auth.example.com"
    scopes: "reports:read"
    issuer: "https://auth.example.com"
    jwks_uri: "https://auth.example.com/.well-known/jwks.json"
    audience: "https://mcp.example.com/mcp"
    mcp_path: "/mcp"
  }
};

service ReportsAPI {
  rpc Publish(PublishRequest) returns (PublishResponse) {
    option (mcp.options.v1.method) = {
      required_scopes: "reports:write"
    };
  }
}
```

The generated `New<File>MCPHTTPHandler(server, verifier)` configures public
Protected Resource Metadata, validates bearer authorization through the SDK
and enforces global `scopes`. Generated tool registrations independently
enforce `required_scopes`. Thus a token with `reports:read` can access
the server but cannot call a tool requiring `reports:write`.

With `verifier=nil` the runtime verifies JWT access tokens using **RS256**
public keys fetched from the specified HTTPS JWKS endpoint; it rejects tokens
with missing or invalid signatures, issuer/audience, expiration or key ID.
Keys are cached and refreshed. This is intentionally limited to standard
RS256 access JWTs. An application can supply a custom
`auth.TokenVerifier` when using opaque tokens or another provider-specific
validation scheme; the verifier overrides the JWKS option.

The generator does **not** create an authorization server, issue tokens,
store OAuth secrets or handle users/passwords. OAuth login, consent, PKCE,
client configuration and identity provider management belong outside this
code generator. Never write access tokens or client secrets into `.proto`
options.

The generated OAuth HTTP handler is secure by default. Constructing an SDK
server and separately exposing it through some other HTTP handler does not
automatically apply the generated OAuth middleware: use the generated
`New<File>MCPHTTPHandler`. For local stdio use, authentication and identity
semantics are distinct from remote bearer authorization.

## Verification

- `go test ./mcpruntime` — token/JWKS validation, permissions,
  asset embedding, ProtoJSON and MCP Apps metadata.
- `go test ./internal/examplemcp` — generated SDK factories,
  real `resources/read`, OAuth HTTP, per-tool access scopes.
- `go test ./internal/codegen` — protobuf options,
  descriptor validation and generated-code goldens.
- `go test ./...` — cross-language repository-wide tests.

The current implementation is proposed in
[PR #5](https://github.com/easyp-tech/protoc-gen-mcp/pull/5); it is not
released or merged into `master` until its final CI checks pass.
