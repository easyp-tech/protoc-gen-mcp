# Protobuf-first Go MCP server, MCP Apps and OAuth

The Go target uses the official
[Model Context Protocol Go SDK](https://github.com/modelcontextprotocol/go-sdk).
Protocol negotiation, Streamable HTTP, sessions, tools, prompts and resources
are SDK-owned. `protoc-gen-mcp` owns only protobuf descriptions, generated
bindings, ProtoJSON/JSON Schema adapters and declarative server wiring.

## Migrating Go applications from 0.7.0

This is a **breaking Go migration** to `github.com/modelcontextprotocol/go-sdk`
v1.8.0, requiring Go 1.26+. Regenerate `*.mcp.go` with the updated plugin
through your Easyp generation configuration, update handwritten handlers and
server wiring, then rebuild and test your application. Do not edit generated
files or mix old generated bindings with the new runtime. Protobuf tool
request/response handlers and the ProtoJSON contract stay the same.

Import SDK types from `github.com/modelcontextprotocol/go-sdk/mcp`:

| Removed or changed API | Replacement |
| --- | --- |
| `*mcpruntime.Server`, `mcpruntime.NewServer(name, version)` | `*mcp.Server`, `mcp.NewServer(&mcp.Implementation{Name: name, Version: version}, nil)`, or a generated `New<File>MCPServer` factory |
| `mcpruntime.ServeStdio(ctx, server)` | `server.Run(ctx, &mcp.StdioTransport{})` |
| `mcpruntime.ServeIO` | `server.Run(ctx, &mcp.IOTransport{Reader: reader, Writer: writer})`; the streams must implement `io.ReadCloser` / `io.WriteCloser` |
| `mcpruntime.NewStreamableHTTPHandler`, `ServeStreamableHTTP`, `StreamableHTTPOptions` | Generated `New<File>MCPHTTPHandler` for configured stateless HTTP, or SDK `mcp.NewStreamableHTTPHandler` / `mcp.StreamableHTTPOptions` and `net/http` serving |
| `mcpruntime.Session`, `WithSession`, `SessionFromContext` | SDK `*mcp.ServerSession`, available as `req.Session` in SDK handlers/middleware, or returned by `server.Connect`; there is no compatibility context-session accessor |
| `Session.SendNotification` | SDK operations such as `req.Session.NotifyProgress(ctx, params)`, `req.Session.Log(ctx, params)` or `server.ResourceUpdated(ctx, params)`; SDK registry changes trigger supported list-change notifications |
| `Server.HandleRaw` | Connect an SDK client/server with `mcp.NewInMemoryTransports` for tests, or use SDK stdio/HTTP transports |
| `mcpruntime.RegisterProtoTool` | Regenerate bindings: their adapter is now `mcpruntime.RegisterSDKProtoTool`; application code should continue calling `Register<Service>Tools` |
| `[]mcpruntime.PromptMessage` | `[]*mcp.PromptMessage`, with `mcp.Content` implementations such as `&mcp.TextContent{Text: text}` |
| `[]mcpruntime.Resource` | `[]mcp.Resource` in generated resource-list handlers |
| `mcpruntime.Ptr` | Removed unused helper; SDK `Annotations.Priority` is a scalar. Use `proto.Bool` where generator adapter annotations require pointers |
| Legacy resource-content types | `[]*mcp.ResourceContents`, also returned by `mcpruntime.MarshalResourceContent` |

Prompt handlers now return pointer elements. For example, the messages portion
of a generated prompt implementation becomes:

```go
return []*mcp.PromptMessage{{
    Role: "user",
    Content: &mcp.TextContent{Text: "Review this code."},
}}, nil
```

SDK resource annotations use `mcp.Annotations{Priority: 0.8,
Audience: []mcp.Role{"user", "assistant"}}`; priority is a value, not a
pointer. SDK icons use `mcp.Icon{Source: url, MIMEType: mimeType}`.
The generator-owned `mcpruntime.ToolAnnotations`, `Icon`, `ToolSpec` and
registration options remain adapters, so do not blindly replace every
`mcpruntime` import. Generated protobuf handlers do not receive an SDK request
or session argument; applications needing SDK-specific session behavior must
integrate through the SDK's handlers/middleware.

### HTTP contract: stateless helpers and stateful SDK servers

`New<File>MCPHTTPHandler`, `mcpruntime.NewConfiguredHTTPHandler` and
`mcpruntime.NewOAuthResourceHandler` deliberately use
`mcp.StreamableHTTPOptions{Stateless: true}`. This is a runtime contract, not an
option inferred from whether OAuth or MCP Apps is enabled:

- The SDK creates a temporary session for each POST and does not read or set
  `MCP-Session-Id`. GET and DELETE return 405 after any authentication checks.
- There is no legacy standalone GET listen stream or configured event store
  for `Last-Event-ID` replay. Code depending on the 0.7.0 persistent-session
  contract must choose a different transport setup.
- Stateless does **not** mean JSON-only responses or no notifications. SDK
  notifications associated with an active request can reach its SSE response;
  keep the incoming request context. The MCP 2026-07-28
  `subscriptions/listen` POST can remain open for subscribed list-change
  notifications. Detached background notifications are not a replacement for
  that request/subscription lifecycle.
- Server-initiated JSON-RPC requests cannot use the stateless transport.
  Protocol negotiation and newer interaction mechanisms remain SDK-owned.

For applications that still require legacy session IDs, GET streams and
resumability, configure the SDK directly. Stateful SDK transport supports
legacy protocols such as 2025-11-25, **not** the sessionless 2026-07-28
protocol. `EventStore` is needed for replay; `Stateless: false` alone does not
enable it. For example, using `context`, `net/http`, `time`, and the SDK's
`auth`, `mcp` and `oauthex` packages:

```go
// server already has your generated tools/prompts/resources registered.
// verifier must validate signature, issuer, audience and expiration.
// Require a stable, nonempty principal ID for SDK session-owner checks.
requireIdentity := func(ctx context.Context, token string, req *http.Request) (*auth.TokenInfo, error) {
    info, err := verifier(ctx, token, req)
    if err != nil {
        return nil, err
    }
    if info == nil || info.UserID == "" {
        return nil, auth.ErrInvalidToken
    }
    return info, nil
}
handler := mcp.NewStreamableHTTPHandler(
    func(*http.Request) *mcp.Server { return server },
    &mcp.StreamableHTTPOptions{
        Stateless: false,
        SessionTimeout: 5 * time.Minute,
        EventStore: mcp.NewMemoryEventStore(nil),
    },
)
protected := auth.RequireBearerToken(requireIdentity, &auth.RequireBearerTokenOptions{
    Scopes: []string{"reports:read"},
    ResourceMetadataURL: "https://mcp.example.com/.well-known/oauth-protected-resource/mcp",
})(handler)
mux := http.NewServeMux()
mux.Handle("/mcp", protected)
// RFC 9728: metadata for https://mcp.example.com/mcp lives under the resource path.
mux.Handle("/.well-known/oauth-protected-resource/mcp", auth.ProtectedResourceMetadataHandler(
    &oauthex.ProtectedResourceMetadata{
        Resource: "https://mcp.example.com/mcp",
        AuthorizationServers: []string{"https://auth.example.com"},
        ScopesSupported: []string{"reports:read", "reports:write"},
    },
))
securedHandler := http.NewCrossOriginProtection().Handler(mux)
// Serve securedHandler behind HTTPS using your application's HTTP server.
// Call handler.Close() when the application shuts down to close SDK sessions.
```

This route is application-owned: replace the example URLs/scopes and apply
authentication to **every** MCP request, including GET and DELETE. A session ID
is not a credential. The SDK checks session ownership only when the verifier
sets a nonempty `auth.TokenInfo.UserID`; it must identify the same principal
consistently, including issuer/tenant distinctions where applicable. The
built-in JWKS verifier maps JWT `sub` to `UserID` but does not require `sub`,
so a stateful application must enforce this itself, as above.
The in-memory event store is process-local and does not survive restarts;
multi-instance routing and durable replay require application design.

Constructing a generated SDK server does **not** install its generated HTTP
OAuth policy on this separate handler. Retain protected-resource discovery,
token/scope checks, HTTPS and cross-origin protection explicitly. Review the
SDK's [transport options](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/mcp/streamable.go)
and [authentication types](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/auth/auth.go)
before adopting the stateful alternative.

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
  features. `<File>` is the exported CamelCase basename without `.proto`: for
  example, `resources.proto` produces `NewResourcesMCPServer`, scoped to the
  resolved `go_package`. Moving a file between directories does not rename
  the canonical API.
- `New<File>MCPHTTPHandler(server, verifier)` when a server option exists;
  the verifier argument may be nil if the configured OAuth provider supports
  the built-in JWKS flow.

Prompt/resource types and registration helpers use the same basename rule.
Generation rejects colliding names within a Go package, including collisions
with protobuf declarations. Rename the basename/declaration or use a different
`go_package`. Existing descriptor-path names remain deprecated aliases/wrappers
for the current paths; migrate callers to canonical names before moving files.

Omitting the server option preserves the existing service registration API. When a file
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

Discovery advertises the union of global scopes and the registered generated
tools' required scopes, including tools registered after the HTTP handler is
constructed. Advertising a tool scope does not make it a global requirement;
do not add method-only scopes to `server.oauth.scopes`
just to make them discoverable.

The generated OAuth HTTP handler rejects missing global or per-tool scopes
**before dispatch** with HTTP 403 and a `WWW-Authenticate: Bearer` challenge
containing `error="insufficient_scope"`, `scope` (the sorted union needed for
that request), and `resource_metadata`. The official SDK client can use its
`StreamableClientTransport.OAuthHandler` to request additional authorization
and retry; no business handler runs before the challenge. Legacy batches are
checked as a whole so retrying cannot repeat partially executed operations.
Authentication and token expiration checks happen before reading the body;
the preflight uses the SDK's 4 MiB request-body limit, including chunked input.

The callback also retains its scope guard for SDK integrations outside this
HTTP helper. Those integrations receive an `isError` tool result and must
provide their own HTTP step-up wiring. Direct SDK tools that bypass generated
registration also own their authorization policy. A client without an
`OAuthHandler` receives an HTTP transport error on denial; the Go SDK may
close that client session, so obtain appropriate authorization and reconnect.
The stateless server itself has no session to revoke.

With `verifier=nil` the runtime verifies JWT access tokens using **RS256**
public keys fetched from the specified HTTPS JWKS endpoint; it rejects tokens
with missing or invalid signatures, issuer/audience, expiration or key ID.
This is intentionally limited to standard RS256 access JWTs. An application
can supply a custom `auth.TokenVerifier` when using opaque tokens or another
provider-specific validation scheme; the verifier overrides the JWKS option.

### JWKS cache and identity-provider outages

Successful JWKS responses are cached for five minutes. A refresh performs
network I/O outside the cache mutex, and concurrent callers share one
in-flight fetch. Every caller can cancel independently without canceling that
shared fetch or causing failure backoff for other requests. Requests for known
keys in a still-fresh cache do not wait for a rotation refresh. Fetches have
a five-second deadline even when a custom HTTP client is supplied.

Failed refreshes use exponential retry delays of 1, 2, 4, 8, 16, 32 and then
60 seconds, capped at one minute and reset after success. Unknown `kid`
refreshes remain throttled to at most once per minute so attacker-controlled
key IDs cannot cause an unbounded fetch loop.

Past the five-minute TTL, a token signed with a **known** key is still
accepted immediately while the cache revalidates in the background, so
requests never wait on the IdP for a key the server already holds. Known keys
stay usable for `JWKSVerifierConfig.MaxStale` after the last successful fetch
(one hour by default); if every refresh fails for that long, verification
**fails closed**. A key the IdP removes stops being accepted at the first
successful refresh, so stale keys only extend trust while the IdP is
unreachable, which is also when it cannot issue new tokens.

Unknown or rotated `kid`s are never served from a stale cache: past the TTL
they wait for a refresh, and during backoff they are rejected without another
network fetch. Set `MaxStale` to a negative value for strict behaviour, where
every key fails closed at the TTL and an IdP outage longer than five minutes
interrupts authentication. Generated `New<File>MCPHTTPHandler` uses the
default; applications needing another availability/revocation policy can pass
a verifier built with `mcpruntime.NewJWKSVerifier` or their own.

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

Protected-resource metadata is served where RFC 9728 places it for the
configured `resource_url`: `https://mcp.example.com/mcp` publishes
`/.well-known/oauth-protected-resource/mcp`, which `WWW-Authenticate` challenges
reference and SDK clients probe first. The root
`/.well-known/oauth-protected-resource` keeps answering for older clients.
`mcp_path` may not live under `/.well-known/`.

Tools with `required_scopes` fail closed without a bearer token. Over stdio,
an in-memory transport, or `NewConfiguredHTTPHandler` without an OAuth config
there is no token, so such tools always return `permission denied` tool
errors; only tools without `required_scopes` are callable there. Serve scoped
tools through the generated OAuth handler, or register them on a separate
server for local use.

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


## Tasks compatibility

Go generation rejects visible `execution.task_support: TASK_SUPPORT_REQUIRED`
declarations instead of silently running task-only operations synchronously.
Omitted, `TASK_SUPPORT_NONE` and `TASK_SUPPORT_OPTIONAL` declarations generate
ordinary synchronous tools: `OPTIONAL` only permits task invocation, so a Go
server that does not advertise `execution.taskSupport` still satisfies it and
protobuf files shared with Tasks-capable targets keep generating for Go. These protobuf declarations describe MCP 2025-11-25 Tasks,
which the pinned official Go SDK cannot implement through its public
registration API: `tools/call` loses the task request field before receiving
middleware, and built-in methods cannot be replaced by custom methods.

The distinct MCP 2026-07-28
[`io.modelcontextprotocol/tasks` extension](https://modelcontextprotocol.io/extensions/tasks/overview)
uses server-directed task creation and `tasks/get` terminal results; it is
not implemented or advertised here. Supporting either lifecycle requires a
separately verified SDK integration, including ownership, cancellation and
bounded retention. The compatibility gate tests this boundary against both
the pinned and latest released SDK, so upstream support changes are visible.
See the [upstream Go SDK Tasks issue](https://github.com/modelcontextprotocol/go-sdk/issues/626).
