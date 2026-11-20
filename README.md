# protoc-gen-mcp

<p align="center">
  <a href="https://github.com/easyp-tech/protoc-gen-mcp/actions/workflows/tests.yml"><img alt="CI" src="https://github.com/easyp-tech/protoc-gen-mcp/actions/workflows/tests.yml/badge.svg"></a>
  <a href="https://coveralls.io/github/easyp-tech/protoc-gen-mcp?branch=master"><img alt="Coverage Status" src="https://coveralls.io/repos/github/easyp-tech/protoc-gen-mcp/badge.svg?branch=master"></a>
  <a href="https://pkg.go.dev/github.com/easyp-tech/protoc-gen-mcp"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/easyp-tech/protoc-gen-mcp.svg"></a>
  <a href="https://github.com/easyp-tech/protoc-gen-mcp/releases/latest"><img alt="Release" src="https://img.shields.io/github/v/release/easyp-tech/protoc-gen-mcp?sort=semver"></a>
  <a href="https://github.com/easyp-tech/protoc-gen-mcp/blob/master/LICENSE"><img alt="License" src="https://img.shields.io/github/license/easyp-tech/protoc-gen-mcp"></a>
  <a href="https://github.com/easyp-tech/protoc-gen-mcp/blob/master/go.mod"><img alt="Go version" src="https://img.shields.io/github/go-mod/go-version/easyp-tech/protoc-gen-mcp?logo=go&logoColor=white"></a>
</p>


`protoc-gen-mcp` generates Go, Python, Kotlin, Java and TypeScript MCP
bindings from protobuf definitions — covering **tools**, **prompts**, and
**resources**. Protobuf is the source of truth: RPCs become tools, annotated
messages become prompts and resources, and the generated code validates
ProtoJSON payloads against generated JSON Schema before calling your typed
handlers. JavaScript projects consume the compiled TypeScript output.

A **Swift** target is merged on `master` and ships in v1.1.0 (not tagged
yet); the latest release, v1.0.0, does not include it. Everything marked
Swift below refers to `master`.

## Supported MCP Primitives

| Primitive | Go | Python | Kotlin | Java | TypeScript | Swift² |
|-----------|:--:|:------:|:------:|:----:|:----------:|:-----:|
| **Tools** | ✅ Full | ✅ Full | ✅ Full | ✅ Full | ✅ Full | ✅ Full |
| **Prompts** | ✅ Full | ✅ Full | ✅ Full | ✅ Full | ✅ Full | ✅ Full |
| **Resources** | ✅ Full | ✅ Full | ✅ Full | ✅ Full | ✅ Full | ✅ Full |
| **Protobuf server factory** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **MCP Apps UI / CSP** | ✅ | ✅ | ✅¹ | ✅¹ | ✅ | ✅ |
| **Markdown, HTML, binary resources** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **OAuth/JWKS and tool scopes** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

² On `master` only; ships in v1.1.0.

Every target serves the same resources contract: ProtoJSON or raw text/blob
bodies, percent-decoded template parameters, annotations, icons and the MCP
Apps `_meta.ui` of a resource. Only Go embeds `source_file` at build time;
the other targets read it through a generated handler that returns the file
body.

Every target generates a file-scoped server factory from
`option (mcp.options.v1.server)`: Python `create_<file>_mcp_server(handlers)`,
TypeScript `create<File>MCPServer(handlers)`, Swift (`master`)
`new<File>MCPServer(handlers) async throws`, Kotlin
`suspend new<File>MCPServer(handlers)` and Java
`<ProtoFile>Mcp.install<File>MCPServer(transportProvider, handlers)`. ¹ The
Kotlin SDK 0.11 and the stdio transport of the Java SDK 1.1 cannot express
`capabilities.extensions`, so they advertise MCP Apps under
`capabilities.experimental`; Java over HTTP serves `capabilities.extensions`
like Go. Tools and resources carry `_meta.ui` everywhere. Every target serves
`option (mcp.options.v1.server).oauth` and per-tool `required_scopes` over
stateless Streamable HTTP with the same metadata, 401 challenge and 403
step-up, checked by one shared contract.

