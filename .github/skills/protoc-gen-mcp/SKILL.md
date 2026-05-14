---
name: protoc-gen-mcp
description: "Develop, test, review, and extend the protoc-gen-mcp protobuf-first MCP generator for Go, Python, Kotlin, Java, TypeScript, and JavaScript-through-TypeScript examples. Use when modifying codegen/renderers, schema generation, ProtoJSON contracts, MCP options, Python handler modes, JVM/Node bindings, standalone examples, easyp workflows, golden snapshots, stdio tests, or project release/verification docs."
---

# protoc-gen-mcp Development

This repository implements a protobuf-first MCP generator and runtime for Go,
Python, Kotlin, Java, and TypeScript MCP server bindings. Keep changes
decision-consistent with `AGENTS.md`; treat that file as the current source of
truth for supported features, layout, commands, and working rules.

## First Steps

1. Run `git status --short --branch` before editing. Do not revert unrelated
   changes.
2. Read `AGENTS.md` when the task touches supported languages, examples,
   commands, public API, or verification status.
3. Keep `.planning/**` and other GSD artifacts local-only unless the user
   explicitly changes that policy.
4. Prefer `easyp` workflows for repository generation and verification. Avoid
   ad hoc direct `protoc` flows unless the target example intentionally uses
   Maven/npm tooling.
5. Keep generator failures compile-time and explicit. Do not add manual
   end-user tool-binding APIs in the MVP.

## Architecture Map

| Path | Role |
|---|---|
| `cmd/protoc-gen-mcp` | `--mcp_out` protoc plugin entrypoint and option dispatch |
| `internal/codegen` | Shared semantic model and language renderers |
| `internal/codegen/render_python.go` | Python dataclass/protobuf/dual handler generation |
| `internal/codegen/jvm_*.go` | SDK-neutral JVM model, naming, and collection |
| `internal/codegen/render_java.go` | Java low-level official SDK sidecar renderer |
| `internal/codegen/render_kotlin.go` | Kotlin official SDK sidecar renderer |
| `internal/codegen/render_typescript.go` | TypeScript official SDK + Protobuf-ES renderer |
| `internal/schema` | Descriptor-to-JSON-Schema conversion and ProtoJSON contract shape |
| `internal/pythontest` | Hermetic Python virtualenv bootstrap for Go tests |
| `internal/examplemcp` | Reusable stdio server checks for Go/Python/JVM examples |
| `examples/5_python_standalone` | Dataclass Python standalone project |
| `examples/10_python_protobuf_standalone` | Dual `dataclass+protobuf` Python standalone project |
| `examples/6_java_standalone` | Java standalone project |
| `examples/7_kotlin_standalone` | Kotlin standalone project |
| `examples/8_typescript_standalone` | TypeScript standalone project |
| `examples/9_javascript_standalone` | JavaScript consuming compiled TypeScript output |
| `examples/jvm` | Gradle compile/install/stdin proof for generated JVM sidecars |
| `examples/node/sdk-spike` | Pinned Node SDK, Protobuf-ES, Ajv compile gate |
| `testdata/golden` | Golden snapshots for generated Go/Python/JVM/TypeScript files |

## Generation Targets

| Target | Plugin Options | Generated Public API |
|---|---|---|
| Go | `lang=go` | `<Service>ToolHandler`, `Register<Service>Tools(server, impl, opts...) error` |
| Python dataclass | `lang=python,python_runtime=google.protobuf` or `python_handler=dataclass` | `*_mcp.py`, dataclasses, `UNSET`, oneof wrappers, mapper helpers, `register_<service>_tools(...)` |
| Python protobuf | `lang=python,python_handler=protobuf` | `*_mcp.py` with raw `*_pb2` handler protocol and identity converters |
| Python dual | `lang=python,python_handler=dataclass+protobuf` | dataclass sidecar in `*_mcp.py` plus raw protobuf sidecar in `*_mcp_pb.py` |
| Kotlin | `lang=kotlin` | `<Service>ToolHandler`, `register<Service>Tools(server, impl, namespace = null)` |
| Java | `lang=java` | top-level `<ProtoFile>Mcp` class, nested `<Service>ToolHandler`, `register<Service>Tools(...)` |
| TypeScript | `lang=typescript` | `*_mcp.ts`, typed `<Service>ToolHandler`, `register<Service>Tools(server, impl, namespace?)` |
| JavaScript | no direct `lang=javascript` in v1.1 | consume compiled TypeScript `.js` plus `.d.ts` output |

