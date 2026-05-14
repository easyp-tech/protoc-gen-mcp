# AGENTS

## Scope

This repository implements a protobuf-first MCP generator and runtime for Go,
Python, Kotlin, Java, and TypeScript MCP server bindings. The MVP is
intentionally narrow and must stay decision-consistent with the current
architecture unless explicitly revised.

## Stack

- Go 1.24+
- Python 3.10+ for generated-runtime verification and example servers
- Python runtime tests bootstrap an isolated virtualenv through
  `internal/pythontest`; they must not depend on globally installed Python
  packages such as `protobuf`
- Gradle 9.2+ and JDK 17+ for JVM compile-gate verification
- `easyp v0.15.2-rc1` for repository linting and code generation workflows
- `google.golang.org/protobuf` for code generation, reflection, and ProtoJSON
- `google.protobuf` for Python generated modules and ProtoJSON conversion
- `github.com/modelcontextprotocol/go-sdk/mcp` as the MCP runtime
- `mcp>=1.27,<2` as the official Python MCP SDK target
- `io.modelcontextprotocol.sdk:mcp` as the official Java MCP SDK target
- `io.modelcontextprotocol:kotlin-sdk-server` as the official Kotlin MCP SDK
  target
- Node.js with TypeScript `6.0.3` for generated TypeScript compile-gate
  verification
- `@modelcontextprotocol/sdk@1.29.0` as the official TypeScript MCP SDK target
- `@bufbuild/protobuf@2.12.0` as the Protobuf-ES runtime and generated `_pb.ts`
  target for TypeScript bindings
- `ajv@8.20.0` for the Node raw JSON Schema validation path
- `github.com/google/jsonschema-go/jsonschema` for JSON Schema parsing and
  validation
- `github.com/bufbuild/protocompile` for in-process descriptor compilation in
  generator tests, so `go test` does not require an external `protoc` binary

## Layout

- `cmd/protoc-gen-mcp`: protoc plugin entrypoint for `--mcp_out`
- `cmd/example-mcp-server`: runnable stdio MCP server for manual agent/client checks
- `cmd/example-python-mcp-server`: runnable stdio MCP server for Python SDK parity checks
- `mcpruntime`: public runtime helpers used by generated code
- `.github/workflows`: GitHub Actions CI and release workflows
- `.goreleaser.yaml`: release packaging for the plugin binary
- `examples`: standalone Go/Python/JVM integration projects; example
  directories use numeric underscore prefixes such as `1_helloworld`,
  `4_crm_system`, `5_python_standalone`, `6_java_standalone`, and
  `7_kotlin_standalone`, `8_typescript_standalone`,
  `9_javascript_standalone`, and `10_python_protobuf_standalone`, plus the
  dedicated JVM workspace `examples/jvm` and Node spike workspace
  `examples/node/sdk-spike`
- `examples/node/sdk-spike`: pinned local Node package scope for TypeScript
  SDK, Protobuf-ES, Ajv, package-lock-backed `npm ci`, and strict NodeNext
  compile checks
- `examples/easyp.lock`: pinned Easyp dependency lock for standalone examples
- `examples/mcp`: generated Python `mcp.options.*` protobuf modules for
  standalone examples; generated from the GitHub dependency declared in
  `examples/easyp.yaml`, not from the local repository root
- `examples/5_python_standalone`: Python-only user-style example with its own
  `pyproject.toml`, `easyp.yaml`, generated `proto`/`mcp` packages, and stdio
  server
- `examples/10_python_protobuf_standalone`: Python-only user-style example
  with its own `pyproject.toml`, `easyp.yaml`, generated `proto`/`mcp`
  packages, opt-in `python_handler=protobuf` sidecar, raw `*_pb2` handlers,
  and stdio server
- `examples/6_java_standalone`: Java user-style standalone project with its
  own Gradle build, `easyp.yaml`, protobuf contract, generated Java protobuf
  classes, generated `lang=java` MCP sidecar, and handwritten stdio server
