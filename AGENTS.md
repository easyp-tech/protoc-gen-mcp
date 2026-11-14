# AGENTS

## Scope

This repository implements a protobuf-first MCP generator and runtime for Go,
Python, Kotlin, Java, and TypeScript MCP server bindings. The MVP is
intentionally narrow and must stay decision-consistent with the current
architecture unless explicitly revised.

## Stack

- Go 1.26+
- Python 3.10+ for generated-runtime verification and example servers
- Python runtime tests bootstrap an isolated virtualenv through
  `internal/pythontest`; they must not depend on globally installed Python
  packages such as `protobuf`
- Gradle 9.2+ and JDK 17+ for JVM compile-gate verification
- `easyp v0.15.2-rc1` for repository linting and code generation workflows
- `google.golang.org/protobuf` for code generation, reflection, and ProtoJSON
- `google.protobuf` for Python generated modules and ProtoJSON conversion
- `github.com/modelcontextprotocol/go-sdk v1.8.0` is the Go target's
  official MCP server runtime, protocol negotiation and transports (stdio and
  stateless Streamable HTTP, including MCP 2026-07-28).
- `mcpruntime` contains protobuf/ProtoJSON/JSON Schema adapters, MCP Apps
  metadata/resource registration, and OAuth resource-server wiring. The
  self-hosted MCP wire runtime has been removed.
- `mcp>=1.30,<2` as the official Python MCP SDK target
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
- `internal/codegen`: shared semantic model, option parsing and the Go, Python,
  Kotlin, Java and TypeScript renderers (`render_*.go`, `jvm_*.go`,
  `python_*.go`, `typescript_*.go`)
- `internal/schema`: protobuf descriptor to JSON Schema conversion
- `mcpruntime`: protobuf, ProtoJSON and JSON Schema adapters, MCP Apps UI
  metadata/resources, and OAuth resource-server integration over the Go SDK
- `mcp/options/v1/options.proto`: custom protobuf options for MCP metadata,
  with checked-in Go (`options.pb.go`) and Python (`options_pb2.py`,
  `options_mcp.py`) output; `mcp/__init__.py` lets `mcp.options.*` coexist with
  the official Python MCP SDK package
- `internal/testproto`: protobuf fixtures and their generated Go/Python code
- `testdata/golden`: golden snapshots for generated Go, Python, Kotlin, Java
  and TypeScript bindings; `testdata/unsupported`: fail-fast fixtures
- `internal/examplemcp`: Go and Python servers for the shared ExampleAPI and
  resources/prompts fixtures and their stdio contract tests; `internal/examplemcp/jvm` is the
  Gradle workspace that builds the Java/Kotlin servers for the JVM stdio tests
- `runtime/typescript`: the `@easyp-tech/protoc-gen-mcp-runtime` npm package
  that generated TypeScript imports (tool, prompt and resource registries,
  dispatch, URI templates, content encoding); SDK, Protobuf-ES and Ajv are
  peer dependencies. The harness and example 8 depend on it through `file:`
  with `install-links=true`, so build it first (`npm ci && npm run build`).
  `GENCODE_VERSION` in `src/version.ts` moves with `typescriptGencodeVersion`
- `runtime/jvm`: Gradle build of `tech.easyp:protoc-gen-mcp-runtime`
  (`tech.easyp.mcp.runtime.McpRuntime`, Java) and
  `tech.easyp:protoc-gen-mcp-runtime-kotlin` (`tech.easyp.mcp.runtime.kotlin`),
  published to Maven Central through `com.vanniktech.maven.publish`. The JVM
  harness and examples 6 and 7 use it through `includeBuild`; contract tests
  read its sources through `withJVMRuntimeSource`. `GENCODE_VERSION` moves with
  `jvmGencodeVersion` in the renderer
- `internal/codegen/testdata/node`: pinned npm package (official TypeScript
  SDK, Protobuf-ES, Ajv) used by the TypeScript compile, runtime and stdio tests
- `runtime/python`: the `protoc-gen-mcp-runtime` PyPI package (module
  `protoc_gen_mcp_runtime`) that generated Python code imports: tool, prompt
  and resource registries, dispatch, URI templates and content encoding; its
  unittest suite runs through `go test ./runtime/python`. Bump
  `GENCODE_VERSION` together with `pythonGencodeVersion` in the renderer when
  generated code needs a new runtime API
- `internal/pythontest`: hermetic Python virtualenv bootstrap for Go tests that
  execute generated Python code
- `internal/stdiotest`: stdio JSON-RPC client and the resources/prompts
  contract run against the Go, Python, TypeScript, Java and Kotlin servers
- `internal/devtools/protobufesnormalize`: normalizer that keeps checked-in
  Protobuf-ES descriptors in `examples/8_typescript_standalone` deterministic
