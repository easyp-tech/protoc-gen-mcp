# protoc-gen-mcp

<p align="center">
  <a href="https://github.com/easyp-tech/protoc-gen-mcp/actions/workflows/tests.yml"><img alt="CI" src="https://github.com/easyp-tech/protoc-gen-mcp/actions/workflows/tests.yml/badge.svg"></a>
  <a href="https://coveralls.io/github/easyp-tech/protoc-gen-mcp?branch=master"><img alt="Coverage Status" src="https://coveralls.io/repos/github/easyp-tech/protoc-gen-mcp/badge.svg?branch=master"></a>
  <a href="https://pkg.go.dev/github.com/easyp-tech/protoc-gen-mcp"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/easyp-tech/protoc-gen-mcp.svg"></a>
  <a href="https://github.com/easyp-tech/protoc-gen-mcp/releases/latest"><img alt="Release" src="https://img.shields.io/github/v/release/easyp-tech/protoc-gen-mcp?sort=semver"></a>
  <a href="https://github.com/easyp-tech/protoc-gen-mcp/blob/master/LICENSE"><img alt="License" src="https://img.shields.io/github/license/easyp-tech/protoc-gen-mcp"></a>
  <a href="https://github.com/easyp-tech/protoc-gen-mcp/blob/master/go.mod"><img alt="Go version" src="https://img.shields.io/github/go-mod/go-version/easyp-tech/protoc-gen-mcp?logo=go&logoColor=white"></a>
</p>


`protoc-gen-mcp` generates Go, Python, Kotlin, Java, and TypeScript MCP

`protoc-gen-mcp` generates Go, Python, Kotlin, Java, and TypeScript MCP
bindings from protobuf definitions — covering **tools**, **prompts**, and
**resources**. Protobuf is the source of truth: RPCs become tools, annotated
messages become prompts and resources, and the generated code validates
ProtoJSON payloads against generated JSON Schema before calling your typed
handlers. JavaScript projects consume the compiled TypeScript output.

## Supported MCP Primitives

| Primitive | Go | Python | Kotlin | Java | TypeScript |
|-----------|:--:|:------:|:------:|:----:|:----------:|
| **Tools** | ✅ Full | ✅ Full | ✅ Full | ✅ Full | ✅ Full |
| **Prompts** | ✅ Full | ✅ Full | ✅ Full | ✅ Full | ✅ Full |
| **Resources** | ✅ Full | ⚠️ Interface | ⚠️ Interface | ⚠️ Interface | ⚠️ Interface |
| **Protobuf server factory** | ✅ | 📋 Planned | 📋 Planned | 📋 Planned | 📋 Planned |
| **MCP Apps UI / CSP** | ✅ | 📋 Planned | 📋 Planned | 📋 Planned | 📋 Planned |
| **Markdown, HTML, binary resources** | ✅ | 📋 Planned | 📋 Planned | 📋 Planned | 📋 Planned |
| **OAuth/JWKS and tool scopes** | ✅ | 📋 Planned | 📋 Planned | 📋 Planned | 📋 Planned |

"Interface" means that a target emits handler declarations but does **not**
yet implement full resource registration and reading.

Each target uses the official MCP SDK of its language: the Go MCP SDK,
the Python `mcp` SDK with `google.protobuf`, `io.modelcontextprotocol.sdk:mcp`
for Java, `io.modelcontextprotocol:kotlin-sdk-server` for Kotlin, and
`@modelcontextprotocol/sdk` with Protobuf-ES and Ajv for TypeScript.

## Quickstart: run the full Go showcase

The [protobuf-first showcase](examples/11_protobuf_first_mcp/README.md)
runs one server with generated tools, prompts, JSON/text/blob resources,
embedded `SKILL.md`, an MCP Apps UI resource, CSP/permissions, and OAuth
resource-server configuration. With **Go 1.26+**, from the repository root:

```bash
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
    - name: go
      out: .
      opts:
        paths: source_relative
    - command: ["go", "run", "github.com/easyp-tech/protoc-gen-mcp/cmd/protoc-gen-mcp@latest"]
      out: .
      opts:
        paths: source_relative
    - name: python
      with_imports: true
      out: .
    - command: ["go", "run", "github.com/easyp-tech/protoc-gen-mcp/cmd/protoc-gen-mcp@latest"]
      out: .
      opts:
        lang: python
        # python_handler: protobuf            # raw *_pb2 handlers
        # python_handler: dataclass+protobuf  # both, raw ones in *_mcp_pb.py
    - command: ["go", "run", "github.com/easyp-tech/protoc-gen-mcp/cmd/protoc-gen-mcp@latest"]
      out: .
      opts:
        lang: java # or kotlin
    - command: ["./node_modules/.bin/protoc-gen-es"]
      with_imports: true
      out: src/generated
      opts:
        target: ts
        import_extension: js
    - command: ["go", "run", "github.com/easyp-tech/protoc-gen-mcp/cmd/protoc-gen-mcp@latest"]
      out: src/generated
      opts:
        lang: typescript
```

```bash
easyp --cfg easyp.yaml mod download
easyp --cfg easyp.yaml generate -p proto -r .
```

Pin a release tag instead of `@latest` for reproducible builds. Protos used only
for Python, JVM or TypeScript do not need a `go_package` option; the plugin
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
  protos import other packages.
- **Java** (`lang=java`) generates one `<ProtoFile>Mcp` sidecar per proto next
  to protobuf Java output. **Kotlin** (`lang=kotlin`) needs Java protobuf
  output, Kotlin protobuf output, and the MCP sidecar together.
- **TypeScript** (`lang=typescript`) expects Protobuf-ES `_pb.ts` files in the
  same tree, generated with `import_extension=js` for Node ESM and
  `moduleResolution: NodeNext`. **JavaScript** projects import the compiled
  `.js` output and `.d.ts` declarations; there is no `lang=javascript`.

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

Prompts and resources are declared on messages, so their Go registration is
file-scoped: `Register<File>Prompts(server, impl, opts...)` and
`Register<File>Resources(ctx, server, impl, opts...)`, where `<File>` is the
CamelCase proto basename. A static resource (`uri`) needs `Read<Msg>(ctx)`; a
template resource (`uri_template`) needs `List<Msg>s(ctx)` and
`Read<Msg>(ctx, params...)` with percent-decoded template values.

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
Kotlin and TypeScript.

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

- **Service** `option (mcp.options.v1.service)`: `namespace`, `description`,
  default `icons`.
- **Method** `option (mcp.options.v1.method)`: `name`, `title`, `description`,
  `hidden`, `annotations` (`read_only_hint`, `destructive_hint`,
  `idempotent_hint`, `open_world_hint`), `icons`, `execution.task_support`.
  Omitted hints stay omitted; some clients render their own defaults, so set
  hints such as `destructive_hint: false` explicitly when it matters.
- **Field** `[(mcp.options.v1.field)]`: `description`, typed `examples`,
  `default_value`, string constraints (`pattern`, `format`, `min_length`,
  `max_length`), number constraints (`minimum`, `maximum`,
  `exclusive_minimum`, `exclusive_maximum`, `multiple_of`), array constraints
  (`min_items`, `max_items`, `unique_items`), `read_only`.
- **Message** `option (mcp.options.v1.message)`: `title`, `description`,
  `examples`. **Enum** / **enum value**: `title`, `description`, `hidden`
  (typically for `_NONE` sentinels). **Oneof**: `description`, `required`.
- **Prompt** `option (mcp.options.v1.prompt)` on a message: `name`, `title`,
  `description`, `icons`. Fields become prompt arguments; `optional` ones are
  not required.
- **Resource** `option (mcp.options.v1.resource)` on a message: exactly one of
  `uri` or `uri_template` (`{param}` placeholders), `name`, `description`,
  `mime_type` (default `application/json`), `annotations` (`audience`,
  `priority`), `icons`, plus `content_field`, `source_file` and `app_ui` for Go.

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
(cd internal/codegen/testdata/node && npm ci)   # TypeScript test harness
go test ./...
```

JVM tests need JDK 17+ and Gradle 9.2+ and skip otherwise. Fixtures and
examples are regenerated with `easyp` (`easyp.test.yaml`, `easyp.showcase.yaml`,
`examples/Makefile`); [AGENTS.md](AGENTS.md) lists every command and the
repository layout. Releases publish the `protoc-gen-mcp` binary through
[GoReleaser](.goreleaser.yaml); downstream projects compile generated sources
against their language SDKs.

## License

Apache License 2.0. See [LICENSE](LICENSE).