- `examples/7_kotlin_standalone`: Kotlin user-style standalone project with
  its own Gradle build, `easyp.yaml`, protobuf contract, generated Java/Kotlin
  protobuf classes, generated `lang=kotlin` MCP sidecar, and handwritten stdio
  server
- `examples/8_typescript_standalone`: TypeScript user-style standalone project
  with its own npm package, `tsconfig.json`, `easyp.yaml`, protobuf contract,
  generated Protobuf-ES `_pb.ts` classes, generated `lang=typescript` MCP
  sidecar, and handwritten stdio server
- `examples/9_javascript_standalone`: JavaScript consumption proof that imports
  compiled `.js` output and `.d.ts` metadata from the TypeScript standalone
  project without using direct `lang=javascript`
- `examples/jvm`: isolated Gradle Kotlin DSL workspace that compiles generated
  Java/Kotlin JVM sidecars against Maven `protoc`, official MCP SDK artifacts,
  and the local `cmd/protoc-gen-mcp` binary
- `examples/jvm/README.md`: user-facing JVM walkthrough covering the tested
  compile, install, run, and stdio verification path
- `examples/jvm/settings.gradle.kts`: JVM example workspace and repository policy
- `examples/jvm/build.gradle.kts`: root JVM helper tasks, including local
  `protoc-gen-mcp` compilation
- `examples/jvm/java-server`: installable Java stdio example app driven by
  generated low-level tool registration
- `examples/jvm/java-server/build.gradle.kts`: Java compile gate and installed
  application script using `lang=java`
- `examples/jvm/kotlin-server`: installable Kotlin stdio example app driven by
  generated low-level tool registration
- `examples/jvm/kotlin-server/build.gradle.kts`: Kotlin compile gate and
  installed application script using `lang=kotlin`
- `easyp.yaml`: main repository config for shipped protobuf APIs
- `easyp.test.yaml`: development and test config for fixture generation
- `mcp/options/v1/options.proto`: custom protobuf options for MCP metadata
- `internal/codegen`: code generation logic
- `internal/codegen/jvm_*.go`: shared JVM semantic model, naming, and collector
  foundation used by the Kotlin and Java renderers
- `internal/codegen/render_java.go`: self-contained Java sidecar renderer
- `internal/codegen/java_contract_test.go`: Java public API, low-level SDK seam,
  schema/ProtoJSON path, metadata projection, and fail-fast contract tests
- `internal/codegen/render_kotlin.go`: self-contained Kotlin sidecar renderer
- `internal/codegen/kotlin_contract_test.go`: Kotlin public API, SDK wiring,
  schema-path, and JVM import contract tests
- `internal/codegen/render_typescript.go`: TypeScript sidecar renderer for
  low-level official SDK imports, typed public handlers, namespace-aware
  registration, schema constants, and private registry metadata
- `internal/codegen/typescript_*_test.go`: TypeScript semantic model, naming,
  renderer contract, golden, runtime, stdio, and strict NodeNext compile-gate
  tests
- `internal/devtools/protobufesnormalize`: repository-local normalizer for
  Protobuf-ES `fileDesc(...)` descriptor strings generated in standalone
  TypeScript examples so checked-in output remains deterministic
- `internal/examplemcp`: reusable example MCP server wiring and stdio smoke test
- `internal/pythontest`: hermetic Python test runtime bootstrap used by Go
  tests that execute generated Python code
- `internal/schema`: protobuf descriptor to JSON Schema conversion
- `internal/testproto`: protobuf fixtures and generated code used in repository tests
- `internal/testproto/example/v1/__init__.py`: generated Python package marker
  for the shared test fixture package
- `mcp/__init__.py`: generated Python package bridge that lets `mcp.options.*`
  protobuf modules coexist with the official Python MCP SDK package
- `mcp/options/v1/options_mcp.py`: generated Python dataclass/runtime bindings
  for the shipped MCP options protobuf API
- `mcp/options/v1/__init__.py`: generated Python package marker for the shipped
  options package
- Python generation emits package `__init__.py` files next to generated
  `*_mcp.py` modules so generated directories can be imported as Python
  packages