- `examples`: one runnable project per language — `1_helloworld` and
  `11_protobuf_first_mcp` (Go), `5_python_standalone`, `6_java_standalone`,
  `7_kotlin_standalone`, `8_typescript_standalone`; `examples/easyp.yaml` and
  `easyp.lock` resolve `mcp/options/v1` from GitHub, not the local root
- `easyp.yaml` (shipped options), `easyp.test.yaml` (Go output for test
  fixtures), `easyp.test.python.yaml` (Python output for every fixture except
  the Go-only `config/v1`),
  `easyp.showcase.yaml` (example 11)
- `.github/workflows`: `tests.yml` (CI and coverage gate), `release.yml`;
  `.goreleaser.yaml` packages the plugin binary

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
  by default unless they are `proto3 optional`, `repeated`, `map`, or `oneof`;
  MCP field options do not override requiredness
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
- MCP prompts and resources are declared on a message (not a service) via
  `option (mcp.options.v1.prompt)` / `option (mcp.options.v1.resource)`, so their
  Go registration is file-scoped: `Register<File>Prompts(server, impl, opts...) error`
  (handler `<File>PromptHandler`, one prompt argument per message field) and
  `Register<File>Resources(ctx, server, impl, opts...) error` (handler
  `<File>ResourceHandler`; static `uri` ⇒ `Read<Msg>(ctx)`, templated
  `uri_template` ⇒ `List<Msg>s(ctx)` + `Read<Msg>(ctx, <param>...)`; ctx-first
  because `List*` runs at registration time). Resource output is ProtoJSON or
  a raw text/blob `content_field`. Python, Kotlin, Java and TypeScript expose
  the same file-scoped `register<File>Prompts` / `register<File>Resources`
  (Kotlin resources registration is `suspend`); outside Go a `source_file`
  resource is read through a handler returning the file body. Unknown
  resource URIs return Invalid Params (-32602, SEP-2164); Kotlin keeps its
  SDK's -32002
- Java sidecars on one transport register into one `McpRuntime` registry, so
  tools, prompts and resources from different proto files compose; the session
  factory is installed once because stdio opens its session inside
  `setSessionFactory`
- Generated Python modules expose `<Service>ToolHandler`
- Generated Python modules expose `register_<service_name>_tools(server, impl, *, namespace=None)`
- Generated Python modules default to `python_handler=dataclass`, exposing
  dataclasses, `UNSET`, explicit `oneof` wrapper variants, mapper helpers, and
  handler protocols using generated public dataclass types from `*_mcp.py`
- Generated Python modules also support opt-in `python_handler=protobuf`,
  where handler protocols accept and return raw generated `*_pb2` message
  classes, generated dataclass public types/mapper helpers are omitted, and
  registration remains `register_<service_name>_tools(server, impl, *, namespace=None)`
- Generated Python modules support dual output with
  `python_handler=dataclass+protobuf`: `*_mcp.py` remains the dataclass sidecar,
  and `*_mcp_pb.py` exposes the raw protobuf sidecar with the same module-local
  `<Service>ToolHandler` and `register_<service_name>_tools(...)` names
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
- Go protobuf file option `(mcp.options.v1.server)` generates a typed
  `New<File>MCPServer` constructor and optional configured
  `New<File>MCPHTTPHandler`. Never put bearer tokens, client secrets or
  identity-provider internals into public protobuf options.
- File-scoped Go public names use the exported CamelCase proto basename,
  excluding directories and `.proto` (for example, `server_config.proto`
  generates `ServerConfigMCPHandlers` and `NewServerConfigMCPServer`). The
  resolved `go_package` supplies the namespace. Canonical and compatibility
  names are checked for collisions across generated files and protobuf symbols
  before rendering. Previous descriptor-path names remain deprecated type
  aliases and function wrappers; they reflect the current descriptor path, so
  callers should migrate to basename APIs before relocating a proto.
- Go MCP Apps resources support static `source_file` (Go embed) and
  `app_ui` CSP/permissions metadata, alongside `content_field` for dynamic
  text/blob resources. Static `source_file` resources require no Go handler.
- Every target generates the file-scoped server factory and `<File>MCPHandlers`
  when a file has `option (mcp.options.v1.server)` or needs MCP Apps
  (`ServerFactoryModel`, `modelNeedsApps`). Kotlin and Java over stdio advertise
  MCP Apps under `capabilities.experimental`; the shared contract takes this as
  `stdiotest.ContractOptions{AppsCapability: AppsInExperimental}`. After
  changing `runtime/typescript`, rebuild it and rerun `npm ci` in its consumers:
  `npm install` keeps the stale copy of a same-version `file:` package