Each target uses the official MCP SDK of its language: the Go MCP SDK,
the Python `mcp` SDK with `google.protobuf`, `io.modelcontextprotocol.sdk:mcp`
for Java, `io.modelcontextprotocol:kotlin-sdk-server` for Kotlin, and
`@modelcontextprotocol/sdk` with Protobuf-ES and Ajv for TypeScript, and
`modelcontextprotocol/swift-sdk` with swift-protobuf for Swift (`master`).

## Install

The latest release is
[v1.0.0](https://github.com/easyp-tech/protoc-gen-mcp/releases/tag/v1.0.0).
Install the plugin with Go 1.26+:

```bash
go install github.com/easyp-tech/protoc-gen-mcp/cmd/protoc-gen-mcp@v1.0.0
```

or download a prebuilt binary from the release:
`protoc-gen-mcp-<version>-<os>-<arch>.tar.gz` for `linux` and `darwin`
and `protoc-gen-mcp-<version>-windows-<arch>.zip`, each for `amd64` and
`arm64`, plus `protoc-gen-mcp-<version>-checksums.txt`. Put
`protoc-gen-mcp` on `PATH`.

[Easyp](#generation-with-easyp) is the recommended driver. With plain
`protoc` the plugin is `--mcp_out`, and `--mcp_opt=lang=` selects the target
(`go` by default, or `python`, `java`, `kotlin`, `typescript`; `swift`
needs a build from `master`). `mcp/options/v1/options.proto` must be on the
include path, for example from a clone of this repository at the release
tag:

```bash
# Go, next to protoc-gen-go output
protoc -I . -I path/to/protoc-gen-mcp \
  --go_out=. --go_opt=paths=source_relative \
  --mcp_out=. --mcp_opt=paths=source_relative \
  proto/weather/v1/weather.proto

# Python; also generate the options module the generated code imports
protoc -I . -I path/to/protoc-gen-mcp \
  --python_out=. --mcp_out=. --mcp_opt=lang=python \
  proto/weather/v1/weather.proto mcp/options/v1/options.proto
```

Generated code imports a small runtime of its language; see
[Runtime packages](#runtime-packages).

## Quickstart: run the full Go showcase

The [protobuf-first showcase](examples/11_protobuf_first_mcp/README.md)
runs one server with generated tools, prompts, JSON/text/blob resources,
embedded `SKILL.md`, an MCP Apps UI resource, CSP/permissions, and OAuth
resource-server configuration. With **Go 1.26+**, from a clone of the
repository:

```bash
git clone https://github.com/easyp-tech/protoc-gen-mcp.git
cd protoc-gen-mcp

# Terminal A: explicitly enable local development-only OAuth tokens.
go run ./examples/11_protobuf_first_mcp -transport=http -demo-auth

# Terminal B: use the official MCP Go SDK client.
go run ./examples/11_protobuf_first_mcp/client -token=demo-read
go run ./examples/11_protobuf_first_mcp/client -token=demo-write
```

A read-only token cannot invoke the write-scoped tool; a write token can.
`-demo-auth` is intentionally insecure and loopback-only; never expose these
tokens publicly. Sample issuer/JWKS URLs in the protobuf fixture are
placeholders, **not** an identity provider.

To inspect any generated server interactively, point the official
[MCP Inspector](https://modelcontextprotocol.io/docs/tools/inspector) at its
command, for example
`npx -y @modelcontextprotocol/inspector go run ./examples/1_helloworld`.

## Generation With Easyp

The intended workflow is [easyp](https://github.com/easyp-tech/easyp), not
manual `protoc` invocation. One config can drive `protoc-gen-go`, the standard
Python generator, Protobuf-ES and `protoc-gen-mcp`. If your config uses the
standard Go plugin, install it first:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
```

Example `easyp.yaml` with every target enabled (keep only the ones you need):

```yaml
deps:
  - github.com/easyp-tech/protoc-gen-mcp

generate:
  inputs:
    - directory:
        path: proto
        root: "."
  plugins:
    # Go
    - name: go
      out: .
      opts:
        paths: source_relative
    - command: ["go", "run", "github.com/easyp-tech/protoc-gen-mcp/cmd/protoc-gen-mcp@v1.0.0"]
      out: .
      opts:
        paths: source_relative
    # Python
    - name: python
      with_imports: true
      out: .
    - command: ["go", "run", "github.com/easyp-tech/protoc-gen-mcp/cmd/protoc-gen-mcp@v1.0.0"]
      out: .
      opts:
        lang: python
        # python_handler: protobuf            # raw *_pb2 handlers
        # python_handler: dataclass+protobuf  # both, raw ones in *_mcp_pb.py
    # Java and Kotlin both need protobuf Java output; Kotlin also needs
    # protobuf Kotlin output (examples 6 and 7)
    - name: java
      with_imports: true
      out: src/generated/main/java
    - name: kotlin # Kotlin only
      out: src/generated/main/kotlin
    - command: ["go", "run", "github.com/easyp-tech/protoc-gen-mcp/cmd/protoc-gen-mcp@v1.0.0"]
      out: src/generated/main/java # src/generated/main/kotlin for lang=kotlin
      opts:
        paths: source_relative
        lang: java # or kotlin
    # TypeScript
    - command: ["./node_modules/.bin/protoc-gen-es"]
      with_imports: true
      out: src/generated
      opts:
        target: ts
        import_extension: js
    - command: ["go", "run", "github.com/easyp-tech/protoc-gen-mcp/cmd/protoc-gen-mcp@v1.0.0"]
      out: src/generated
      opts:
        lang: typescript
    # Swift, master only until v1.1.0: protoc-gen-swift must be on PATH
    - name: swift
      out: Sources/App/Generated
    - command: ["go", "run", "github.com/easyp-tech/protoc-gen-mcp/cmd/protoc-gen-mcp@master"]
      out: Sources/App/Generated
      opts:
        paths: source_relative
        lang: swift
```

```bash
easyp --cfg easyp.yaml mod download
easyp --cfg easyp.yaml generate -p proto -r .
```

Pin a release tag (not `@latest`) for reproducible builds; the Swift
target needs `@master` until v1.1.0 is tagged. Protos used only for Python,
JVM, TypeScript or Swift do not need a `go_package` option; the plugin
synthesizes the metadata it needs. `mcp.options.v1` declares its own
`go_package`, so no Easyp override is required.

Language notes:

- **Python** supports `python_runtime=google.protobuf`. The default
  `python_handler=dataclass` hands your code dataclasses from `*_mcp.py`, with
  `UNSET` (not `None`) for absent optional fields and explicit `oneof` wrapper
  variants. `python_handler=protobuf` uses raw `*_pb2` messages instead, and
  `dataclass+protobuf` emits both. Generation also writes a small
  `mcp/__init__.py` bridge so `mcp.options.*` modules coexist with the official
  `mcp` SDK package. Enable `with_imports: true` on the Python plugin when your
  protos import other packages. Generated modules import the
  `protoc-gen-mcp-runtime` package, which holds the tool, prompt and resource
  registries; it is not on PyPI yet, so install the wheel attached to the
  release (see [Runtime packages](#runtime-packages)). Generated code checks
  on import that the runtime matches it. With
  `option (mcp.options.v1.server)` a module also exports
  `create_<file>_mcp_http_app(server, verifier=None)`, an ASGI app (for
  example for uvicorn) that serves stateless Streamable HTTP through the SDK
  session manager and, with `oauth`, enforces bearer tokens, RFC 9728
  metadata and per-tool `required_scopes` exactly like the Go handler;
  `verifier` is an SDK `TokenVerifier`, and without one the built-in RS256
  JWKS verifier is used.
- **Java** (`lang=java`) generates one `<ProtoFile>Mcp` sidecar per proto next
  to protobuf Java output. **Kotlin** (`lang=kotlin`) needs Java protobuf
  output, Kotlin protobuf output, and the MCP sidecar together. Generated
  sidecars depend on `tech.easyp:protoc-gen-mcp-runtime` (Java) or
  `tech.easyp:protoc-gen-mcp-runtime-kotlin` (Kotlin), which hold the
  registries and dispatch; sidecars of several proto files share one registry
  per transport or server. With `option (mcp.options.v1.server)` a Java
  sidecar also exports `new<File>MCPHttpServlet(server[, verifier])`: fill a
  `tech.easyp.mcp.runtime.McpHttpServer` with `install<File>MCPServer` and
  mount the returned servlet at `/*` in any Jakarta Servlet 6 container. MCP
  is served by the SDK `HttpServletStatelessServerTransport`; with `oauth` the
  servlet adds bearer authentication, RFC 9728 metadata and the per-tool
  `required_scopes` step-up of the Go handler. `verifier` is a
  `tech.easyp.mcp.runtime.oauth.TokenVerifier`; when it is null the built-in
  RS256 JWKS verifier (nimbus-jose-jwt, from
  `tech.easyp:protoc-gen-mcp-runtime-oauth`) is used. Handlers read the token
  with `McpRuntime.currentAccessToken()`; the `McpAsyncServerExchange`
  argument is null over stateless HTTP. A Kotlin file with the server option
  exports `Application.install<File>MCPHttp(handlers, verifier = null)` for
  Ktor: a route that builds a fresh server per request on the SDK
  `StreamableHttpServerTransport` (as the SDK's own stateless endpoint does)
  and adds the same metadata, bearer check and scope step-up; suspend
  handlers read the token with `currentAccessToken()`.
- **TypeScript** (`lang=typescript`) expects Protobuf-ES `_pb.ts` files in the
  same tree, generated with `import_extension=js` for Node ESM and
  `moduleResolution: NodeNext`. **JavaScript** projects import the compiled
  `.js` output and `.d.ts` declarations; there is no `lang=javascript`.
  Generated files import `@easyp-tech/protoc-gen-mcp-runtime`, which keeps
  the tool, prompt and resource registries; it is not on npm yet, so install
  the tarball attached to the release (see
  [Runtime packages](#runtime-packages)). `@modelcontextprotocol/sdk`,
  `@bufbuild/protobuf` and `ajv` are its peer dependencies, so the server and
  the runtime share one SDK. With `option (mcp.options.v1.server)` a file also
  exports `create<File>MCPHttpHandler(handlers, verifier?)`, a request
  listener for `node:http` or express that builds a fresh server per request
  (the stateless SDK transport requires it) and, with `oauth`, uses the SDK
  `requireBearerAuth` plus the runtime scope gate and RFC 9728 metadata;
  `verifier` defaults to the built-in RS256 JWKS verifier on `jose`.
- **Swift** (`lang=swift`, on `master`, ships in v1.1.0) writes
  `<file>.mcp.swift` next to the `<file>.pb.swift` of `protoc-gen-swift`
  (swift-protobuf 1.39+), in the same module. The runtime ships from this
  repository as a Swift package resolved by git, like the Go module, so no
  registry is involved. Until v1.1.0 is tagged, depend on the branch:
  `.package(url: "https://github.com/easyp-tech/protoc-gen-mcp.git", branch: "master")`
  (after the release, `from: "1.1.0"`), with the products
  `ProtocGenMCPRuntime` and, for files with a server option,
  `ProtocGenMCPHTTP`. Handlers are `Sendable` protocols with `async throws`
  methods that receive a `ToolContext` carrying the bearer token. Start a
  server with `serve(server, transport:)`, which advertises MCP Apps under
  `capabilities.extensions` through a transport wrapper because the SDK's typed
  capabilities cannot express it. `new<File>MCPHTTPHandler(handlers, verifier:)`
  returns a framework-neutral `MCPHTTPHandler` (SDK `HTTPRequest` in,
  `MCPHTTPResponse` out) to mount in Hummingbird, Vapor or SwiftNIO; it uses
  the SDK `StatelessHTTPServerTransport` and, with `oauth`, the RS256 JWKS
  verifier on jwt-kit. swift-protobuf cannot print default values, so
  generated field tables complete ProtoJSON output like protojson
  `EmitDefaultValues`. `TASK_SUPPORT_REQUIRED` is rejected as for Go.
  See [examples/9_swift_standalone](examples/9_swift_standalone).

### Runtime packages

The runtimes are versioned with the plugin, but **none of them is published
to a package registry yet**: there is no PyPI, npm or Maven Central release.
Every GitHub release carries the Python wheel and the npm tarball, so
install them straight from the release:

```bash
pip install https://github.com/easyp-tech/protoc-gen-mcp/releases/download/v1.0.0/protoc_gen_mcp_runtime-1.0.0-py3-none-any.whl
npm install https://github.com/easyp-tech/protoc-gen-mcp/releases/download/v1.0.0/easyp-tech-protoc-gen-mcp-runtime-1.0.0.tgz
```

The JVM runtimes (`tech.easyp:protoc-gen-mcp-runtime`,
`protoc-gen-mcp-runtime-kotlin` and `protoc-gen-mcp-runtime-oauth`) are not
attached to the release. Build them from a checkout of the release tag with
`gradle -p runtime/jvm publishToMavenLocal` and add `mavenLocal()` to your
repositories, or use them through a Gradle `includeBuild` of `runtime/jvm`
as examples 6 and 7 do. Swift resolves its runtime from this repository by
git (see above). The examples in this repository use the in-repo runtimes
directly.

## Generated API

Every target exposes a typed handler interface per service and a registration
helper. Tool names are `{namespace}_{method}`; dots in either segment become
underscores, so namespace `weather.v1` and RPC `GetForecast` become
`weather_v1_GetForecast`.

| Target | Handler | Registration |
|---|---|---|
| Go | `<Service>ToolHandler` | `Register<Service>Tools(server *mcp.Server, impl, opts...) error` |
| Python | `<Service>ToolHandler` protocol | `register_<service>_tools(server, impl, *, namespace=None)` |
| Kotlin | `<Service>ToolHandler` | `register<Service>Tools(server, impl, namespace: String? = null)` |
| Java | `<ProtoFile>Mcp.<Service>ToolHandler` | `<ProtoFile>Mcp.register<Service>Tools(transportProvider, impl, namespace)` |
| TypeScript | `<Service>ToolHandler` | `register<Service>Tools(server, impl, namespace?)` |
| Swift (`master`) | `<Service>ToolHandler` protocol | `register<Service>Tools(server, impl, namespace: String? = nil) async throws` |

Prompts and resources are declared on messages, so their Go registration is
file-scoped: `Register<File>Prompts(server, impl, opts...)` and
`Register<File>Resources(ctx, server, impl, opts...)`, where `<File>` is the
CamelCase proto basename. A static resource (`uri`) needs `Read<Msg>(ctx)`; a
template resource (`uri_template`) needs `List<Msg>s(ctx)` and
`Read<Msg>(ctx, params...)` with percent-decoded template values.

The other targets follow the same shape:

| Target | Prompts | Resources |
|---|---|---|
| Python | `register_<file>_prompts(server, impl, *, namespace=None)` | `register_<file>_resources(server, impl, *, namespace=None)` |
| Kotlin | `register<File>Prompts(server, impl, namespace)` | `suspend fun register<File>Resources(server, impl, namespace)` |
| Java | `<ProtoFile>Mcp.register<File>Prompts(transportProvider, impl, namespace)` | `<ProtoFile>Mcp.register<File>Resources(transportProvider, impl, namespace)` |
| TypeScript | `register<File>Prompts(server, impl, namespace?)` | `register<File>Resources(server, impl, namespace?)` |
| Swift (`master`) | `register<File>Prompts(server, impl, namespace:) async throws` | `register<File>Resources(server, impl, namespace:) async throws` |

Resource handlers return the generated message type of the target (a Python
dataclass or `*_pb2` message, a Protobuf-ES message, a Java/Kotlin protobuf
message). Like Go, Kotlin lists template instances once, at registration, so
its Server must declare the resources and prompts capabilities; Python,
TypeScript and Java list them on every `resources/list` and declare the
capabilities themselves.

At runtime the generated code validates arguments against the input schema,
unmarshals them with ProtoJSON, calls your handler, marshals the response with
ProtoJSON, validates it against the output schema, and returns it as both
`structuredContent` and text content. Handler errors become tool results with
`isError: true`.

Minimal Go server:

```go
package main

import (
	"context"
	"log"

	weatherv1 "github.com/acme/weather/gen/weather/v1"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type handler struct{}

func (handler) Forecast(_ context.Context, req *weatherv1.GetForecastRequest) (*weatherv1.GetForecastResponse, error) {
	return &weatherv1.GetForecastResponse{ReportId: "forecast-1"}, nil
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{Name: "weather-mcp", Version: "v0.1.0"}, nil)
	if err := weatherv1.RegisterWeatherAPITools(server, handler{}); err != nil {
		log.Fatal(err)
	}
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
```

The [examples](examples/README.md) show the same wiring for Python, Java,
Kotlin, TypeScript and Swift (`master`).

### Go: HTTP, protobuf-defined servers, Apps and OAuth

Generated registration accepts `*mcp.Server` from the official Go SDK, which
owns transports, sessions and protocol negotiation. A plain Streamable HTTP
endpoint:

```go
handler := mcp.NewStreamableHTTPHandler(
	func(*http.Request) *mcp.Server { return server },
	&mcp.StreamableHTTPOptions{Stateless: true},
)
mux := http.NewServeMux()
mux.Handle("/mcp", http.NewCrossOriginProtection().Handler(handler))
_ = http.ListenAndServe("127.0.0.1:8080", mux)
```

That endpoint is **unprotected**. For public servers, declare the file-level
`(mcp.options.v1.server)` option instead: it carries name, version, MCP Apps
capabilities and OAuth resource-server settings, and the generator emits a
typed `New<File>MCPServer(ctx, handlers)` factory plus
`New<File>MCPHTTPHandler(server, verifier)`. OAuth verifies RS256 access JWTs
against a configured HTTPS JWKS endpoint, issuer and audience, or calls your
own `auth.TokenVerifier`. Per-RPC `required_scopes` add authorization; a
missing scope returns HTTP 403 `insufficient_scope` before dispatch, so SDK
clients can step up and retry. Never put bearer tokens or client secrets in
protobuf options.

Resources can also carry raw bodies: set `mime_type` and `content_field` to a
singular `string` or `bytes` field for dynamic text or blobs, or `source_file`
for a static file that the generator embeds with `go:embed` (no handler
needed). A static resource with `mime_type: "text/html;profile=mcp-app"` and
`app_ui` CSP/permissions becomes an MCP Apps UI. Go rejects
`execution.task_support: TASK_SUPPORT_REQUIRED` because the Go SDK does not
implement the Tasks lifecycle; other values stay synchronous. The
[showcase](examples/11_protobuf_first_mcp/README.md) exercises all of this.

## Schema Contract

Generated `inputSchema` and `outputSchema` follow ProtoJSON:

- `int64`, `uint64`, `fixed64`, `sfixed64`, `sint64` are JSON strings; 32-bit
  integers are JSON integers
- `float` / `double` accept numbers and the strings `NaN`, `Infinity`,
  `-Infinity`
- `bytes` are base64 strings and enums use value names
- `Timestamp` is an RFC 3339 string, `Duration` a string such as `"3600s"`,
  `FieldMask` a field-mask string
- `Struct`, `Value` and `ListValue` accept arbitrary JSON; `Any` uses the object
  form with `@type`
- recursive messages use `$defs` / `$ref`; `oneof` groups become JSON Schema
  constraints

Requiredness is proto3-driven: a singular field without `optional` is required;
`optional`, `repeated`, `map` and `oneof` fields are not, and they accept
explicit JSON `null` so clients that pre-validate cached schemas stay
compatible with ProtoJSON unset semantics. Field options do not change
requiredness.

Supported scope: proto3 only, unary RPCs only, and the well-known types `Any`,
`Empty`, `Timestamp`, `Duration`, `FieldMask`, `Struct`, `Value`, `ListValue`
and all scalar wrappers. Streaming RPCs and other `google.protobuf` messages
fail at generation time.

## Metadata Options

Import `mcp/options/v1/options.proto` to control generation. The proto file
documents every field; this is the summary.

- **File** `option (mcp.options.v1.server)`: `name` (default: the proto
  basename), `version` (default `v0.0.1`), `apps.enabled` (unset follows
  whether the file uses MCP Apps), and `oauth` with `resource_url`,
  `authorization_servers`, `scopes`, `issuer`, `jwks_uri`, `audience` and
  `mcp_path` (default `/mcp`). Every target then generates the file's server
  factory and Streamable HTTP handler.
- **Service** `option (mcp.options.v1.service)`: `namespace`, `description`,
  default `icons`.
- **Method** `option (mcp.options.v1.method)`: `name`, `title`, `description`,
  `hidden`, `annotations` (`title`, `read_only_hint`, `destructive_hint`,
  `idempotent_hint`, `open_world_hint`), `icons`, `execution.task_support`,
  `app_ui` (`resource_uri`, `visibility`) for MCP Apps tool `_meta.ui`, and
  `required_scopes` for per-tool OAuth scopes. Omitted hints stay omitted;
  some clients render their own defaults, so set hints such as
  `destructive_hint: false` explicitly when it matters.
- **Field** `[(mcp.options.v1.field)]`: `description`, typed `examples`,
  `default_value`, string constraints (`pattern`, `format`, `min_length`,
  `max_length`), number constraints (`minimum`, `maximum`,
  `exclusive_minimum`, `exclusive_maximum`, `multiple_of`), array constraints
  (`min_items`, `max_items`, `unique_items`), `read_only`.
- **Message** `option (mcp.options.v1.message)`: `title`, `description`,
  `examples`. **Enum** `option (mcp.options.v1.enum)`: `title`,
  `description`. **Enum value** `[(mcp.options.v1.enum_value)]`:
  `description`, `hidden` (typically for `_NONE` sentinels). **Oneof**
  `option (mcp.options.v1.oneof)`: `description`, `required`.
- **Prompt** `option (mcp.options.v1.prompt)` on a message: `name`, `title`,
  `description`, `icons`. Fields become prompt arguments; `optional` ones are
  not required.
- **Resource** `option (mcp.options.v1.resource)` on a message: exactly one of
  `uri` or `uri_template` (`{param}` placeholders), `name`, `description`,
  `mime_type` (default `application/json`), `annotations` (`audience`,
  `priority`), `icons`, `content_field` (a singular `string` or `bytes` field
  served as raw text or blob), `source_file` (a static file body: Go embeds
  it, the other targets return it from a generated handler method) and
  `app_ui` (`csp`, `permissions`, `domain`, `prefers_border`) for MCP Apps
  UI resources. All of them work in every target.

Comments are metadata too: plain lines become descriptions, `Example: <json>`
adds one schema example and `Examples: a | b` adds several. Typed `examples`
in field options take precedence.

```proto
service WeatherAPI {
  option (mcp.options.v1.service) = { namespace: "weather" };

  // Forecast returns a weather report.
  rpc Forecast(GetForecastRequest) returns (GetForecastResponse) {
    option (mcp.options.v1.method) = {
      name: "GetForecast"
      title: "Get forecast"
      annotations: { read_only_hint: true }
    };
  }
}

message GetForecastRequest {
  // city is required because it is singular and not `optional`.
  string city = 1 [(mcp.options.v1.field) = {
    examples: [{ string_value: "Paris" }]
    min_length: 2
  }];
  optional string units = 2 [(mcp.options.v1.field) = {
    default_value: { string_value: "metric" }
  }];
  oneof selector {
    option (mcp.options.v1.oneof) = { required: true };
    string city_alias = 3;
    int64 city_id = 4;
  }
}
```

## Agent Skill

Install the [skills.sh](https://skills.sh/) agent skill to let your AI coding
assistant build MCP servers with protoc-gen-mcp:

```bash
npx skills add easyp-tech/protoc-gen-mcp-skill
```

The skill lives in
[easyp-tech/protoc-gen-mcp-skill](https://github.com/easyp-tech/protoc-gen-mcp-skill).

## Development

```bash
go build ./cmd/protoc-gen-mcp
(cd runtime/typescript && npm ci && npm run build)   # TypeScript runtime, consumed through file:
(cd internal/codegen/testdata/node && npm ci)        # TypeScript test harness
go test ./...
```

JVM tests need JDK 17+ and Gradle 9.2+ and Swift tests a Swift 6.2+
toolchain; both skip otherwise. Fixtures and examples are regenerated with
`easyp` (`easyp.test.yaml`, `easyp.test.swift.yaml` with `protoc-gen-swift` on
`PATH`, `easyp.showcase.yaml`,
`examples/Makefile`); [AGENTS.md](AGENTS.md) lists every command and the
repository layout. Releases publish the `protoc-gen-mcp` binary through
[GoReleaser](.goreleaser.yaml); downstream projects compile generated sources
against their language SDKs.

## License

Apache License 2.0. See [LICENSE](LICENSE).