- `testdata/golden`: golden snapshots for generated Go, Python, Kotlin, Java,
  and TypeScript binding files
- `testdata/unsupported`: negative fixtures for fail-fast generator coverage

## MVP Rules

- Target only `proto3`
- Support only MCP tools with unary request/response
- Supported protobuf features: scalar, enum, nested message, repeated,
  `oneof`, `optional`, maps, recursive message schemas via `$defs`/`$ref`, and
  these well-known types:
  `google.protobuf.Any`, `Empty`, `Timestamp`, `Duration`, `FieldMask`,
  `Struct`, `Value`, `ListValue`, `BoolValue`, `StringValue`, `BytesValue`,
  `Int32Value`, `UInt32Value`, `Int64Value`, `UInt64Value`, `FloatValue`,
  and `DoubleValue`
- Unsupported and required to fail fast at generation time:
  non-unary protobuf RPC methods and unsupported `google.protobuf` message
  types
- JSON contract is ProtoJSON-first
- Tool input requiredness is a generated MCP schema policy, not the protobuf
  `required` label: singular fields are marked as required in the tool schema
  by default unless they are `proto3 optional`, `repeated`, `map`, `oneof`,
  or explicitly marked optional through MCP field options
- Generated schemas must accept explicit JSON `null` for fields that are not
  marked as required by that generated MCP tool schema so MCP clients that
  validate cached `inputSchema` remain compatible with ProtoJSON unset
  semantics
- Generated schemas should carry agent-facing examples for complex ProtoJSON
  shapes. Explicit JSON examples from comments/options are materialized as JSON
  literals when possible, and fallback examples are synthesized for advanced
  forms such as maps, `Any`, recursive messages, and ProtoJSON special scalar
  encodings
- `mcp.options.v1` must declare its `go_package` directly in
  `mcp/options/v1/options.proto` so downstream Easyp consumers do not need
  a special `go_package_prefix` override
- Direct lang=javascript generation is deferred in v1.1; JavaScript support
  is through compiled TypeScript target `.js` output plus generated `.d.ts`
  declarations.

## Public API

- Generated code exposes `<Service>ToolHandler`
- Generated code exposes `Register<Service>Tools(server, impl, opts...) error`
- Generated Python modules expose `<Service>ToolHandler`
- Generated Python modules expose `register_<service_name>_tools(server, impl, *, namespace=None)`
- Generated Python modules default to `python_handler=dataclass`, exposing
  dataclasses, `UNSET`, explicit `oneof` wrapper variants, mapper helpers, and
  handler protocols using generated public dataclass types from `*_mcp.py`
- Generated Python modules also support opt-in `python_handler=protobuf`,
  where handler protocols accept and return raw generated `*_pb2` message
  classes, generated dataclass public types/mapper helpers are omitted, and
  registration remains `register_<service_name>_tools(server, impl, *, namespace=None)`
- Generated Kotlin files expose `<Service>ToolHandler`
- Generated Kotlin files expose
  `register<Service>Tools(server: Server, impl: <Service>ToolHandler, namespace: String? = null)`
- Generated Java files expose one top-level `public final <ProtoFile>Mcp`
  sidecar class per proto file
- Generated Java files expose nested `<Service>ToolHandler` interfaces inside
  the sidecar class
- Generated Java files expose
  `register<Service>Tools(McpServerTransportProvider transportProvider, <Service>ToolHandler impl, String namespace)`
- Generated TypeScript files expose `<Service>ToolHandler`
- Generated TypeScript files expose
  `register<Service>Tools(server, impl, namespace?)`
- Runtime exposes only the minimal registration options used by generated code
- Generated MCP tool names must not contain dots; namespace prefixes and method
  names are joined with underscores, and any dots in configured segments are
  normalized to underscores
- Generated tool annotations are forwarded exactly as declared in proto
  options; omitted hints stay omitted, so external clients may apply their own
  display defaults for missing values

## Current Status

