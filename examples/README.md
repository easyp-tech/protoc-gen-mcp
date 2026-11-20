# protoc-gen-mcp Examples

One runnable example per target language. Each standalone project has its own
`easyp.yaml` and lockfile, the way a user-owned project would. The numbers are
stable directory names, not an order: gaps are examples that were removed or
merged into these ones.

| Example | Language | What it shows |
|---|---|---|
| [1_helloworld](./1_helloworld) | Go | Minimal quickstart: one RPC, generated bindings, `mcp.Server` wiring. |
| [11_protobuf_first_mcp](./11_protobuf_first_mcp/README.md) | Go | Full showcase: generated server factory, prompts, raw/UI resources, OAuth and scoped tools. |
| [5_python_standalone](./5_python_standalone) | Python | Python-only project with `pyproject.toml`, generated dataclass handlers, stdio and an OAuth-protected HTTP mode. |
| [6_java_standalone](./6_java_standalone) | Java | Gradle project with a generated `lang=java` sidecar, stdio and an OAuth-protected Jetty servlet. |
| [7_kotlin_standalone](./7_kotlin_standalone) | Kotlin | Gradle project with Java + Kotlin protobuf output and a `lang=kotlin` sidecar, stdio and an OAuth-protected Ktor route. |
| [8_typescript_standalone](./8_typescript_standalone) | TypeScript | npm project with Protobuf-ES output, a `lang=typescript` sidecar, stdio and an OAuth-protected HTTP mode. JavaScript projects consume its compiled `.js` and `.d.ts` output. |
| [9_swift_standalone](./9_swift_standalone) | Swift (`master`, ships in v1.1.0) | SwiftPM project with swift-protobuf output, a `lang=swift` sidecar, stdio and an OAuth-protected HTTP mode on Hummingbird. |

## Prerequisites

- [easyp](https://github.com/easyp-tech/easyp) for code generation.
- Go 1.26+ for the Go examples.
- Python 3.10+ for `5_python_standalone`.
- JDK 17+ and Gradle 9.2+ for `6_java_standalone` and `7_kotlin_standalone`.
- Node.js and npm for `8_typescript_standalone`.
- A Swift 6.1+ toolchain and `protoc-gen-swift` (`brew install swift-protobuf`) for `9_swift_standalone`.

## Running

All commands below run from the repository root. Regenerate every example
from its `.proto` definitions (the generated Java and Kotlin sources are not
committed, so do this once before building examples 6 and 7):

```bash
make -C examples generate
```

The `mcp/options/v1/options.proto` import is resolved through the `deps` entry
in each `easyp.yaml` and pinned by its lockfile, so the examples do not depend
on the repository-root options package.

Then run the example you need. Python, TypeScript and the JVM examples use
the runtime built from this repository (`runtime/python`, `runtime/typescript`,
`runtime/jvm`), because no runtime is published to PyPI, npm or Maven Central
yet; build the TypeScript runtime once before example 8.

```bash
(cd examples/1_helloworld && go run main.go)
go run ./examples/11_protobuf_first_mcp -transport=http -demo-auth
(cd examples/5_python_standalone && make setup && make run)
(cd examples/6_java_standalone && gradle --no-daemon installDist && build/install/todo-java-standalone/bin/todo-java-standalone)
(cd examples/7_kotlin_standalone && gradle --no-daemon installDist && build/install/todo-kotlin-standalone/bin/todo-kotlin-standalone)
(cd runtime/typescript && npm ci && npm run build)
(cd examples/8_typescript_standalone && make build && make run)
(cd examples/9_swift_standalone && make run)
```

The JVM start scripts serve stdio. In either JVM project,
`gradle --no-daemon run --args="--http --demo-auth"` starts the
OAuth-protected HTTP mode.

MCP clients such as `@modelcontextprotocol/inspector` often render omitted
tool-annotation hints with client-side defaults. Set hints such as
`destructive_hint: false` explicitly in the proto when the UI must be
deterministic.
