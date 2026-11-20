# Standalone Swift MCP Server

This example is structured like a user-owned SwiftPM project. It has its own
`Package.swift`, `easyp.yaml`, protobuf contract, generated Swift sources and
a server that runs over stdio or stateless Streamable HTTP.

Swift support is on `master` and ships in v1.1.0, which is not tagged yet;
the v1.0.0 release has no Swift target.

`Package.swift` takes the runtime from this checkout through a local path
dependency (`.package(name: "protoc-gen-mcp", path: "../..")`), so the example
always builds against the current source tree. A project outside the
repository adds the same package by URL, with no registry. Until v1.1.0 is
tagged, track the branch:

```swift
.package(url: "https://github.com/easyp-tech/protoc-gen-mcp.git", branch: "master"),
// products: ProtocGenMCPRuntime, ProtocGenMCPHTTP
```

After the v1.1.0 release, use `from: "1.1.0"` instead.

## Generate

Install `protoc-gen-swift` (for example `brew install swift-protobuf`), then:

```bash
make generate
```

`easyp.yaml` runs `protoc-gen-swift` and the local `protoc-gen-mcp` with
`lang=swift`. Both write to `Sources/NotebookServer/Generated`, so
`proto/notebook.mcp.swift` sits next to `proto/notebook.pb.swift` in one
module. Outside this repository, point the plugin at
`go run github.com/easyp-tech/protoc-gen-mcp/cmd/protoc-gen-mcp@master` until
v1.1.0 is released; v1.0.0 binaries do not know `lang=swift`.

## Build And Run

```bash
make build
make run
```

The server exposes:

- `notebook_CreateNote`
- `notebook_SearchNotes`
- `notebook_DeleteNote` (needs `notebook:write`)
- `notebook_Health`

plus the `notebook://notes/{id}` resource template and the `review_note`
prompt. `Notebook.swift` implements the generated `NotebookAPIToolHandler`,
`NotebookPromptHandler` and `NotebookResourceHandler` protocols, and
`main.swift` builds the server with the generated
`newNotebookMCPServer(handlers)` and starts it with `serve(server, transport:)`.

## OAuth Over HTTP

`option (mcp.options.v1.server)` in the proto makes the server an OAuth
protected resource over stateless Streamable HTTP: every request needs
`notebook:read`, and `notebook_DeleteNote` also needs `notebook:write`. Over
stdio there is no token, so `notebook_DeleteNote` is denied inside the call.
Start the server with local demonstration tokens (loopback only, never in
production):

```bash
swift run NotebookServer --http --demo-auth
```

Then walk through discovery and the scope step-up:

```bash
curl -s http://127.0.0.1:8080/.well-known/oauth-protected-resource/mcp
for token in demo-read demo-write; do
  curl -si http://127.0.0.1:8080/mcp \
    -H "Authorization: Bearer $token" \
    -H 'Content-Type: application/json' \
    -H 'Accept: application/json, text/event-stream' \
    -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"notebook_DeleteNote","arguments":{"id":"note-1"}}}'
done
```

`demo-read` gets `403` with `WWW-Authenticate: Bearer error="insufficient_scope",
scope="notebook:read notebook:write"` before the handler runs; `demo-write`
gets the tool result. Without `--demo-auth` the server verifies RS256 JWTs
against the issuer and JWKS declared in the proto.

`HTTP.swift` mounts the generated `MCPHTTPHandler` in Hummingbird; the handler
takes the SDK's `HTTPRequest` and returns an `MCPHTTPResponse`, so Vapor or
SwiftNIO adapters look the same.