- Implemented:
- `cmd/protoc-gen-mcp` plugin scaffold and generated `*.mcp.go` bindings
  - typed plugin option parsing for `lang=go|python|kotlin|java|typescript` and
    `python_runtime=google.protobuf|betterproto|grpclib`, plus Python-only
    `python_handler=dataclass|protobuf`
  - generated TypeScript `*_mcp.ts` sidecars for `lang=typescript`, targeting
    the official `@modelcontextprotocol/sdk` low-level `Server` import path and
    Protobuf-ES `_pb.js` modules, including typed `<Service>ToolHandler`
    interfaces, namespace-aware `register<Service>Tools(server, impl,
    namespace?)`, raw schema JSON constants, imported message schema/file
    registry refs, metadata registry records, NodeNext `.js` specifiers, and
    strict `verbatimModuleSyntax` import type/value separation
  - generated Node stdio process verification for TypeScript sidecars, covering
    `tools/list`, valid tool calls, text/structured output parity, invalid
    input rejection, invalid output rejection, and advanced ProtoJSON shapes
  - shared JVM foundation for `lang=kotlin` and `lang=java`: parser and
    generator dispatch accept both targets, collect SDK-neutral
    `internal/codegen/jvm_*.go` models, preserve existing `FileModel` schema
    JSON/annotations/icons/type semantics, and feed both the Kotlin and Java
    renderers
  - generated self-contained Java `*.java` sidecars for `lang=java`, targeting
    the official Java MCP SDK through the low-level
    `McpServerTransportProvider.setSessionFactory(...)` seam, including one
    filename-matching `public final` sidecar class per proto file, nested
    protobuf-typed handler interfaces, namespace-aware
    `register<Service>Tools(...)`, generated `tools/list`/`tools/call`
    request-handler wiring, raw schema JSON constants, official ProtoJSON
    parse/marshal helpers, raw protocol-map metadata projection for
    annotations/icons/execution, and generator-owned input/output validation
  - generated self-contained Kotlin `*_mcp.kt` bindings for `lang=kotlin`,
    targeting `io.modelcontextprotocol:kotlin-sdk-server`, including
    protobuf-typed handler interfaces, namespace-aware
    `register<Service>Tools(...)`, generated low-level `tools/list` and
    `tools/call` helper wiring, raw schema JSON constants, ProtoJSON
    parse/marshal helpers, annotations, icons, and task-support mapping
  - single-source custom generator option handling through
    `protogen.Options.ParamFunc`, with fail-fast rejection of unknown
    `protoc-gen-mcp` params
  - non-Go request preparation synthesizes internal `go_package` metadata for
    `.proto` files that omit it, so Python/JVM/TypeScript users do not need
    Go-specific proto options just to run `lang=python`, `lang=java`,
    `lang=kotlin`, or `lang=typescript`
  - generated self-contained Python `*_mcp.py` bindings for
    `lang=python,python_runtime=google.protobuf`, including default
    dataclass-mode handler protocols, dataclasses, `UNSET`, explicit `oneof`
    wrapper variants, schema JSON constants, shared per-server registry wiring,
    namespace-aware `register_<service_name>_tools(...)`, generated
    protobuf<->dataclass mappers, and ProtoJSON dict/message conversion via
    `json_format.ParseDict`
  - opt-in generated Python `python_handler=protobuf` mode, where handler
    protocols use raw generated `*_pb2` request/response message classes,
    registration uses identity converters through the existing
    `_RegisteredTool.from_pb/to_pb` seam, and message-only dependency files do
    not emit empty public `*_mcp.py` sidecars
  - generated Python `mcp/__init__.py` bridge support file so `mcp.options.*`
    protobuf output coexists with the official `mcp` SDK package namespace
  - generated Python package `__init__.py` files next to `*_mcp.py` output so
    examples can import generated bindings through package imports such as
    `from proto import helloworld_mcp`
  - checked-in generated Python package markers and Python bindings for shipped
    and test protobuf APIs, including `mcp/options/v1/options_mcp.py` and
    `internal/testproto/example/v1/__init__.py`
  - Python generator-level parity coverage against the shared semantic IR,
    including Python golden output, public API-shape assertions, and
    fail-fast negative tests for unsupported runtimes, unsupported protobuf
    descriptors, and streaming RPCs
  - runnable Python stdio example server in `cmd/example-python-mcp-server`
    backed by generated dataclass-based `example_mcp.py` bindings and the
    official MCP Python SDK
  - standalone Python examples in `examples/` implemented against generated
    dataclasses from `*_mcp.py`, with checked-in Python example artifacts
    regenerated through `examples/Makefile`
  - `examples/5_python_standalone` models an external Python-only project with
    its own `pyproject.toml`, `easyp.yaml`, generated package `__init__.py`
    files, generated local `mcp.options.*` modules, and `server.py` without
    `sys.path` bootstrap code
  - `examples/10_python_protobuf_standalone` models an external Python-only
    raw `*_pb2` handler project with its own `pyproject.toml`, `easyp.yaml`,
    generated package `__init__.py` files, generated local `mcp.options.*`
    modules, opt-in `python_handler=protobuf` sidecar, and checked-in stdio
    server
  - standalone examples use `examples/easyp.yaml` `deps` plus
    `examples/easyp.lock` to resolve `mcp/options/v1/options.proto` from
    `github.com/easyp-tech/protoc-gen-mcp` instead of depending on the local
    repository root
  - end-to-end stdio integration coverage that runs the shared example MCP
    contract checks against both the Go server and the Python SDK server,
    including `CallToolResult` text/structured parity and output-validation
    failure coverage for Python
  - custom MCP protobuf options in `mcp/options/v1/options.proto`
  - generated tool metadata includes `ToolAnnotations` (`read_only_hint`, `destructive_hint`, `idempotent_hint`, `open_world_hint`) and `Icon` mappings directly to the Go SDK
  - `read_only_hint` now propagates correctly into both generated Go and
    Python tool annotations
  - Python generation augments each file's public type module with only the
    current-file types actually referenced by other generated files, so
    cross-file imports work without forcing unrelated hidden-only types into
    the public API surface
  - dedicated `examples/` directory featuring 10 standalone integration
    projects spanning quickstarts to complex CRM mocks and pure Python, Java,
    Kotlin, TypeScript, and JavaScript user-style projects
  - support for `oneof` explicit requiredness through `mcp.options.v1.oneof` options
  - strict schema generation correctly differentiating zero-values (`0`, `0.0`, `""`) using pointer constraints
  - runtime registration and JSON Schema validation in `mcpruntime`
  - descriptor-to-JSON-Schema generation in `internal/schema`
  - support for maps, recursive message schemas, top-level/nested `oneof`, and
    selected well-known protobuf types in generated schemas
  - tool-first requiredness in generated MCP schemas: singular non-`optional`
    fields are required by default unless they are `repeated`, `map`, `oneof`,
    or relaxed through MCP field options
  - ProtoJSON-compatible nullable support for fields that are not required by
    that generated MCP tool schema, including client-side acceptance of
    explicit JSON `null`
  - auto-generated JSON Schema examples for complex fields, including parsed
    explicit JSON examples for `Any` payloads plus synthesized fallback
    examples for maps, recursive messages, repeated scalars, and special
    ProtoJSON scalar encodings
  - generated schema JSON constants are emitted as interpreted Go string
    literals so proto comments and examples containing backticks do not break
    generated `*.mcp.go` files
  - Easyp main/test configs without any special managed-mode override for
    `mcp.options.v1`, because the options package declares `go_package`
    directly in source
  - GitHub Actions CI in `.github/workflows/tests.yml`
  - GoReleaser-based tagged releases in `.github/workflows/release.yml` and
    `.goreleaser.yaml`
  - runnable `cmd/example-mcp-server` and reusable wiring in
    `internal/examplemcp`
  - underscore-only MCP tool naming; expected example tool names are
    `example_CreateReport`, `example_DescribeAdvancedShapes`,
    `example_DescribeScalarShapes`, and `example_Health`
  - representative Java output for `lang=java` is locked by
    `testdata/golden/example_mcp.java.golden`, and generator coverage includes a
    narrow `javac` compile smoke proving that example handler code compiles
    against the generated Java API without depending on repository-internal
    runtime types; full runnable Java example/runtime proof remains deferred to
    Phase 04
