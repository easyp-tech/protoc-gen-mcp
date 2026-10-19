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
| `mcpruntime` | Protobuf/ProtoJSON/JSON Schema adapters, MCP Apps metadata/resources and OAuth helpers over the official Go MCP SDK. No self-contained wire server or transports. |
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
| Go | `lang=go` | Tools: `<Service>ToolHandler`, `Register<Service>Tools(server, impl, opts...) error`. Prompts: `<File>PromptHandler`, `Register<File>Prompts(server, impl, opts...) error`. Resources: `<File>ResourceHandler`, `Register<File>Resources(ctx, server, impl, opts...) error`. `server` is the official SDK's `*mcp.Server`. |
| Python dataclass | `lang=python,python_runtime=google.protobuf` or `python_handler=dataclass` | `*_mcp.py`, dataclasses, `UNSET`, oneof wrappers, mapper helpers, `register_<service>_tools(...)` |
| Python protobuf | `lang=python,python_handler=protobuf` | `*_mcp.py` with raw `*_pb2` handler protocol and identity converters |
| Python dual | `lang=python,python_handler=dataclass+protobuf` | dataclass sidecar in `*_mcp.py` plus raw protobuf sidecar in `*_mcp_pb.py` |
| Kotlin | `lang=kotlin` | `<Service>ToolHandler`, `register<Service>Tools(server, impl, namespace = null)` |
| Java | `lang=java` | top-level `<ProtoFile>Mcp` class, nested `<Service>ToolHandler`, `register<Service>Tools(...)` |
| TypeScript | `lang=typescript` | `*_mcp.ts`, typed `<Service>ToolHandler`, `register<Service>Tools(server, impl, namespace?)` |
| JavaScript | no direct `lang=javascript` in v1.1 | consume compiled TypeScript `.js` plus `.d.ts` output |

Go `<File>` names use the exported CamelCase proto basename (for example,
`resources.proto` → `Resources`), scoped to `go_package`, not directory paths.
Path-derived names remain deprecated aliases; migrate before relocating files.
Generation rejects Go name collisions and unsupported task-support declarations
(`TASK_SUPPORT_OPTIONAL` / `REQUIRED`); omitted/`NONE` is synchronous.
Generated OAuth HTTP handlers return HTTP 403 `insufficient_scope` challenges
before tool dispatch. SDK clients use `OAuthHandler` for scope step-up/retry.

## Resources and Prompts (message-level)

Unlike tools (RPC-method-level, on a `service`), MCP **prompts** and **resources**
are declared on a **message** via `option (mcp.options.v1.prompt)` /
`option (mcp.options.v1.resource)` (extensions 91008 / 91009 in
`mcp/options/v1/options.proto`). Because they are file-scoped, their generated
Go registration functions are named after the proto **file**, not a service.

- **Prompt**: each field of the annotated message becomes a prompt argument
  (scalar/enum only; `optional` ⇒ not required). Handler returns
  `[]*mcp.PromptMessage`, with SDK content such as `&mcp.TextContent{Text: text}`.
  Arguments are parsed from `map[string]string`
  via `mcpruntime.ParsePromptArguments`.
- **Resource**: set exactly one of `uri` (static) or `uri_template` (templated,
  RFC-6570 simple `{param}`). Message fields are the output schema, serialized
  with ProtoJSON via `mcpruntime.MarshalResourceContent`. Templated resources
  generate a `List<Msg>s(ctx) ([]mcp.Resource, error)` (advertised
  instances) plus `Read<Msg>(ctx, <param>...) (*<Msg>, error)`; URI params are
  extracted with `mcpruntime.ExtractURIParams`. `Register<File>Resources` takes
  `ctx` first because it calls `List*` eagerly at registration time.
- `MarshalResourceContent` returns `[]*mcp.ResourceContents`. `content_field`
  emits raw string/blob content; `source_file` embeds a static file without a
  handler. MCP Apps resources carry SDK `_meta.ui` metadata.
