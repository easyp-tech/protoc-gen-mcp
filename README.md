# protoc-gen-mcp

`protoc-gen-mcp` generates Go and Python MCP tool bindings from protobuf services.

## MVP

- protobuf is the source of truth
- generator emits typed Go and Python MCP bindings
- Go runtime uses the official Go MCP SDK
- Python runtime targets the official MCP Python SDK with `google.protobuf`
- generated Python handlers use dataclasses and explicit `oneof` wrapper types
  from `*_mcp.py`; `*_pb2.py` stays an internal runtime dependency
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
freshness check, and `go test ./...`. Releases are implemented in
[release.yml](.github/workflows/release.yml)
and use [`.goreleaser.yaml`](.goreleaser.yaml)
to publish tagged builds of `protoc-gen-mcp`.

## Test MCP Server

The repository also includes a runnable stdio MCP server for manual client
checks:

```bash
go run ./cmd/example-mcp-server
python ./cmd/example-python-mcp-server/main.py
```

It serves the generated tools from `internal/testproto/example/v1` and is used
by the stdio smoke tests in `internal/examplemcp/stdio_test.go`.
The example server currently exposes:

- `example_CreateReport`
- `example_Health`
- `example_DescribeAdvancedShapes`
- `example_DescribeScalarShapes`

## Examples

We provide several standalone, runnable examples demonstrating the power of generated MCP tools, protobuf `options`, validation constraints, and integration with the official Go and Python SDKs. Check out the [examples/](examples/) directory for:
- [1_helloworld](examples/1_helloworld/) - Minimal Quickstart setup.
- [2_weather_api](examples/2_weather_api/) - Read-only queries, validation limits, Oneofs.
- [3_file_manager](examples/3_file_manager/) - Destructive tools and schema-based string parameter constraints.
- [4_crm_system](examples/4_crm_system/) - A full mock system with FieldMask partial updates, custom icons mapping, schemas nested types, and advanced array filters.
- [5_python_standalone](examples/5_python_standalone/) - A Python-only user-style project with its own `pyproject.toml`, `easyp.yaml`, generated bindings, and stdio server.

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
Go/Python projects, `easyp` can drive `protoc-gen-go`, the standard Python
protobuf generator, and `protoc-gen-mcp` from one config.

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
```

Typical commands:

```bash
easyp --cfg easyp.yaml validate-config
easyp --cfg easyp.yaml lint -p mcp -r .
easyp --cfg easyp.yaml generate -p mcp -r .
```

That generates `*.pb.go`, `*_pb2.py`, `*.mcp.go`, `*_mcp.py`, and package
`__init__.py` files next to the source `.proto` files. The Python target
currently supports only `python_runtime=google.protobuf`. Generated Python
output also emits a small `mcp/__init__.py` bridge so `mcp.options.*` protobuf
modules can coexist with the official `mcp` SDK package in one import tree. No
special Easyp override is required for `mcp.options.v1`, because the package
declares `go_package` directly in `options.proto`. For reproducible builds,
prefer pinning a specific tag instead of `@latest`.

For Python-only projects, omit the Go plugins and keep only the standard
`python` plugin plus `protoc-gen-mcp` with `lang=python`. User-authored Python
protos do not need a `go_package` option just to satisfy the generator; the
plugin synthesizes internal Go package metadata before building the descriptor
model. See [examples/5_python_standalone](examples/5_python_standalone/) for a
complete Python-only project with its own virtualenv setup.

When you generate Python handlers, implement against the dataclasses from
`*_mcp.py`, not the raw protobuf classes from `*_pb2.py`. The generated Python
runtime maps MCP JSON -> ProtoJSON -> `pb2` -> dataclasses before calling your
handler, then maps your dataclass response back through protobuf serialization
and output-schema validation.

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

Comments are also used as metadata:

- plain comment lines become descriptions
- `Example: ...` adds one schema example (though `FieldOptions.examples` offers stronger typing).
- `Examples: ... | ...` adds multiple schema examples.

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

## Status

This repository currently implements the MVP only:

- tools only
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
