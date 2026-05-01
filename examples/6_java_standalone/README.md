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

The server registers generated tools through:

```java
TodoMcp.registerTodoAPITools(transportProvider, new Handler(), "todo");
```

The generated source lives under `src/generated/main/java` after generation.
The handwritten server lives under `src/main/java` and implements the generated
`TodoMcp.TodoAPIToolHandler` interface.
