# Standalone Kotlin MCP Server

This example is structured like a user-owned Kotlin project. It has its own
`easyp.yaml`, Gradle build, protobuf contract, generated source directories,
and stdio MCP server.

## Generate

```bash
make generate
```

`easyp.yaml` generates three source sets:

- Java protobuf classes in `src/generated/main/java`
- Kotlin protobuf helpers in `src/generated/main/kotlin`
- the `lang=kotlin` MCP sidecar in `src/generated/main/kotlin`

The proto is JVM-native and intentionally does not declare `go_package`.
`protoc-gen-mcp` synthesizes the internal metadata that `protogen` needs for
`lang=kotlin`.

`with_imports: true` on Java protobuf generation is used so the local
`mcp.options.v1` Java extension class is generated from the Easyp dependency.
The build removes generated `com.google.protobuf` sources afterward and uses
the protobuf Java jar for Google runtime classes.

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

```kotlin
val server = newTodoMCPServer(handlers)
```

Over HTTP the same handlers are served by the generated Ktor route
`installTodoMCPHttp(handlers, verifier)` on the CIO engine.

The handwritten server lives under `src/main/kotlin` and implements the
generated `TodoAPIToolHandler` interface.

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
