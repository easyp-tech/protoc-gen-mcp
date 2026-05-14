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

The server registers generated tools through:

```kotlin
registerTodoAPITools(server, Handler(), namespace = "todo")
```

The handwritten server lives under `src/main/kotlin` and implements the
generated `TodoAPIToolHandler` interface.
