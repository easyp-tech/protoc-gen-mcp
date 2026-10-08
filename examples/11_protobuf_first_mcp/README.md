# Runnable protobuf-first MCP showcase (Go)

This example demonstrates the **current Go generator** from a single
[protobuf contract](proto/showcase.proto). It is an application you can run,
not a collection of unrelated snippets.

| Feature | What the example demonstrates |
| --- | --- |
| Server | File-level `server` option generates a typed SDK server factory |
| Tools | Typed protobuf/JSON Schema tool handlers, one with MCP Apps `_meta.ui` |
| Authorization | Server-wide `showcase:read` and tool-specific `showcase:write` |
| OAuth | Discovery, bearer middleware, a custom verifier for *demo-only* tokens |
| JWT/JWKS | Protobuf settings for production RS256 verification (fixture endpoints) |
| Prompts | `showcase_guide`, declared on a protobuf message |
| Dynamic resources | JSON status, text/plain URI template, raw binary attachment |
| Embedded assets | A real `SKILL.md` and a self-contained HTML MCP App resource |
| Browser metadata | CSP domains, clipboard request and border preference |
| Transports | stdio and stateless Streamable HTTP via the official Go MCP SDK |
| Tests | Real MCP SDK client exercises the contract end-to-end |

## Run it

From the **repository root** (Go 1.26 is required by `go.mod`):

**Terminal A — start the loopback-only OAuth demonstration**

```bash
go run ./examples/11_protobuf_first_mcp -transport=http -addr=127.0.0.1:8080 -demo-auth
```

**Terminal B — explore the API with the actual Go MCP SDK client**

```bash
# Read-only user: can use tools and resources, cannot publish a report.
go run ./examples/11_protobuf_first_mcp/client -token=demo-read

# Read/write user: the same publish tool succeeds.
go run ./examples/11_protobuf_first_mcp/client -token=demo-write
```

The client prints the negotiated MCP protocol, tools, tool `_meta.ui`,
prompt, resource list, resource templates, retrieved text and binary content,
plus the outcome of a scope-protected tool call.

Alternatively, use curl to inspect the OAuth discovery and missing-token
challenge:

```bash
curl -i http://127.0.0.1:8080/.well-known/oauth-protected-resource
curl -i http://127.0.0.1:8080/mcp
```

The second request must return **401 Unauthorized**. The demonstration tokens
are hard-coded fixtures, **not legitimate JWTs**. The `-demo-auth` flag
explicitly opts in to these tokens and refuses non-loopback bind addresses.
Do not use this mode on a public server, reverse proxy, container port mapping,
or production deployment. The OAuth issuer, authorization server and JWKS URLs
in [showcase.proto](proto/showcase.proto) are illustrative placeholders;
this example is *not* a complete OAuth authorization server.

**stdio mode** does not require OAuth configuration:

```bash
go run ./examples/11_protobuf_first_mcp -transport=stdio
```

You can attach an MCP Inspector or another stdio client. The write tool remains
scope-protected and will refuse calls with no authenticated token context;
use the HTTP demo to exercise write access.

## Verify everything

```bash
go test ./examples/11_protobuf_first_mcp/...
```

The integration test exercises:

- MCP initialization and auto-advertised `io.modelcontextprotocol/ui`
  extension.
- `tools/list`, ProtoJSON structured response and UI resource reference.
- `prompts/list` and `prompts/get`.
- `resources/list`, `resources/templates/list` and `resources/read`.
- `text/markdown` SKILL.md and `text/html;profile=mcp-app` sourced
  directly from embedded files; JSON, raw text and MCP binary blobs.
- Resource `_meta.ui` with generated CSP and sandbox permission request.
- Public OAuth protected-resource metadata, 401 when unauthenticated.
- Denial with read-only token, success with read/write token, state change
  reflected in the JSON resource.
- Refusal to bind insecure demo tokens to a non-loopback interface.

The root CI also compiles and checks that regeneration produces no diff.

## Regenerate code after editing protobuf

Committed `showcase.pb.go` and `showcase.mcp.go` files are generated and
checked in, so **you do not need Easyp to run the example**. To regenerate:

```bash
go install github.com/easyp-tech/easyp/cmd/easyp@v0.15.2-rc1
export PATH="$(go env GOPATH)/bin:$PATH"
easyp --cfg easyp.showcase.yaml generate -p examples/11_protobuf_first_mcp/proto -r .
go test ./examples/11_protobuf_first_mcp/...
```

This intentionally uses the **local repository's**
`mcp/options/v1/options.proto`, not the older released protobuf options
from the pinned standalone `examples/easyp.yaml`.

For additional details on the protobuf metadata and emitted SDK APIs, see
[the Go MCP Apps/OAuth guide](../../docs/mcp-apps-oauth.md).

## What the UI example does (and does not)

`Dashboard` declares a `ui://showcase/dashboard` resource in
[showcase.proto](proto/showcase.proto), and `GetOverview` points to it via
`app_ui.resource_uri`. The frontend is a self-contained HTML document
which renders its own explanation and demonstrates a clipboard permission
request; it needs **no Vite, npm or CDN** to be embedded.

A compliant MCP Apps host can retrieve and render the HTML. The included
Go SDK client verifies the protocol metadata and content but is **not a
browser/iframe host** and does not render the UI. This example intentionally
does **not** implement the MCP Apps JavaScript client bridge or browser-side
`callTool`; those concerns are separate from the generator's currently
supported resource and metadata features.

## Security and production migration

This demonstration deliberately keeps storage in memory and uses a local,
explicitly-enabled dummy verifier. For production, replace fixture
`resource_url`, `authorization_servers`, `issuer`, `jwks_uri` and
`audience` with your actual identity provider's HTTPS endpoints. Use the
generated `New<File>MCPHTTPHandler(server, nil)` for the built-in RS256/JWKS
verifier, or supply a real `auth.TokenVerifier` for provider-specific tokens.
The verifier must validate signature, issuer, audience and expiration.
Deploy behind HTTPS, enforce access policies for dynamic resources as needed,
and do not embed credentials in protobuf options or static UI assets.

The server factory registers objects **from this protobuf file**. For a
multi-file application, explicitly register additional generated handlers
on the returned `*mcp.Server`.

The complete rollout plan for equivalent Python, TypeScript, Java and Kotlin
generators is in [the cross-language roadmap](../../docs/cross-language-roadmap.md).