## Core Invariants

- Support only `proto3` and unary request/response MCP tools.
- JSON contract is ProtoJSON-first for every target.
- Tool input requiredness is schema policy, not protobuf `required`.
- Singular non-optional fields are schema-required by default.
- `optional`, `repeated`, `map`, `oneof`, and explicitly optional fields are
  not schema-required and must accept explicit JSON `null`.
- Recursive messages use `$defs`/`$ref`; schemas include useful examples for
  complex ProtoJSON shapes.
- Generated tool names must not contain dots. Join namespace and method with
  underscores and normalize dots to underscores.
- Unknown `protoc-gen-mcp` params, streaming RPCs, proto2, and unsupported
  `google.protobuf.*` message types must fail fast.
- Non-Go targets must not require users to add Go-specific proto metadata just
  to generate Python/JVM/TypeScript bindings.

For detailed schema behavior, read
`references/schema-generation.md`. For failure diagnosis, read
`references/troubleshooting.md`.

## Common Workflows

### Modify a Renderer

1. Identify whether the change belongs in shared `FileModel`/schema metadata or
   in a target-specific renderer.
2. Reuse existing schema JSON, annotations, icons, and ProtoJSON semantics.
   Do not recompute schema meaning inside language renderers.
3. Update focused contract tests for the target before or with implementation:
   `*_contract_test.go`, `typescript_*_test.go`, Python renderer tests, or Go
   golden tests.
4. Regenerate affected fixtures through `easyp`, then refresh matching golden
   snapshots in `testdata/golden`.
5. Run the narrow target tests first, then broader tests if the change affects
   shared behavior.

### Modify Python Handler Modes

1. Keep dataclass mode the default.
2. Preserve protobuf-only compatibility: `python_handler=protobuf` writes raw
   handlers to `*_mcp.py`.
3. Preserve dual output: `python_handler=dataclass+protobuf` writes dataclass
   handlers to `*_mcp.py` and raw handlers to `*_mcp_pb.py`.
4. Use `internal/pythontest` for runtime tests so Go tests do not depend on
   globally installed Python packages.
5. Update `examples/5_python_standalone` or
   `examples/10_python_protobuf_standalone` when behavior changes user-visible
   generated output.

### Modify JVM Support

1. Keep `internal/codegen/jvm_*.go` SDK-neutral. Java and Kotlin SDK APIs differ;
   share semantic collection, not SDK registration code.
2. Java uses low-level official SDK wiring through
   `McpServerTransportProvider.setSessionFactory(...)`.
3. Kotlin uses `io.modelcontextprotocol:kotlin-sdk-server` low-level server
   helpers.
4. Validate with focused Java/Kotlin codegen tests and the Gradle gate in
   `examples/jvm`.

### Modify TypeScript or JavaScript Support

1. TypeScript targets `@modelcontextprotocol/sdk`, Protobuf-ES `_pb.ts`, and
   strict NodeNext import rules.
2. Keep generated imports `.js`-suffixed and split type/value imports under
   `verbatimModuleSyntax`.
3. JavaScript support is consumption of compiled TypeScript output; do not add
   direct `lang=javascript` without revising the MVP decision.
4. Validate with `examples/node/sdk-spike`, focused TypeScript codegen tests,
   and standalone Node stdio tests.

### Add or Change MCP Options

1. Edit `mcp/options/v1/options.proto`; keep its direct `go_package`.
2. Regenerate shipped options with `easyp --cfg easyp.yaml generate -p mcp -r .`.
3. Update metadata extraction and schema/rendering only where the option
   actually applies.
4. Add fixture coverage in `internal/testproto` and refresh goldens.

### Check a Standalone Example with MCP Inspector

1. Build or set up the example first, for example:
   `cd examples/10_python_protobuf_standalone && make setup`.
2. Start Inspector from the example directory:
   `npx -y @modelcontextprotocol/inspector .venv/bin/python server.py`.
3. Open the printed Inspector URL, connect, run `List Tools`, and call at least
   one state-changing/read tool plus health when available.
4. Confirm Inspector reports `Tool Result: Success`, output schema validity,
   and structured/text parity.
5. Stop the Inspector process when done.

## Essential Commands