- `examples/jvm` provides a real Gradle compile gate that builds the local
  `protoc-gen-mcp` binary, runs Maven `protoc 4.34.1`, and compiles generated
  Java/Kotlin sidecars against `io.modelcontextprotocol.sdk:mcp:1.1.1`,
  `io.modelcontextprotocol:kotlin-sdk-server:0.11.1`, and
  `com.networknt:json-schema-validator:1.5.9`
- installable Java and Kotlin stdio example servers under `examples/jvm`,
  built through Gradle `installDist`, that register generated tools through
  `ExampleMcp.registerExampleAPITools(...)` and
  `registerExampleAPITools(server, impl, namespace = "example")` instead of
  SDK `addTool` APIs
- standalone Java and Kotlin user-style projects under
  `examples/6_java_standalone` and `examples/7_kotlin_standalone`, each with
  its own `easyp.yaml`, lockfile, Gradle build, handwritten stdio server, and
  generated-source cleanup for Google protobuf classes supplied by Maven
- standalone TypeScript and JavaScript Node projects under
  `examples/8_typescript_standalone` and `examples/9_javascript_standalone`,
  covering user-style `lang=typescript` generation with Protobuf-ES plus plain
  JavaScript consumption of compiled generated `.js` and `.d.ts` output
- repository docs now route JVM users through `examples/jvm/README.md` and
  Node users through `examples/8_typescript_standalone` plus
  `examples/9_javascript_standalone`; rollout messaging states that releases
  publish the `protoc-gen-mcp binary` while downstream JVM and Node users
  compile generated sources against official SDK artifacts and npm dependencies
  - `example_DescribeAdvancedShapes` covers the full current advanced contract
    matrix in the test server: maps, timestamps, durations, field masks,
    `Struct`/`Value`/`ListValue`, `Any`, scalar wrappers, raw float ProtoJSON,
    recursive nodes, and top-level `oneof`
  - `example_DescribeScalarShapes` covers plain protobuf scalar kinds in the
    test server, including `int32`/`sint32`/`sfixed32`, `uint32`/`fixed32`,
    `int64`/`sint64`/`sfixed64`, `uint64`/`fixed64`, `float`, `double`,
    `bool`, `string`, `bytes`, enum, nested message, repeated scalar, and
    proto3 `optional` scalar/enum fields
