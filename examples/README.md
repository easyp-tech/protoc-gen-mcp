# protoc-gen-mcp Examples

This directory contains standalone, runnable examples of how to build and
expose Model Context Protocol (MCP) tools using Protocol Buffers with the Go,
Python, Kotlin, Java, TypeScript, and JavaScript SDK paths.

## Prerequisites
- **easyp**: Ensure you have [easyp](https://github.com/easyp-tech/easyp) installed for code generation.
- **JDK 17+ and Gradle 9.2+**: Required for the standalone Java/Kotlin projects
  and the dedicated JVM workspace under [`examples/jvm`](./jvm/README.md).
- **Node.js and npm**: Required for the standalone TypeScript project,
  JavaScript consumption proof, and Node compile gates.

## Running the Examples
1. Generate the example artifacts from the `.proto` definitions.
   Run this from the root of the repository or from the examples directory:
   ```bash
   cd examples
   make generate
   ```
   For `1_helloworld` through `4_crm_system`, the generated Python handler surface lives in `proto/*_mcp.py`, with
   generated `proto/__init__.py` files so the example servers can import
   bindings through `from proto import ...`. The example Python servers
   implement those dataclasses directly; `*_pb2.py` stays internal to the
   generated runtime. The `mcp/options/v1/options.proto` import is resolved
   through the `deps` entry in `examples/easyp.yaml` and pinned by
   `examples/easyp.lock`, so the examples do not depend on the local
   repository-root options package during generation.
   `5_python_standalone`, `10_python_protobuf_standalone`,
   `6_java_standalone`, `7_kotlin_standalone`, and `8_typescript_standalone`
   each have their own `easyp.yaml` and lockfile to model user-owned projects.
2. Run any of the Go servers (e.g. `1_helloworld`):
   ```bash
   cd examples/1_helloworld
   go run main.go
   ```
3. Run the matching Python server:
   ```bash
   cd examples/1_helloworld
   python main.py
   ```
4. Run the Python-only standalone project:
   ```bash
   cd examples/5_python_standalone
   make setup
   make run
   ```
5. Run the raw protobuf Python standalone project:
   ```bash
   cd examples/10_python_protobuf_standalone
   make setup
   make run
   ```
6. Build or run the standalone JVM projects:
   ```bash
   cd examples/6_java_standalone
   make build

   cd ../7_kotlin_standalone
   make build
   ```
7. Run the Java/Kotlin workspace:
   Follow [`examples/jvm/README.md`](./jvm/README.md) for the tested
   `compileJava` / `compileKotlin`, `installDist`, and installed-script stdio
   flow.
8. Build or run the standalone TypeScript project:
   ```bash
   cd examples/8_typescript_standalone
   make build
   make run
   ```
9. Build or run the standalone JavaScript consumption proof:
   ```bash
   cd examples/9_javascript_standalone
   make build
   make run
   ```

When you inspect tools in clients like `@modelcontextprotocol/inspector`,
remember that omitted tool-annotation hints are often rendered with
client-side defaults. If you need deterministic badges in UI, set hints such
as `destructive_hint: false` explicitly in the proto instead of relying on
omission.

## Included Examples

### [1_helloworld](./1_helloworld)
A minimal Quickstart example. It contains a single RPC method that takes a name and returns a greeting. Demonstrates the simplest way to configure `easyp.yaml` and initialize `mcp.Server` with the generated bindings.

### [2_weather_api](./2_weather_api) (Validation & Safe Read-Only Queries)
A mock weather service demonstrating input safety limits and schema hints.
- **Validation**: Strict limits on parameters (e.g. `minimum`, `maximum` degrees).
- **Oneof**: Allows lookup either by `city` or by `coordinates`, using explicit
  generated Python wrapper variants in the handler API.
- **Annotations**: Sets `read_only_hint: true` and `open_world_hint: true` so the LLM knows it is harmless to aggressively scan locations without altering state.

### [3_file_manager](./3_file_manager) (Destructive Operations)
A straightforward demo for operating on a temporary directory.
- **Security Check**: Strings validated by `pattern: "^[a-zA-Z0-9_\\-\\.]+$"` to guarantee LLMs don't trigger path injections.
- **Destructive Operations**: Explicitly flags the `DeleteFile` tool with `destructive_hint: true` so MCP hosts demand user confirmation before dropping files.

### [4_crm_system](./4_crm_system) (Enterprise Kitchen Sink)
A full-featured (mocked) customer relationship management service covering complex configurations.
- **Icons**: Distinct tool and service icons encoded as JSON.
- **Well-Known Types (WKTs)**: `google.protobuf.Timestamp` and `google.protobuf.FieldMask`
  surface in Python handlers as their ProtoJSON forms (`RFC 3339` strings and
  field-mask strings), while the generated runtime keeps protobuf conversion
  internal.
- **Advanced Constraints**: Ensures slice entries hold unique, non-empty values (`min_items: 1`, `unique_items: true`).
- **Rich Output**: Complex responses carrying arrays of message objects back to the LLM context.

### [5_python_standalone](./5_python_standalone) (Python-Only User Project)
A pure Python MCP server example with its own `pyproject.toml`, `easyp.yaml`,
protobuf contract, generated bindings, and stdio server.
- **Independent environment**: `make setup` creates a local `.venv` and installs the official Python MCP SDK plus protobuf/jsonschema dependencies.
- **No import hacks**: `server.py` imports generated code with `from proto import notebook_mcp` and does not edit `sys.path`.
- **Python-only proto**: the user-authored proto does not need `go_package`; `protoc-gen-mcp` synthesizes internal metadata for Python generation.
- **Default dataclass mode**: handlers implement generated dataclasses from
  `notebook_mcp.py`; use the raw protobuf example when existing code should
  keep `*_pb2` request/response classes.

### [10_python_protobuf_standalone](./10_python_protobuf_standalone) (Python Raw Protobuf Handler User Project)
A pure Python MCP server example that opts into `python_handler: dataclass+protobuf`.
- **Raw `*_pb2` handlers**: the server imports `from proto import tasks_mcp_pb, tasks_pb2` and implements `TaskAPIToolHandler` with `tasks_pb2.*` request/response classes.
- **Same generated registration**: `server.py` still registers tools through
  `tasks_mcp_pb.register_task_api_tools(server, TaskStore())`.
- **Dual-generation option**: dataclass output stays in `*_mcp.py` and raw
  protobuf output is generated in `*_mcp_pb.py`.
- **Generator-owned contract**: schemas, ProtoJSON parsing, validation,
  annotations, structured output, and stdio behavior remain in the generated
  sidecars.

### [6_java_standalone](./6_java_standalone) (Java User Project)
A standalone Java MCP server with its own Gradle build, `easyp.yaml`, protobuf
contract, and handwritten stdio server.
- **Java-native proto**: the user-authored proto uses `java_package` and does
  not need a Go `go_package` option.
- **Local options generation**: `with_imports: true` generates the local
  `mcp.options.v1` Java class from the Easyp dependency, while the build
  removes generated Google protobuf sources and relies on the protobuf jar.
- **Generated API**: the server implements `TodoMcp.TodoAPIToolHandler` and
  registers tools through `TodoMcp.registerTodoAPITools(...)`.

### [7_kotlin_standalone](./7_kotlin_standalone) (Kotlin User Project)
A standalone Kotlin MCP server with its own Gradle build, `easyp.yaml`,
protobuf contract, and handwritten stdio server.
- **Dual protobuf generation**: `easyp.yaml` generates Java protobuf classes,
  Kotlin protobuf helpers, and the `lang=kotlin` MCP sidecar.
- **Kotlin-native handler**: the server implements `TodoAPIToolHandler` and
  registers tools through `registerTodoAPITools(server, impl, namespace = ...)`.
- **No Go proto metadata**: JVM request preparation synthesizes internal
  protogen metadata, so the user-authored proto does not need `go_package`.

### [8_typescript_standalone](./8_typescript_standalone) (TypeScript User Project)
A standalone TypeScript MCP server with its own `package.json`, `tsconfig.json`,
`easyp.yaml`, protobuf contract, generated TypeScript sources, and handwritten
stdio server.
- **Protobuf-ES generation**: `easyp.yaml` runs `@bufbuild/protoc-gen-es` with
  `target=ts` and `import_extension=js`.
- **Generated MCP sidecar**: `protoc-gen-mcp` runs with `lang=typescript` and
  emits `src/generated/proto/notebook_mcp.ts`.
- **Official SDK runtime**: the server uses `Server`, `StdioServerTransport`,
  and generated `registerNotebookAPITools(...)`.

### [9_javascript_standalone](./9_javascript_standalone) (JavaScript Consumption Proof)
A plain JavaScript MCP server that consumes compiled `.js` and `.d.ts` output
from the TypeScript standalone project.
- **No direct JavaScript renderer**: this project does not use
  `lang=javascript`; it proves JavaScript consumers can use compiled
  TypeScript target output.
- **Editor/type metadata**: `src/server.js` uses `// @ts-check` and JSDoc
  imports against generated declarations from `8_typescript_standalone/dist`.
- **Runtime proof**: the server starts over stdio and registers generated
  notebook tools from compiled output.

### [jvm](./jvm/README.md) (Java & Kotlin Official SDK Workspace)
An isolated Gradle workspace that generates and runs Java and Kotlin MCP
servers against the official JVM SDKs.
- **Compile gate**: `gradle --no-daemon -p examples/jvm :java-server:compileJava :kotlin-server:compileKotlin`
- **Installable scripts**: `gradle --no-daemon -p examples/jvm :java-server:installDist :kotlin-server:installDist`
- **Verified runtime path**: installed scripts under `build/install/.../bin/...`
- **Kotlin dual generation**: Java protobuf output, Kotlin protobuf output,
  and the `lang=kotlin` MCP sidecar all participate in the tested build graph

See [`examples/jvm/README.md`](./jvm/README.md) for the full runnable
walkthrough.