- Go, Python, TypeScript and Java implement `option (mcp.options.v1.server).oauth`
  and per-tool `required_scopes`; Kotlin still rejects them at
  generation (`validateGoOnlyOptions`) so no tool is served without its scope
  check. `internal/httpcontract` is the shared HTTP/OAuth contract (metadata,
  401 challenge, 403 step-up with `scope=`, 400/413 preflight) run against the
  config/v1 fixture of each implementing target. Python's HTTP app
  (`protoc_gen_mcp_runtime.new_http_app`) layers the SDK
  `StreamableHTTPSessionManager(stateless=True)`, `AuthenticationMiddleware`,
  `BearerAuthBackend` and `RequireAuthMiddleware` with the runtime's scope gate
  and metadata; `JWKSTokenVerifier` ports `mcpruntime/jwks.go` on PyJWT and httpx.
  TypeScript's `newHttpHandler` is an express app: SDK `requireBearerAuth` before
  `express.json` (4 MiB), the runtime scope gate, then a fresh `Server` and
  `StreamableHTTPServerTransport` per request; its public types avoid express
  and Node typings (`TokenVerifier`, structural `HttpHandler`).
  Java's `McpHttpServlet` wraps the SDK `HttpServletStatelessServerTransport`
  with a runtime `McpStatelessServerHandler` over the registry of an
  `McpHttpServer` key (so `initialize` can carry `capabilities.extensions`),
  and does metadata, bearer auth and the scope gate itself; the token reaches
  handlers through the transport context and `McpRuntime.currentAccessToken()`.
  `runtime/jvm/oauth` (`protoc-gen-mcp-runtime-oauth`) holds the JVM-wide
  pieces: `JWKSTokenVerifier` on nimbus-jose-jwt, `OAuthResourceServer`,
  `ScopeGate` and a strict JSON reader; the Kotlin runtime is meant to reuse it.
  Method `app_ui` becomes tool `_meta.ui` in every target.
- Generated OAuth HTTP handlers enforce server-wide and per-tool
  `required_scopes` before dispatch, returning HTTP 403 `insufficient_scope`
  challenges for missing scopes. SDK client `OAuthHandler` can perform step-up
  and retry. Callback scope guards remain in place for other transports;
  missing tokens are denied.
- RS256 JWKS verification requires HTTPS JWKS, issuer and audience; custom
  `auth.TokenVerifier` remains supported for opaque or provider-specific tokens.
- Go generation rejects `TASK_SUPPORT_REQUIRED` because the official Go SDK
  does not implement the MCP 2025-11-25 Tasks lifecycle. Omitted,
  `TASK_SUPPORT_NONE` and `TASK_SUPPORT_OPTIONAL` stay synchronous.
- Go runtime transports: SDK `StdioTransport` / `NewStreamableHTTPHandler`
  (stateless HTTP by default, with SDK-managed protocol negotiation and
  cancellation). Legacy transport implementation is removed
- Generated MCP tool names must not contain dots; namespace prefixes and method
  names are joined with underscores, and any dots in configured segments are
  normalized to underscores
- Generated tool annotations are forwarded exactly as declared in proto
  options; omitted hints stay omitted, so external clients may apply their own
  display defaults for missing values

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

- Validate configs: `easyp --cfg easyp.yaml validate-config`,
  `easyp --cfg easyp.test.yaml validate-config`
- Lint / generate shipped options: `easyp --cfg easyp.yaml lint -p mcp -r .`,
  `easyp --cfg easyp.yaml generate -p mcp -r .`
- Lint / generate test fixtures:
  `easyp --cfg easyp.test.yaml lint -p internal/testproto -r .`,
  `easyp --cfg easyp.test.yaml generate -p internal/testproto -r .`,
  `easyp --cfg easyp.test.python.yaml generate -p internal/testproto -r .`
- Generate the showcase:
  `easyp --cfg easyp.showcase.yaml generate -p examples/11_protobuf_first_mcp/proto -r .`
- Generate standalone examples: `cd examples && make generate`
- Prepare the TypeScript runtime and harness:
  `cd runtime/typescript && npm ci && npm run build`, then
  `cd internal/codegen/testdata/node && npm ci && npm run typecheck`
- Build and test the JVM runtime: `gradle --no-daemon -p runtime/jvm build`
- Build and install the JVM test servers:
  `gradle --no-daemon -p internal/examplemcp/jvm :java-server:installDist :kotlin-server:installDist`
- Run tests: `go test ./...` (TypeScript tests need the harness installed;
  JVM tests skip without a working JDK and Gradle)
- Run JVM stdio tests: `go test ./internal/examplemcp -run 'Test(Java|Kotlin).*OverStdio' -count=1`
- Run example 11: `go run ./examples/11_protobuf_first_mcp -transport=http -demo-auth`
- Build plugin: `go build ./cmd/protoc-gen-mcp`
- Validate GoReleaser config: `goreleaser check`
