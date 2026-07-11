# protoc-gen-mcp

`protoc-gen-mcp` generates Go, Python, Kotlin, Java, and TypeScript MCP
bindings from protobuf definitions — covering **tools**, **prompts**, and
**resources**. JavaScript users consume the compiled TypeScript output and
declarations.

## Supported MCP Primitives

| Primitive | Go | Python | Kotlin | Java | TypeScript |
|-----------|:--:|:------:|:------:|:----:|:----------:|
| **Tools** | ✅ Full | ✅ Full | ✅ Full | ✅ Full | ✅ Full |
| **Prompts** | ✅ Full | ✅ Full | ✅ Full | ✅ Full | ✅ Full |
| **Resources** | ✅ Full | ⚠️ Interface | ⚠️ Interface | ⚠️ Interface | ⚠️ Interface |

## MVP

- protobuf is the source of truth
- generator emits typed Go, Python, Kotlin, Java, and TypeScript MCP bindings
- **Tools**: generated from `service` RPC methods with `(mcp.options.v1.method)`
- **Prompts**: generated from `message` types with `(mcp.options.v1.prompt)`
- **Resources**: generated from `message` types with `(mcp.options.v1.resource)`
  — static (fixed URI) and template (parameterized URI) resources
- Go runtime uses the official Go MCP SDK
- Python runtime targets the official MCP Python SDK with `google.protobuf`
- Kotlin runtime targets the official `io.modelcontextprotocol:kotlin-sdk-server`
  SDK
- Java runtime targets the official `io.modelcontextprotocol.sdk:mcp` SDK
- TypeScript runtime targets the official `@modelcontextprotocol/sdk` SDK,
  Protobuf-ES, and Ajv-backed raw JSON Schema validation
- generated Python handlers default to dataclasses and explicit `oneof`
  wrapper types from `*_mcp.py`; `python_handler=protobuf` opts into raw
  `*_pb2` handler request/response classes with the same registration helper;
  `python_handler=dataclass+protobuf` generates both surfaces side by side
- generated Kotlin handlers implement `<Service>ToolHandler` and are registered
  through `register<Service>Tools(server: Server, impl: <Service>ToolHandler, namespace: String? = null)`
- generated Java handlers implement nested `<Service>ToolHandler` interfaces
  inside a generated `<ProtoFile>Mcp` sidecar and are registered through
  `register<Service>Tools(McpServerTransportProvider transportProvider, <Service>ToolHandler impl, String namespace)`
- generated TypeScript handlers implement `<Service>ToolHandler` and are
  registered through `register<Service>Tools(server, impl, namespace?)`
- Direct lang=javascript generation is intentionally deferred; JavaScript
  projects use compiled TypeScript target `.js` files plus `.d.ts` declarations
- request and response JSON follows ProtoJSON rules
- runtime validation is driven by generated JSON Schema

## Repository Workflows

Use `easyp` for repository generation and checks.
The repository is currently aligned to `easyp v0.15.2-rc1`.

```bash
easyp --cfg easyp.yaml lint -p mcp -r .
easyp --cfg easyp.yaml generate -p mcp -r .
easyp --cfg easyp.test.yaml lint -p internal/testproto -r .
easyp --cfg easyp.test.yaml generate -p internal/testproto -r .
gradle --no-daemon -p examples/jvm :java-server:compileJava :kotlin-server:compileKotlin
gradle --no-daemon -p examples/jvm :java-server:installDist :kotlin-server:installDist
go test ./internal/examplemcp -run 'Test(Java|Kotlin).*OverStdio' -count=1
(cd examples/node/sdk-spike && npm ci && npm run typecheck && npm run build)
(cd examples/10_python_protobuf_standalone && make lint && make clean generate)
(cd examples/8_typescript_standalone && make lint && make clean build)
(cd examples/9_javascript_standalone && make clean build)
go test ./examples -run 'TestStandalonePython(Protobuf)?ExampleOverStdio' -count=1
go test ./examples -run 'TestStandalone(TypeScript|JavaScript)ExampleOverStdio' -count=1
go test ./...
goreleaser check
```

`easyp.yaml` is the main config for shipped protobuf APIs. `easyp.test.yaml`
is the development and test config for repository fixtures.

`mcp/options/v1/options.proto` carries its own explicit `go_package`, so
consumers do not need a special Easyp `go_package_prefix` override just to use
the MCP options package.

CI is implemented in [tests.yml](.github/workflows/tests.yml)
and runs config validation, Easyp lint, Easyp generation, a generated-file
freshness check, JVM compile/install gates, JVM stdio parity checks, Node
compile/build gates, generated Node stdio checks through `go test ./...`, and
standalone Python/Node example tests. Releases are implemented in
[release.yml](.github/workflows/release.yml)
and use [`.goreleaser.yaml`](.goreleaser.yaml)
to publish tagged builds of the `protoc-gen-mcp` binary. This repository does
not publish separate Java, Kotlin, TypeScript, or JavaScript runtime artifacts;
downstream projects compile generated source against their language SDK
dependencies.