```bash
# Validate configs
easyp --cfg easyp.yaml validate-config
easyp --cfg easyp.test.yaml validate-config

# Lint/generate shipped options
easyp --cfg easyp.yaml lint -p mcp -r .
easyp --cfg easyp.yaml generate -p mcp -r .

# Lint/generate test fixtures
easyp --cfg easyp.test.yaml lint -p internal/testproto -r .
easyp --cfg easyp.test.yaml generate -p internal/testproto -r .

# Generate standalone examples
cd examples && make generate

# Full Go test suite
go test ./...

# Build plugin
go build ./cmd/protoc-gen-mcp
```

## Focused Verification Commands

```bash
# Python handler option and renderer contracts
go test ./internal/codegen -run 'TestParseOptions|TestGenerate_PythonMultipleHandlers|TestPythonRenderer_EmitsDataclassPublicAPI|TestPythonRenderer_EmitsProtobufHandlerPublicAPI|TestPythonRenderer_ProtobufHandlerImportsCrossFileProtobufModules|TestGenerate_PythonProtobufHandlerSkipsCrossFilePublicTypeModules' -count=1

# Java contracts and golden output
go test ./internal/codegen -run 'TestJavaContract_.*|TestGenerateJavaExampleGolden|TestGenerateJavaExampleHandlerCompileSmoke|TestGenerate_JavaTargetEmitsOutput' -count=1

# Kotlin contracts and golden output
go test ./internal/codegen -run 'TestGenerateKotlinExampleGolden|TestKotlinContract_.*' -count=1

# TypeScript semantic model, renderer, golden, and NodeNext compile smoke
go test ./internal/codegen -run 'TestTypeScriptModel|TestTypeScriptNames|TestTypeScriptContract|TestGenerateTypeScript|TestGenerate_TypeScript|TestTypeScriptGeneratedPublicAPICompilesUnderNodeNext' -count=1

# Generated Node stdio verification
go test ./internal/codegen -run 'TestTypeScriptGeneratedNodeServer.*OverStdio|TestTypeScriptGeneratedNodeServerRejectsInvalid(Input|Output)OverStdio' -count=1

# JVM compile/install gates
gradle --no-daemon -p examples/jvm :java-server:compileJava :kotlin-server:compileKotlin
gradle --no-daemon -p examples/jvm :java-server:installDist :kotlin-server:installDist

# Standalone examples
cd examples/5_python_standalone && make setup && make run
cd examples/10_python_protobuf_standalone && make setup && make run
cd examples/6_java_standalone && make build
cd examples/7_kotlin_standalone && make build
cd examples/8_typescript_standalone && make build && make run
cd examples/9_javascript_standalone && make build && make run

# Standalone stdio tests
go test ./examples -run 'TestStandalone(TypeScript|JavaScript)ExampleOverStdio' -count=1
go test ./examples -run 'TestStandalonePython(Protobuf)?ExampleOverStdio|TestPythonExamplesOverStdio' -count=1

# Installed JVM stdio tests
go test ./internal/examplemcp -run 'TestJava.*OverStdio' -count=1
go test ./internal/examplemcp -run 'TestKotlin.*OverStdio' -count=1
```

## Golden Snapshot Notes

- Go golden: `testdata/golden/example.mcp.go.golden`.
- Python goldens include dataclass/protobuf/dual handler outputs.
- Java golden: `testdata/golden/example_mcp.java.golden`.
- Kotlin golden: `testdata/golden/example_mcp.kt.golden`.
- TypeScript golden: `testdata/golden/example_mcp.ts.golden`.
- Regenerate fixtures through `easyp.test.yaml`; do not manually patch generated
  output unless diagnosing a renderer bug.

## Common Mistakes to Avoid

- Do not assume the project is Go-only; most changes must preserve Python, JVM,
  and TypeScript contracts.
- Do not make Go tests depend on globally installed `protoc` or Python
  packages. Use `protocompile` and `internal/pythontest` patterns.
- Do not add direct `lang=javascript` without changing the documented MVP rule.
- Do not collapse Python dual output into one module; `*_mcp.py` and
  `*_mcp_pb.py` have intentional compatibility meaning.
- Do not use SDK `addTool` shortcuts for JVM generated bindings; the project
  owns low-level protocol mapping and validation.
- Do not commit `.planning/**` artifacts unless the user explicitly asks.
- Do not forget to update `AGENTS.md` when technology choices, layout,
  supported features, public API, or verification commands change.