- Verified:
  - `easyp` lint and generation flows for `mcp` and `internal/testproto`
  - `go test ./internal/codegen -count=1` for generator, Go/Python/Kotlin/Java,
    and shared JVM foundation coverage
  - `go test ./internal/codegen -run 'TestParseOptions|TestPythonRenderer_EmitsDataclassPublicAPI|TestPythonRenderer_EmitsProtobufHandlerPublicAPI|TestPythonRenderer_ProtobufHandlerImportsCrossFileProtobufModules|TestGenerate_PythonProtobufHandlerSkipsCrossFilePublicTypeModules' -count=1`
    for Python handler option parsing and generated dataclass/protobuf handler
    API contracts
  - `go test ./internal/codegen -run 'TestGenerateKotlinExampleGolden|TestKotlinContract_.*' -count=1`
    for Kotlin golden output and focused Kotlin renderer contracts
- `go test ./internal/codegen -run 'TestJavaContract_.*|TestGenerateJavaExampleGolden|TestGenerateJavaExampleHandlerCompileSmoke|TestGenerate_JavaTargetEmitsOutput' -count=1`
  for Java golden output, low-level renderer contracts, and the Phase 03
  narrow `javac` API compile smoke
- `go test ./internal/codegen -run 'TestTypeScriptModel|TestTypeScriptNames|TestTypeScriptContract|TestGenerateTypeScript|TestGenerate_TypeScript|TestTypeScriptGeneratedPublicAPICompilesUnderNodeNext' -count=1`
  for TypeScript semantic model, naming, renderer contract, golden, generator,
  and strict NodeNext public API compile coverage
