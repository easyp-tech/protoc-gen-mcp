# Standalone Java MCP Server

This example is structured like a user-owned Java project. It has its own
`easyp.yaml`, Gradle build, protobuf contract, generated source directory, and
stdio MCP server.

## Generate

```bash
make generate
```

`easyp.yaml` does two jobs:

- resolves `mcp/options/v1/options.proto` through `deps`
- runs Java protobuf generation plus `protoc-gen-mcp` with `lang=java`

The proto is Java-native and intentionally does not declare `go_package`.
`protoc-gen-mcp` synthesizes the internal metadata that `protogen` needs for
`lang=java`.

`with_imports: true` is used so the local `mcp.options.v1` Java extension
class is generated from the Easyp dependency. The build removes generated
`com.google.protobuf` sources afterward and uses the protobuf Java jar for
Google runtime classes.

The local repository version uses:

```yaml
command: ["go", "run", "../../cmd/protoc-gen-mcp"]
```

In an external project, replace it with the released plugin binary or module
entrypoint you want to pin.

## Build And Run

```bash
make build
make run
```

The server builds everything the proto declares through the generated factory:

```java
TodoMcp.installTodoMCPServer(transportProvider, handlers);
```

Over HTTP the same handlers are installed on a `McpHttpServer` and served by
`TodoMcp.newTodoMCPHttpServlet(server, verifier)` in Jetty.

The generated source lives under `src/generated/main/java` after generation.
The handwritten server lives under `src/main/java` and implements the generated
`TodoMcp.TodoAPIToolHandler` interface.

## OAuth Over HTTP

`option (mcp.options.v1.server)` in the proto makes the server an OAuth
protected resource over stateless Streamable HTTP: every request needs
`todo:read`, and `todo_DeleteTask` also needs `todo:write`. Over stdio there is no
token, so `todo_DeleteTask` is denied inside the call. Start the server with local
demonstration tokens (loopback only, never in production):

```bash
gradle --no-daemon run --args="--http --demo-auth"
```

Then walk through discovery and the scope step-up:

```bash
curl -s http://127.0.0.1:8080/.well-known/oauth-protected-resource/mcp
for token in demo-read demo-write; do
  curl -si http://127.0.0.1:8080/mcp \
    -H "Authorization: Bearer $token" \
    -H 'Content-Type: application/json' \
    -H 'Accept: application/json, text/event-stream' \
    -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"todo_DeleteTask","arguments":{"id":"task-1"}}}'
done
```

`demo-read` gets `403` with `WWW-Authenticate: Bearer error="insufficient_scope",
scope="todo:read todo:write"` before the handler runs; `demo-write` gets the
tool result. Without `--demo-auth` the server verifies RS256 JWTs against the
issuer and JWKS declared in the proto.