- SDK `mcp.Annotations.Priority` is `float64`, and `Audience` is `[]mcp.Role`.
  The generator's `mcpruntime.ToolAnnotations` preserves all four hints as
  `*bool` (`proto.Bool(...)`); its adapter converts to SDK annotations, where
  `ReadOnlyHint` and `IdempotentHint` are plain `bool`, while `DestructiveHint`
  and `OpenWorldHint` remain pointers. Do not apply one pointer rule to both types.

## Go runtime (`mcpruntime`)

The Go target uses `github.com/modelcontextprotocol/go-sdk v1.8.0` (Go 1.26+).
Import `github.com/modelcontextprotocol/go-sdk/mcp`, construct the server with
`mcp.NewServer(&mcp.Implementation{Name: name, Version: version}, nil)`, register
generated tools/prompts/resources, then run
`server.Run(ctx, &mcp.StdioTransport{})`. For custom streams, use
`mcp.IOTransport{Reader: reader, Writer: writer}` with `io.ReadCloser` /
`io.WriteCloser` values. Python, JVM and TypeScript keep their native MCP SDKs.

Go bindings must be regenerated after the SDK migration. The old
`mcpruntime.Server`, `NewServer`, `ServeStdio`, `ServeIO`, transport/session
APIs and `HandleRaw` are removed. Generated tool registration uses
`mcpruntime.RegisterSDKProtoTool`; callers should continue using the generated
`Register<Service>Tools` API. Test protocol behavior with an SDK client and
`mcp.NewInMemoryTransports` instead of raw wire dispatch.

The protobuf `server` file option emits `New<File>MCPServer` and
`New<File>MCPHTTPHandler`. The latter, `NewConfiguredHTTPHandler` and
`NewOAuthResourceHandler` deliberately use stateless Streamable HTTP. No
persistent session IDs, legacy GET listen streams or replay are provided by
these helpers. Request-associated SSE notifications and new-protocol
`subscriptions/listen` remain SDK-owned. Legacy stateful HTTP requires direct
`mcp.NewStreamableHTTPHandler` configuration, an event store for resumption,
and application-owned OAuth middleware/discovery. Never expose an unwrapped
SDK handler assuming generated OAuth policy still applies. See
[`docs/mcp-apps-oauth.md`](../../../docs/mcp-apps-oauth.md) for migration and
session-owner authentication requirements.

Built-in RS256 JWKS validation uses a five-minute cache. Refreshes share one
in-flight fetch outside the cache mutex, with independent waiter cancellation
and exponential failed-fetch backoff (one second to one minute). Fresh known
keys remain usable; expired keys always fail closed until refresh succeeds.
Do not add a stale-key fallback or bypass issuer/audience/expiration checks.

Because generated resource/prompt code was historically only checked as text
goldens, keep it **compiled**: `internal/examplemcp` registers the generated
resource/prompt handlers so `go build ./...` and the round-trip test
(`TestResourcesPromptsRoundTrip`) exercise them.

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

- Go goldens: `testdata/golden/example.mcp.go.golden` (tools),
  `prompts.mcp.go.golden`, `resources.mcp.go.golden`.
- Python goldens include dataclass/protobuf/dual handler outputs.
- Java golden: `testdata/golden/example_mcp.java.golden`.
- Kotlin golden: `testdata/golden/example_mcp.kt.golden`.
- TypeScript golden: `testdata/golden/example_mcp.ts.golden`.
- Regenerate fixtures through `easyp.test.yaml`; do not manually patch generated
  output unless diagnosing a renderer bug.
- Regenerate goldens with the writer tests:
  `WRITE_GOLDEN=1 go test ./internal/codegen -run 'TestWrite(Example|Prompts|Resources)GoldenFiles'`.
  Goldens are text snapshots — they do not compile the output, so pair renderer
  changes with a compiled consumer (`internal/examplemcp`) to catch type errors.

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