- `cd examples/node/sdk-spike && npm ci && npm run typecheck && npm run build`
  for the pinned local TypeScript SDK/Protobuf-ES compile/build gate
- `go test ./internal/codegen -run 'TestTypeScriptGeneratedNodeServer.*OverStdio|TestTypeScriptGeneratedNodeServerRejectsInvalid(Input|Output)OverStdio' -count=1`
  for generated Node stdio process verification
- `gradle --no-daemon -p examples/jvm :java-server:compileJava :kotlin-server:compileKotlin`
  for the Phase 04 JVM compile gate
- `gradle --no-daemon -p examples/jvm :java-server:installDist :kotlin-server:installDist`
  for installable JVM example scripts
- `cd examples/6_java_standalone && make lint && make clean build`
  for the standalone Java user-project generation and compile flow
- `cd examples/7_kotlin_standalone && make lint && make clean build`
  for the standalone Kotlin user-project generation and compile flow
- `cd examples/8_typescript_standalone && make lint && make clean build`
  for the standalone TypeScript user-project generation and compile flow
- `cd examples/9_javascript_standalone && make clean build`
  for JavaScript consumption of compiled TypeScript target output
- `cd examples/10_python_protobuf_standalone && make lint && make clean generate`
  for the raw protobuf Python standalone user-project generation flow
- `go test ./examples -run 'TestStandalone(TypeScript|JavaScript)ExampleOverStdio' -count=1`
  for standalone Node stdio parity
- `go test ./...`
- stdio smoke tests via `internal/examplemcp/stdio_test.go`
- `go test ./internal/examplemcp -run 'TestJava.*OverStdio' -count=1`
  for Java installed-script stdio parity, invalid input, and invalid output
- `go test ./internal/examplemcp -run 'TestKotlin.*OverStdio' -count=1`
  for Kotlin installed-script stdio parity, invalid input, and invalid output
- Python stdio integration coverage for the shared server:
  `go test ./internal/examplemcp -run 'TestPythonServerOverStdio|TestPythonServerRejectsInvalidOutputOverStdio' -count=1`
  - Python stdio integration coverage for standalone examples:
    `go test ./examples -run TestPythonExamplesOverStdio -count=1`
  - Python stdio integration coverage for the Python-only standalone example:
    `go test ./examples -run TestStandalonePythonExampleOverStdio -count=1`
  - Python stdio integration coverage for the raw protobuf standalone example:
    `go test ./examples -run TestStandalonePythonProtobufExampleOverStdio -count=1`
  - `internal/codegen` tests build descriptor requests in-process through
    `protocompile`, so `go test ./...` no longer depends on `protoc` being in
    `PATH`
  - client-side schema acceptance checks against advertised `tools/list`
    `inputSchema` for canonical valid payloads, including recursive objects and
    explicit `null` on fields that are not schema-required
  - end-to-end official MCP Python SDK execution over stdio using generated
    Python bindings, including runtime output-validation failure handling
  - manual Cursor validation of `example_Health` and `example_CreateReport`
  - manual Inspector CLI verification of standalone `tools/list` annotations,
    including `notebook_SearchNotes` `readOnlyHint=true` and
    `openWorldHint=false`
- Open:
  - automate broader client compatibility checks beyond Go SDK + Cursor manual
    verification
  - manually verify recursive payloads and explicit `null` payloads in external
    agent runtimes beyond repository tests
  - confirm direct external MCP tool invocation in Codex agent runtimes when
    tool wrappers are surfaced by the host session

## Working Rules

- Keep `AGENTS.md` in sync whenever any of these change:
  - technology choices
  - repository layout
  - supported/unsupported protobuf features
  - generated public API
  - build/test commands
- Do not introduce a manual end-user tool-binding API in MVP
- Prefer compile-time generator errors over runtime ambiguity
- When generator output for `internal/testproto` changes, regenerate it through
  `easyp.test.yaml` and refresh the matching golden snapshot
- Repository verification should use `easyp`; avoid ad hoc direct generation
  flows for development checks
- Keep `mcp/options/v1/options.proto` as the source of truth for the
  options package `go_package`; do not reintroduce a special Easyp override
  unless the package layout changes again