## TypeScript And JavaScript Support

TypeScript support is implemented for Node stdio servers and verified through
the official MCP TypeScript SDK, Protobuf-ES, and Ajv. User-owned standalone
project layouts are shown in
[`examples/8_typescript_standalone`](examples/8_typescript_standalone/) and
[`examples/9_javascript_standalone`](examples/9_javascript_standalone/).

- Node prerequisites for the in-repo walkthrough are Go 1.24+, Node.js, npm,
  `easyp v0.15.2-rc1`, and the npm dependencies pinned by the examples.
- The Node generator mode is `lang=typescript`.
- The tested stack is `@modelcontextprotocol/sdk@1.29.0`,
  `@bufbuild/protobuf@2.12.0`, `@bufbuild/protoc-gen-es`, and `ajv@8.20.0`.
- Protobuf message classes are generated by Protobuf-ES as `_pb.ts` files with
  NodeNext-compatible `.js` import specifiers.
- `protoc-gen-mcp` generates `*_mcp.ts` sidecars that install low-level
  `tools/list` and `tools/call` handlers on the official SDK `Server`.
- Generated TypeScript preserves the shared raw JSON Schema and compiles it
  with Ajv, then maps request/response payloads through Protobuf-ES ProtoJSON.
- JavaScript consumption uses the compiled `.js` output and generated `.d.ts`
  declarations from the TypeScript target. Direct lang=javascript generation
  is intentionally deferred until compiled TypeScript output proves
  insufficient for common JavaScript projects.

Use the standalone TypeScript example when you want a full user-project
generation flow. Use the JavaScript example when you want to inspect how a
plain `.js` server imports compiled generated output without a separate
renderer.

## JVM Support

JVM support is implemented and CI-verified through the runnable
[`examples/jvm`](examples/jvm/README.md) workspace. User-owned standalone
project layouts are shown in
[`examples/6_java_standalone`](examples/6_java_standalone/) and
[`examples/7_kotlin_standalone`](examples/7_kotlin_standalone/).

- JVM prerequisites for the in-repo walkthrough are Go 1.24+, JDK 17+, and
  Gradle 9.2+.
- The JVM generator modes are `lang=java` and `lang=kotlin`.
- The Java path compiles generated protobuf Java output plus a generated
  `lang=java` MCP sidecar against the official
  `io.modelcontextprotocol.sdk:mcp` SDK.
- The Kotlin path follows the tested dual-generation flow from
  `examples/jvm/kotlin-server/build.gradle.kts`: Java protobuf output, Kotlin
  protobuf output, and then the `lang=kotlin` MCP sidecar are all required.
- The canonical runnable JVM verification path uses `installDist` plus the
  installed scripts under `examples/jvm/*/build/install/.../bin/*`, matching
  `internal/examplemcp/jvm_stdio_test.go` and CI.
- Standalone JVM protos can be Java/Kotlin-native and omit `go_package`; the
  generator synthesizes internal protogen metadata for `lang=java` and
  `lang=kotlin`.

Use the root README for the language matrix and repository workflow, inspect
the standalone Java/Kotlin directories for user-project shape, and use
[examples/jvm/README.md](examples/jvm/README.md) for the cross-language JVM
verification workspace.

## Test MCP Server

The repository also includes a runnable MCP server for manual client checks:

```bash
# stdio (default)
go run ./cmd/example-mcp-server

# Streamable HTTP (MCP spec 2025-11-25), localhost only by default
go run ./cmd/example-mcp-server -transport=http -addr=127.0.0.1:8080 -path=/mcp

python ./cmd/example-python-mcp-server/main.py
```

It serves the generated tools from `internal/testproto/example/v1` and is used
by the stdio smoke tests in `internal/examplemcp/stdio_test.go`.
The example server currently exposes:

- `example_CreateReport`
- `example_Health`
- `example_DescribeAdvancedShapes`
- `example_DescribeScalarShapes`

### Go Streamable HTTP (`mcpruntime`)

Go servers can expose the same generated tools over **Streamable HTTP** instead
of (or in addition to) stdio:

```go
mux := http.NewServeMux()
mux.Handle("/mcp", mcpruntime.NewStreamableHTTPHandler(server, mcpruntime.StreamableHTTPOptions{
    AllowedOrigins: []string{"http://localhost:3000"},
}))
// Prefer binding to 127.0.0.1 for local deployments (DNS-rebinding mitigation).
http.ListenAndServe("127.0.0.1:8080", mux)
```

Or:

```go
mcpruntime.ServeStreamableHTTP(ctx, "127.0.0.1:8080", server, mcpruntime.StreamableHTTPOptions{
    Path: "/mcp",
})
```

Behavior highlights:

- Single MCP endpoint: `POST` (JSON-RPC), `GET` (SSE listen), `DELETE` (session end)
- Multi-client sessions via `MCP-Session-Id` on `initialize`
- Origin validation (localhost Origins allowed by default when the allowlist is empty)
- Optional SSE POST responses with `PreferSSE: true`
- `Last-Event-ID` replay for GET streams (in-memory buffer)
- Legacy HTTP+SSE (2024-11-05) is **not** implemented

## Testing With MCP Inspector

Use the official
[MCP Inspector](https://modelcontextprotocol.io/docs/tools/inspector) for
interactive stdio checks while developing a server. The Inspector runs through
`npx` and starts the MCP server command you pass after the Inspector package.

For the Go example server:

```bash
npx -y @modelcontextprotocol/inspector go run ./cmd/example-mcp-server
```

For the Python example server:

```bash
npx -y @modelcontextprotocol/inspector python ./cmd/example-python-mcp-server/main.py
```

For the standalone raw protobuf Python example, create the example virtualenv
first and then point the Inspector at the local server:

```bash
(cd examples/10_python_protobuf_standalone && make setup)
(cd examples/10_python_protobuf_standalone && npx -y @modelcontextprotocol/inspector .venv/bin/python server.py)
```

For installed JVM examples, build the scripts first and then point the
Inspector at the installed application:

```bash
gradle --no-daemon -p examples/jvm :java-server:installDist :kotlin-server:installDist
npx -y @modelcontextprotocol/inspector ./examples/jvm/java-server/build/install/java-server/bin/java-server
npx -y @modelcontextprotocol/inspector ./examples/jvm/kotlin-server/build/install/kotlin-server/bin/kotlin-server
```

For standalone Node examples, build first and then point the Inspector at the
Node server entrypoint:

```bash
(cd examples/8_typescript_standalone && make build)
npx -y @modelcontextprotocol/inspector node ./examples/8_typescript_standalone/dist/server.js

(cd examples/9_javascript_standalone && make build)
npx -y @modelcontextprotocol/inspector node ./examples/9_javascript_standalone/src/server.js
```

When the Inspector UI opens, connect to the server, open the Tools tab, run
List Tools, then call a generated tool such as `example_Health` or
`example_CreateReport`. This is the fastest manual check that generated tool
names, schemas, annotations, ProtoJSON inputs, structured outputs, and server
stdio wiring are visible to an MCP client.

## Examples

We provide several standalone, runnable examples demonstrating generated MCP
tools, protobuf `options`, validation constraints, and integration with the
official Go, Python, Kotlin, Java, TypeScript, and JavaScript SDK paths. Check
out the
[examples/](examples/) directory for:
- [1_helloworld](examples/1_helloworld/) - Minimal Quickstart setup.
- [2_weather_api](examples/2_weather_api/) - Read-only queries, validation limits, Oneofs.
- [3_file_manager](examples/3_file_manager/) - Destructive tools and schema-based string parameter constraints.
- [4_crm_system](examples/4_crm_system/) - A full mock system with FieldMask partial updates, custom icons mapping, schemas nested types, and advanced array filters.
- [5_python_standalone](examples/5_python_standalone/) - A Python-only user-style project with its own `pyproject.toml`, `easyp.yaml`, generated bindings, and stdio server.
- [10_python_protobuf_standalone](examples/10_python_protobuf_standalone/) - A Python-only user-style project that opts into `python_handler=dataclass+protobuf` and implements the raw `*_mcp_pb.py` sidecar with `*_pb2` classes.
- [6_java_standalone](examples/6_java_standalone/) - A Java user-style project with its own Gradle build, `easyp.yaml`, protobuf contract, and generated MCP sidecar.
- [7_kotlin_standalone](examples/7_kotlin_standalone/) - A Kotlin user-style project with its own Gradle build, `easyp.yaml`, protobuf contract, and generated MCP sidecar.
- [8_typescript_standalone](examples/8_typescript_standalone/) - A TypeScript user-style project with its own npm package, `tsconfig.json`, `easyp.yaml`, Protobuf-ES output, generated MCP sidecar, and stdio server.
- [9_javascript_standalone](examples/9_javascript_standalone/) - A JavaScript consumption proof that imports compiled generated `.js` output and `.d.ts` metadata from the TypeScript standalone project.
- [jvm](examples/jvm/README.md) - A Java/Kotlin official SDK workspace with
  Gradle-managed protobuf generation, `lang=java` / `lang=kotlin` sidecars,
  `installDist` scripts, and stdio verification.

## Agent Skill

Install the [skills.sh](https://skills.sh/) agent skill to let your AI coding
assistant build MCP servers with protoc-gen-mcp:

```bash
npx skills add easyp-tech/protoc-gen-mcp-skill
```

The skill source lives in a separate repository:
[easyp-tech/protoc-gen-mcp-skill](https://github.com/easyp-tech/protoc-gen-mcp-skill).

The skill teaches agents the full workflow: define proto → configure easyp →
generate → implement handler → serve. It covers proto options, requiredness
policy, ProtoJSON contract, and common patterns.

## Generation With Easyp

The intended workflow is `easyp`, not manual `protoc` invocation. For mixed
Go/Python/JVM/Node projects, `easyp` can drive `protoc-gen-go`, the standard
Python protobuf generator, Protobuf-ES, and `protoc-gen-mcp` from one config.

Before running generation, make sure `protoc-gen-go` is installed and available
in `PATH` if your config uses the standard Go plugin:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
```

Example `easyp.yaml`:

```yaml
lint:
  use:
    - PACKAGE_DEFINED
    - PACKAGE_VERSION_SUFFIX
    - RPC_NO_CLIENT_STREAMING
    - RPC_NO_SERVER_STREAMING

generate:
  inputs:
    - directory:
        path: mcp
        root: "."
  plugins:
    - name: go
      out: .
      opts:
        paths: source_relative
    - name: python
      with_imports: true
      out: .
    - command: ["go", "run", "github.com/easyp-tech/protoc-gen-mcp/cmd/protoc-gen-mcp@latest"]
      out: .
      opts:
        paths: source_relative
    - command: ["go", "run", "github.com/easyp-tech/protoc-gen-mcp/cmd/protoc-gen-mcp@latest"]
      out: .
      opts:
        paths: source_relative
        lang: python
        python_runtime: google.protobuf
        # Optional: use raw *_pb2 handler request/response classes instead of
        # the default generated dataclass API.
        # python_handler: protobuf
        # Optional: generate both surfaces in one pass. This keeps *_mcp.py as
        # the dataclass sidecar and adds *_mcp_pb.py for raw *_pb2 handlers.
        # python_handler: dataclass+protobuf
    - command: ["go", "run", "github.com/easyp-tech/protoc-gen-mcp/cmd/protoc-gen-mcp@latest"]
      out: .
      opts:
        paths: source_relative
        lang: java
    - command: ["go", "run", "github.com/easyp-tech/protoc-gen-mcp/cmd/protoc-gen-mcp@latest"]
      out: .
      opts:
        paths: source_relative
        lang: kotlin
    - command: ["./node_modules/.bin/protoc-gen-es"]
      with_imports: true
      out: src/generated
      opts:
        target: ts
        import_extension: js
    - command: ["go", "run", "github.com/easyp-tech/protoc-gen-mcp/cmd/protoc-gen-mcp@latest"]
      out: src/generated
      opts:
        paths: source_relative
        lang: typescript
```

Typical commands:

```bash
easyp --cfg easyp.yaml validate-config
easyp --cfg easyp.yaml lint -p mcp -r .
easyp --cfg easyp.yaml generate -p mcp -r .
```

That generates language-specific protobuf output plus MCP sidecars such as
`*.pb.go`, `*_pb2.py`, `*.mcp.go`, `*_mcp.py`, Protobuf-ES `_pb.ts`, and
TypeScript `*_mcp.ts` files. The Python target currently supports only
`python_runtime=google.protobuf`; handler mode defaults to
`python_handler=dataclass`, and `python_handler=protobuf` opts into raw
`*_pb2` handlers. `python_handler=dataclass+protobuf` generates both surfaces
in one pass: `*_mcp.py` remains the dataclass sidecar and `*_mcp_pb.py` exposes
the raw protobuf handler sidecar. Generated Python output also emits a small
`mcp/__init__.py` bridge so `mcp.options.*` protobuf modules can coexist with
the official `mcp` SDK package in one import tree. No special Easyp override is
required for `mcp.options.v1`, because the package declares `go_package`
directly in `options.proto`. For reproducible builds, prefer pinning a specific
tag instead of `@latest`.

For JVM consumers, the language selectors are `lang=java` and `lang=kotlin`.
The Java path generates a Java sidecar alongside protobuf Java output. The
Kotlin path must follow the same rule as the in-repo Gradle example: Java
protobuf output, Kotlin protobuf output, and the `lang=kotlin` MCP sidecar are
all part of the working build graph. User-authored JVM protos do not need a
Go `go_package` option just to satisfy `protoc-gen-mcp`. See
[examples/6_java_standalone](examples/6_java_standalone/),
[examples/7_kotlin_standalone](examples/7_kotlin_standalone/), and
[examples/jvm/README.md](examples/jvm/README.md) for runnable layouts.

For TypeScript consumers, generate Protobuf-ES `_pb.ts` files first and then run
`protoc-gen-mcp` with `lang=typescript` into the same generated-source tree.
Use `import_extension=js` so emitted imports work under Node ESM and
`moduleResolution: NodeNext`. The generated MCP sidecar imports the official
`@modelcontextprotocol/sdk` low-level `Server`, preserves raw JSON Schemas,
validates them through Ajv, and maps payloads through Protobuf-ES ProtoJSON.
See [examples/8_typescript_standalone](examples/8_typescript_standalone/) for a
complete standalone project with pinned npm dependencies and checked-in
generated sources.

For JavaScript consumers, do not use `lang=javascript`; direct
lang=javascript generation is intentionally deferred. Compile the TypeScript
target and import the emitted `.js` files from JavaScript. The generated `.d.ts`
files provide editor and `// @ts-check` metadata. See
[examples/9_javascript_standalone](examples/9_javascript_standalone/) for the
tested consumption path.

For Python-only projects, omit the Go plugins and keep only the standard
`python` plugin plus `protoc-gen-mcp` with `lang=python`. User-authored Python
protos do not need a `go_package` option just to satisfy the generator; the
plugin synthesizes internal Go package metadata before building the descriptor
model. See [examples/5_python_standalone](examples/5_python_standalone/) for a
complete default dataclass-mode Python project with its own virtualenv setup.

The default Python handler mode is `python_handler=dataclass`: implement
against the dataclasses from `*_mcp.py`, not raw protobuf classes from
`*_pb2.py`. The generated Python runtime maps MCP JSON -> ProtoJSON -> `pb2` ->
dataclasses before calling your handler, then maps your dataclass response back
through protobuf serialization and output-schema validation.

If you already have server logic written against standard protobuf classes, add
`python_handler: protobuf` to the Python MCP plugin options. In that mode the
same generated registration helper accepts handlers typed with raw `*_pb2`
messages, while schemas, ProtoJSON parsing, validation, annotations, and
structured output stay generator-owned:

```python
import mcp.server.lowlevel
from proto import tasks_mcp, tasks_pb2


class TaskStore(tasks_mcp.TaskAPIToolHandler):
    def create_task(
        self,
        _ctx: tasks_mcp.ToolRequestContext,
        req: tasks_pb2.CreateTaskRequest,
    ) -> tasks_pb2.CreateTaskResponse:
        ...


server = mcp.server.lowlevel.Server("tasks-server")
tasks_mcp.register_task_api_tools(server, TaskStore())
```

See
[examples/10_python_protobuf_standalone](examples/10_python_protobuf_standalone/)
for the raw `*_pb2` standalone layout.

If you want both handler surfaces from one generator invocation, use
`python_handler: dataclass+protobuf`. The dataclass API is emitted to the
normal `*_mcp.py` module, and the raw protobuf API is emitted to `*_mcp_pb.py`
with the same protocol and registration names inside that module:

```python
from proto import tasks_mcp, tasks_mcp_pb, tasks_pb2

# Dataclass implementation uses tasks_mcp.TaskAPIToolHandler.
# Raw protobuf implementation uses tasks_mcp_pb.TaskAPIToolHandler.
tasks_mcp_pb.register_task_api_tools(server, RawTaskStore())
```

For optional fields and `oneof` groups, the handler-facing absence sentinel is
`UNSET`, not `None`. JSON `null` for a schema-optional field is mapped to
`UNSET` on input, and handlers should return `UNSET` when a field should remain
unset in the serialized protobuf output.

If your Python package imports other `.proto` files that must also resolve as
Python modules at runtime, make sure the standard Python generator emits those
imports too. In `easyp`, that typically means enabling `with_imports: true` on
the Python plugin for that package.

Generated `ToolAnnotations` are forwarded as declared in protobuf options. The
generator does not synthesize extra hint values that were not set in source
proto. Some MCP clients, including `@modelcontextprotocol/inspector`, display
omitted hints using their own client-side defaults. If you want UI badges to
stay unambiguous, set hints like `destructive_hint: false` explicitly instead
of relying on omission.

Example Python handler:

```python
import mcp.server.lowlevel
from weather_mcp import (
    GetCurrentWeatherRequest,
    GetCurrentWeatherRequestLocationCityVariant,
    GetCurrentWeatherResponse,
    register_weather_api_tools,
)


class WeatherAPI:
    def get_current_weather(self, _ctx, req: GetCurrentWeatherRequest) -> GetCurrentWeatherResponse:
        if not isinstance(req.location, GetCurrentWeatherRequestLocationCityVariant):
            raise ValueError("city lookup is required")
        return GetCurrentWeatherResponse(condition="Sunny", temperature=22.5)


server = mcp.server.lowlevel.Server("weather-mcp-server", version="1.0.0")
register_weather_api_tools(server, WeatherAPI())
```

Generated tool names never contain dots. The runtime joins the optional service
namespace and RPC tool name with underscores, so a service namespace
`weather.v1` and RPC `GetForecast` become tool name `weather_v1_GetForecast`.

Generated JSON Schemas use a proto3-driven requiredness policy: a singular field
WITHOUT the `optional` keyword is required. A singular field WITH the `optional` keyword 
is optional. `repeated`, `map`, and `oneof` fields are always optional.
Fields that are not required by that generated MCP schema accept explicit JSON `null`, so
MCP clients that pre-validate cached `inputSchema` do not reject otherwise
valid tool calls before they reach the server.

Generated schemas also emit examples for complex ProtoJSON forms. The `examples` option
in fields can be strictly typed through `ExampleValue` structured messages (e.g. integer,
string, array, and object representations). The generator synthesizes fallback examples
for maps, recursive messages, `Any`, special float encodings, and other advanced shapes
to make agent-side tool invocation more discoverable.

## Supported ProtoJSON Contract

The generator publishes MCP `inputSchema` and `outputSchema` that follow
ProtoJSON rather than plain protobuf reflection semantics.

- `int64` / `uint64` / `fixed64` / `sfixed64` / `sint64` are JSON strings
- `int32` / `uint32` / `fixed32` / `sfixed32` / `sint32` are JSON integers
- `float` / `double` accept JSON numbers and ProtoJSON special strings
  `NaN`, `Infinity`, `-Infinity`
- `bytes` use base64 strings
- enums use enum names
- `Timestamp` uses RFC 3339 strings
- `Duration` uses protobuf duration strings such as `"3600s"`
- `FieldMask` uses ProtoJSON field-mask strings
- `Struct`, `Value`, and `ListValue` map to arbitrary JSON values
- `Any` uses ProtoJSON object form with `@type`
- recursive messages are emitted through `$defs` / `$ref`
- top-level and nested `oneof` groups are expressed through JSON Schema
  constraints

Requiredness policy: field requiredness in the generated MCP JSON Schema is
determined entirely by proto3 syntax.
- A singular field WITHOUT the `optional` keyword is required.
- A singular field WITH the `optional` keyword is optional.
- `repeated`, `map`, and `oneof` fields are always optional.
Fields that are not schema-required accept explicit JSON `null`.

## Complete Proto Example

This example shows the current supported surface, including service, method,
and field options, all plain scalar families, maps, `oneof`, recursive
messages, ProtoJSON-special forms, selected well-known types, constraints, and
hidden RPCs.

```proto
syntax = "proto3";

package weather.v1;

option go_package = "github.com/acme/weather-mcp/weather/v1;weatherv1";

import "mcp/options/v1/options.proto";
import "google/protobuf/any.proto";
import "google/protobuf/duration.proto";
import "google/protobuf/empty.proto";
import "google/protobuf/field_mask.proto";
import "google/protobuf/struct.proto";
import "google/protobuf/timestamp.proto";
import "google/protobuf/wrappers.proto";

service WeatherAPI {
  option (mcp.options.v1.service) = {
    namespace: "weather"
    description: "Weather tools exported as MCP tools."
    icons: [{
      src: "https://example.com/weather-icon.png"
      mime_type: "image/png"
    }]
  };

  // Forecast returns a weather report.
  rpc Forecast(GetForecastRequest) returns (GetForecastResponse) {
    option (mcp.options.v1.method) = {
      name: "GetForecast"
      title: "Get forecast"
      description: "Fetch the forecast for a city."
      annotations: {
        read_only_hint: true
      }
    };
  }

  // Health returns an empty acknowledgement.
  rpc Health(google.protobuf.Empty) returns (HealthResponse) {
    option (mcp.options.v1.method) = {
      title: "Health check"
      description: "Verify that the MCP server is alive."
    };
  }

  // InternalDebug is omitted from generated tools.
  rpc InternalDebug(InternalDebugRequest) returns (InternalDebugResponse) {
    option (mcp.options.v1.method) = {
      hidden: true
    };
  }
}

message GetForecastRequest {
  option (mcp.options.v1.message) = {
    title: "Forecast Request"
    description: "Parameters to fetch the weather forecast."
  };

  // city is the primary lookup key and required by proto3 omission of `optional`.
  string city = 1 [(mcp.options.v1.field) = {
    description: "City name accepted by the upstream weather provider."
    examples: [{ string_value: "Paris" }]
    min_length: 2
    max_length: 100
  }];

  // units uses real proto3 optional presence.
  optional string units = 2 [(mcp.options.v1.field) = {
    default_value: { string_value: "metric" }
  }];

  // tags stays optional in generated MCP schema because it is repeated.
  repeated string tags = 3 [(mcp.options.v1.field) = {
    examples: [{ array_value: { items: [{ string_value: "urgent" }] } }]
    max_items: 5
    unique_items: true
  }];

  // labels demonstrates map<string, string>.
  map<string, string> labels = 4;

  // page demonstrates numeric integer boundaries.
  int32 page = 10 [(mcp.options.v1.field) = {
    minimum: 1
    default_value: { integer_value: 1 }
  }];
  
  // temperature_floor demonstrates float bounds.
  float temperature_floor = 20 [(mcp.options.v1.field) = {
    minimum: -50.0
    maximum: 60.0
  }];

  // details is automatically implicitly required by proto3.
  ForecastDetails details = 23;

  google.protobuf.Timestamp observed_at = 24;
  google.protobuf.Duration ttl = 25;
  google.protobuf.FieldMask mask = 26;
  google.protobuf.Struct filters = 27;

  // Any is treated generically.
  google.protobuf.Any detail_any = 39;

  RecursiveNode tree = 40;

  oneof selector {
    option (mcp.options.v1.oneof) = {
      description: "Select how to find the city."
      required: true
    };
    string city_alias = 41;
    int64 city_id = 42;
    ForecastDetails city_details = 43;
  }
}

message GetForecastResponse {
  string report_id = 1 [(mcp.options.v1.field) = { read_only: true }];
  int64 total_count = 2;
  ForecastMode mode = 3;
  ForecastDetails details = 4;
  repeated string warnings = 5;
  google.protobuf.Empty ack = 6;
}

message HealthResponse {
  google.protobuf.Empty ack = 1;
}

message InternalDebugRequest {
  string token = 1;
}

message InternalDebugResponse {}

message ForecastDetails {
  string label = 1;
}

message RecursiveNode {
  string name = 1;
  RecursiveNode child = 2;
  repeated RecursiveNode children = 3;
}

enum ForecastMode {
  option (mcp.options.v1.enum) = {
    title: "Forecast Mode"
    description: "Scope of the forecast request."
  };
  
  FORECAST_MODE_NONE = 0 [(mcp.options.v1.enum_value) = { hidden: true }];
  FORECAST_MODE_DAILY = 1;
  FORECAST_MODE_HOURLY = 2;
}
```

## Metadata Options

Import `mcp/options/v1/options.proto` in your `.proto` files to override
generation behavior.

### Service Options
Used via: `option (mcp.options.v1.service) = { ... };`

- `namespace`: Prepended to every generated tool name for this service (e.g. `weather_GetForecast`).
- `description`: Overrides the service description inferred from proto comments.
- `icons`: Defines default `[Icon]` mapping for all tools generated from this service.

### Method Options
Used via: `option (mcp.options.v1.method) = { ... };`

- `name`: Overrides the RPC segment of the generated tool name.
- `title`: Sets a human-readable title.
- `description`: Overrides the RPC description inferred from proto comments.
- `hidden`: Suppresses tool generation for this RPC method completely.
- `annotations`: Provides `ToolAnnotations` hints for agents (like `read_only_hint`, `destructive_hint`, `idempotent_hint`, `open_world_hint`).
  Omitted hints are left omitted in generated output; some clients render their
  own defaults for missing values.
- `icons`: Provides an array of `Icon` metadata for the tool, overriding the service default.
- `execution`: Defines `ExecutionOptions` like `task_support`.

### Field Options
Used via: `[(mcp.options.v1.field) = { ... }]`

- `description`: Overrides descriptions from proto comments.
- `examples`: Adds explicitly typed `ExampleValue` examples in the schema.
- `default_value`: Sets an explicit default using an `ExampleValue`.
- **String constraints:** `pattern` (Regex), `format` (e.g. `email`, `date-time`), `min_length`, `max_length`.
- **Number constraints:** `minimum`, `maximum`, `exclusive_minimum`, `exclusive_maximum`, `multiple_of`.
- **Array constraints:** `min_items`, `max_items`, `unique_items`.
- `read_only`: Marks a field as read-only.

### Other Structures
- **Message Options (`mcp.options.v1.message`)**: `title`, `description`, `examples`.
- **Enum Options (`mcp.options.v1.enum`)**: `title`, `description`.
- **EnumValue Options (`mcp.options.v1.enum_value`)**: `description`, `hidden` (often used to hide sentinel zeroes).
- **Oneof Options (`mcp.options.v1.oneof`)**: `description`, `required`.

### Prompt Options
Used via: `option (mcp.options.v1.prompt) = { ... };` on a message.

- `name`: Override the prompt name (default: snake_case of message name).
- `title`: Human-readable title.
- `description`: Description shown to clients.
- `icons`: Icons for the prompt.

Message fields become prompt arguments. Singular fields are required; `optional`
and `repeated` fields become optional arguments.

### Resource Options
Used via: `option (mcp.options.v1.resource) = { ... };` on a message.

- `uri`: Static URI for fixed resources (mutually exclusive with `uri_template`).
- `uri_template`: Parameterized URI template with `{param}` placeholders
  (mutually exclusive with `uri`).
- `name`: Human-readable display name.
- `description`: Description of the resource.
- `mime_type`: MIME type of the resource content (default: `application/json`).
- `annotations`: `ResourceAnnotations` with `audience` and `priority` hints.
- `icons`: Icons for the resource.

Static resources generate `Read<Message>(ctx)` handlers.
Template resources generate `List<Message>s(ctx)` + `Read<Message>(ctx, params...)` handlers.

Comments are also used as metadata:

- plain comment lines become descriptions
- `Example: ...` adds one schema example (though `FieldOptions.examples` offers stronger typing).
- `Examples: ... | ...` adds multiple schema examples.
- `Examples in JSON: ...` uses the raw JSON string as an inline example.

## Generated API And Server Integration

After `easyp --cfg easyp.yaml generate -p mcp -r .`, the generated package
exposes a typed handler interface plus a registration helper:

```go
type WeatherAPIToolHandler interface {
	Forecast(ctx context.Context, req *weatherv1.GetForecastRequest) (*weatherv1.GetForecastResponse, error)
	Health(ctx context.Context, req *emptypb.Empty) (*weatherv1.HealthResponse, error)
}

func RegisterWeatherAPITools(
	server *mcp.Server,
	impl WeatherAPIToolHandler,
	opts ...mcpruntime.RegisterOption,
) error
```

Typical server wiring looks like this:

```go
package main

import (
	"context"
	"log"

	weatherv1 "github.com/acme/weather/gen/weather/v1"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type handler struct{}

func (handler) Forecast(
	_ context.Context,
	req *weatherv1.GetForecastRequest,
) (*weatherv1.GetForecastResponse, error) {
	return &weatherv1.GetForecastResponse{
		ReportId:   "forecast-1",
		TotalCount: 42,
		Mode:       weatherv1.ForecastMode_FORECAST_MODE_DAILY,
		Details: &weatherv1.ForecastDetails{
			Label: req.GetDetails().GetLabel(),
		},
		Warnings: []string{"none"},
		Ack:      &emptypb.Empty{},
	}, nil
}

func (handler) Health(
	_ context.Context,
	_ *emptypb.Empty,
) (*weatherv1.HealthResponse, error) {
	return &weatherv1.HealthResponse{
		Ack: &emptypb.Empty{},
	}, nil
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "weather-mcp",
		Version: "v0.1.0",
	}, nil)

	if err := weatherv1.RegisterWeatherAPITools(server, handler{}); err != nil {
		log.Fatal(err)
	}

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
```

Generated runtime behavior:

- request arguments are validated against generated JSON Schema first
- request JSON is unmarshaled with `protojson.Unmarshal`
- handler receives typed protobuf messages
- response protobuf is marshaled with `protojson.Marshal`
- `structuredContent` carries the canonical ProtoJSON object
- text content mirrors the same payload for clients that still rely on text

For a service like the example above:

- `Forecast` is exposed as tool `weather_GetForecast`
- `Health` is exposed as tool `weather_Health`
- `InternalDebug` is omitted because `hidden = true`

The TypeScript target follows the same public shape with Protobuf-ES message
types:

```ts
import { create } from "@bufbuild/protobuf";
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  ForecastResponseSchema,
  HealthResponseSchema,
} from "./gen/weather/v1/weather_pb.js";
import {
  type WeatherAPIToolHandler,
  registerWeatherAPITools,
} from "./gen/weather/v1/weather_mcp.js";

const handler: WeatherAPIToolHandler = {
  async forecast(_ctx, req) {
    return create(ForecastResponseSchema, {
      reportId: "forecast-1",
      details: req.details,
    });
  },
  async health() {
    return create(HealthResponseSchema);
  },
};

const server = new Server(
  { name: "weather-mcp", version: "0.1.0" },
  { capabilities: { tools: {} } },
);
registerWeatherAPITools(server, handler, "weather");
await server.connect(new StdioServerTransport());
```

JavaScript projects import the compiled `weather_mcp.js` and `weather_pb.js`
files from the TypeScript build output instead of generating a separate
JavaScript sidecar.

## Status

This repository implements three MCP server primitives:

- **Tools** — full support across 5 languages (Go, Python, Kotlin, Java, TypeScript)
- **Prompts** — full support across 5 languages
- **Resources** — full Go support; Python, Kotlin, Java, TypeScript generate
  typed handler interfaces with stub registration (full implementation planned)
- unary RPC only
- proto3 only
- supported protobuf features: scalar, enum, nested message, repeated,
  `oneof`, `optional`, maps, recursive message schemas via `$defs`/`$ref`, and
  these well-known types:
  `google.protobuf.Any`, `Empty`, `Timestamp`, `Duration`, `FieldMask`,
  `Struct`, `Value`, `ListValue`, `BoolValue`, `StringValue`, `BytesValue`,
  `Int32Value`, `UInt32Value`, `Int64Value`, `UInt64Value`, `FloatValue`,
  and `DoubleValue`
- generated MCP schema requiredness is proto3-driven: a singular field WITHOUT
  the `optional` keyword is required. A singular field WITH the `optional` keyword
  is optional. `repeated`, `map`, and `oneof` fields are always optional.
- fields that are not required by that generated MCP schema accept explicit
  JSON `null` to match ProtoJSON parser behavior for unset values
- unsupported and required to fail fast:
  non-unary protobuf RPC methods and unsupported `google.protobuf` message
  types