- planning docs are local-only in this repository and should not be committed
  unless the user explicitly changes that policy.

## Commands

- Validate configs:
  - `easyp --cfg easyp.yaml validate-config`
  - `easyp --cfg easyp.test.yaml validate-config`
- Lint shipped protobuf API: `easyp --cfg easyp.yaml lint -p mcp -r .`
- Generate shipped protobuf API: `easyp --cfg easyp.yaml generate -p mcp -r .`
- Lint test fixtures: `easyp --cfg easyp.test.yaml lint -p internal/testproto -r .`
- Generate test fixtures: `easyp --cfg easyp.test.yaml generate -p internal/testproto -r .`
- Generate standalone example artifacts:
  - `cd examples && make generate`
- Run TypeScript Node compile gate:
  - `cd examples/node/sdk-spike && npm ci && npm run typecheck && npm run build`
- Run focused TypeScript codegen tests:
  - `go test ./internal/codegen -run 'TestTypeScriptModel|TestTypeScriptNames|TestTypeScriptContract|TestGenerateTypeScript|TestGenerate_TypeScript|TestTypeScriptGeneratedPublicAPICompilesUnderNodeNext' -count=1`
- Run TypeScript public API compile smoke:
  - `cd examples/node/sdk-spike && npm ci`
  - `go test ./internal/codegen -run TestTypeScriptGeneratedPublicAPICompilesUnderNodeNext -count=1`
- Run generated Node stdio tests:
  - `go test ./internal/codegen -run 'TestTypeScriptGeneratedNodeServer.*OverStdio|TestTypeScriptGeneratedNodeServerRejectsInvalid(Input|Output)OverStdio' -count=1`
- Run focused Python handler option and renderer tests:
  - `go test ./internal/codegen -run 'TestParseOptions|TestPythonRenderer_EmitsDataclassPublicAPI|TestPythonRenderer_EmitsProtobufHandlerPublicAPI|TestPythonRenderer_ProtobufHandlerImportsCrossFileProtobufModules|TestGenerate_PythonProtobufHandlerSkipsCrossFilePublicTypeModules' -count=1`
- Run JVM compile gate:
  - `gradle --no-daemon -p examples/jvm :java-server:compileJava :kotlin-server:compileKotlin`
- Install JVM example scripts:
  - `gradle --no-daemon -p examples/jvm :java-server:installDist :kotlin-server:installDist`
- Run Python-only standalone example:
  - `cd examples/5_python_standalone && make setup && make run`
- Run raw protobuf Python standalone example:
  - `cd examples/10_python_protobuf_standalone && make setup && make run`
  - `cd examples/10_python_protobuf_standalone && make lint && make clean generate`
- Build standalone Java example:
  - `cd examples/6_java_standalone && make build`
- Build standalone Kotlin example:
  - `cd examples/7_kotlin_standalone && make build`
- Run standalone TypeScript example:
  - `cd examples/8_typescript_standalone && make build && make run`
- Run standalone JavaScript example:
  - `cd examples/9_javascript_standalone && make build && make run`
- Run standalone Node stdio tests:
  - `go test ./examples -run 'TestStandalone(TypeScript|JavaScript)ExampleOverStdio' -count=1`
- Build plugin: `go build ./cmd/protoc-gen-mcp`
- Validate GoReleaser config: `goreleaser check`
- Build example MCP server binary:
  - `go build -o example-mcp-server ./cmd/example-mcp-server/main.go`
- Run example MCP server: `go -C /abs/path/to/protoc-gen-mcp run ./cmd/example-mcp-server`
- Run Python example MCP server:
  - `python /abs/path/to/protoc-gen-mcp/cmd/example-python-mcp-server/main.py`
- Run built example MCP server binary: `./example-mcp-server`
- Run tests: `go test ./...`
- Run Java stdio example tests:
  - `go test ./internal/examplemcp -run 'TestJava.*OverStdio' -count=1`
- Run Kotlin stdio example tests:
  - `go test ./internal/examplemcp -run 'TestKotlin.*OverStdio' -count=1`
