# protoc-gen-mcp

`protoc-gen-mcp` generates Go MCP tool bindings from protobuf services.

## MVP

- protobuf is the source of truth
- generator emits typed Go MCP bindings
- runtime uses the official Go MCP SDK
- request and response JSON follows ProtoJSON rules
- runtime validation is driven by generated JSON Schema

## Repository Workflows

Use `easyp` for repository generation and checks.
The repository is currently aligned to `easyp v0.15.2-rc1`.

```bash
easyp --cfg easyp.yaml lint -p api -r .
easyp --cfg easyp.yaml generate -p api -r .
easyp --cfg easyp.test.yaml lint -p internal/testproto -r .
easyp --cfg easyp.test.yaml generate -p internal/testproto -r .
go test ./...
goreleaser check
```

`easyp.yaml` is the main config for shipped protobuf APIs. `easyp.test.yaml`
is the development and test config for repository fixtures.

`api/mcp/options/v1/options.proto` carries its own explicit `go_package`, so
consumers do not need a special Easyp `go_package_prefix` override just to use
the MCP options package.

CI is implemented in [tests.yml](/Users/khasbulatabdullin/DISK/code/languages/Go/protoc-gen-mcp/.github/workflows/tests.yml)
and runs config validation, Easyp lint, Easyp generation, a generated-file
freshness check, and `go test ./...`. Releases are implemented in
[release.yml](/Users/khasbulatabdullin/DISK/code/languages/Go/protoc-gen-mcp/.github/workflows/release.yml)
and use [`.goreleaser.yaml`](/Users/khasbulatabdullin/DISK/code/languages/Go/protoc-gen-mcp/.goreleaser.yaml)
to publish tagged builds of `protoc-gen-mcp-go`.

## Test MCP Server

The repository also includes a runnable stdio MCP server for manual client
checks:

```bash
go run github.com/easyp-tech/protoc-gen-mcp/cmd/example-mcp-server@latest
```

It serves the generated tools from `internal/testproto/example/v1` and is used
by the stdio smoke test in `internal/examplemcp/stdio_test.go`.
The example server currently exposes:

- `example_CreateReport`
- `example_Health`
- `example_DescribeAdvancedShapes`
- `example_DescribeScalarShapes`

## Generation With Easyp

The intended workflow is `easyp`, not manual `protoc` invocation. `easyp`
drives both `protoc-gen-go` and `protoc-gen-mcp-go` with the same repository
config.

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
        path: api
        root: "."
  plugins:
    - name: go
      out: .
      opts:
        paths: source_relative
    - command: ["go", "run", "github.com/easyp-tech/protoc-gen-mcp/cmd/protoc-gen-mcp-go@latest"]
      out: .
      opts:
        paths: source_relative
```

Typical commands:

```bash
easyp --cfg easyp.yaml validate-config
easyp --cfg easyp.yaml lint -p api -r .
easyp --cfg easyp.yaml generate -p api -r .
```

That generates both `*.pb.go` and `*.mcp.go` next to the source `.proto` files.
No special Easyp override is required for `api.mcp.options.v1`, because the
package declares `go_package` directly in `options.proto`. For reproducible
builds, prefer pinning a specific tag instead of `@latest`.

Generated tool names never contain dots. The runtime joins the optional service
namespace and RPC tool name with underscores, so a service namespace
`weather.v1` and RPC `GetForecast` become tool name `weather_v1_GetForecast`.

Generated JSON Schemas use a tool-first requiredness policy: a singular field
is marked as required by default unless it is `proto3 optional`, `repeated`,
`map`, `oneof`, or explicitly relaxed through MCP field options. Fields that
are not required by that generated MCP schema accept explicit JSON `null`, so
MCP clients that pre-validate cached `inputSchema` do not reject otherwise
valid tool calls before they reach the server.
Generated schemas also emit examples for complex ProtoJSON forms. Explicit
field/message examples are parsed as JSON literals when possible, and the
generator synthesizes fallback examples for maps, recursive messages, `Any`,
special float encodings, and other advanced shapes to make agent-side tool
invocation more discoverable.

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

Requiredness is a generated MCP schema policy, not protobuf `required`.
Singular fields are required by default unless they are `proto3 optional`,
`repeated`, `map`, `oneof`, or explicitly relaxed through MCP field options.
Fields that are not schema-required accept explicit JSON `null`.

## Complete Proto Example

This example shows the current supported surface, including service, method,
and field options, all plain scalar families, maps, `oneof`, recursive
messages, ProtoJSON-special forms, selected well-known types, wrappers, and
hidden or disabled RPCs.

```proto
syntax = "proto3";

package api.weather.v1;

option go_package = "github.com/acme/weather-mcp/api/weather/v1;weatherv1";

import "api/mcp/options/v1/options.proto";
import "google/protobuf/any.proto";
import "google/protobuf/duration.proto";
import "google/protobuf/empty.proto";
import "google/protobuf/field_mask.proto";
import "google/protobuf/struct.proto";
import "google/protobuf/timestamp.proto";
import "google/protobuf/wrappers.proto";

service WeatherAPI {
  option (api.mcp.options.v1.service) = {
    namespace: "weather"
    description: "Weather tools exported as MCP tools."
  };

  // Forecast returns a weather report.
  // Example: {"city":"Paris","units":"metric","labels":{"env":"prod"}}
  rpc Forecast(GetForecastRequest) returns (GetForecastResponse) {
    option (api.mcp.options.v1.method) = {
      name: "GetForecast"
      title: "Get forecast"
      description: "Fetch the forecast for a city."
      examples: "{\"city\":\"Paris\",\"units\":\"metric\",\"labels\":{\"env\":\"prod\"}}"
    };
  }

  // Health returns an empty acknowledgement.
  rpc Health(google.protobuf.Empty) returns (HealthResponse) {
    option (api.mcp.options.v1.method) = {
      title: "Health check"
      description: "Verify that the MCP server is alive."
    };
  }

  // InternalDebug is omitted from generated tools.
  rpc InternalDebug(InternalDebugRequest) returns (InternalDebugResponse) {
    option (api.mcp.options.v1.method) = {
      hidden: true
    };
  }

  // DeprecatedProbe is also omitted from generated tools.
  rpc DeprecatedProbe(DeprecatedProbeRequest) returns (DeprecatedProbeResponse) {
    option (api.mcp.options.v1.method) = {
      disabled: true
    };
  }
}

message GetForecastRequest {
  // city is the primary lookup key.
  string city = 1 [(api.mcp.options.v1.field) = {
    description: "City name accepted by the upstream weather provider."
    examples: "\"Paris\""
  }];

  // units uses real proto3 optional presence.
  optional string units = 2;

  // tags stays optional in generated MCP schema because it is repeated.
  repeated string tags = 3;

  // labels demonstrates map<string, string>.
  map<string, string> labels = 4;

  // buckets demonstrates numeric map keys encoded as JSON object keys.
  map<int32, string> buckets = 5;

  // toggles demonstrates bool map keys encoded as JSON object keys.
  map<bool, string> toggles = 6;

  // limits demonstrates uint64 map keys encoded as JSON strings.
  map<uint64, string> limits = 7;

  // include_alerts is relaxed even though it is a singular scalar.
  bool include_alerts = 8 [(api.mcp.options.v1.field).optional = true];

  // trace_token demonstrates bytes -> base64.
  bytes trace_token = 9;

  int32 page = 10;
  sint32 signed_offset = 11;
  sfixed32 fixed_window = 12;
  uint32 region_code = 13;
  fixed32 fixed_region_code = 14;
  int64 since_epoch = 15;
  sint64 signed_epoch_delta = 16;
  sfixed64 fixed_delta = 17;
  uint64 request_id = 18;
  fixed64 fixed_request_id = 19;
  float temperature_floor = 20;
  double confidence = 21;
  ForecastMode mode = 22;

  // details is forced required explicitly.
  ForecastDetails details = 23 [(api.mcp.options.v1.field).required = true];

  google.protobuf.Timestamp observed_at = 24 [(api.mcp.options.v1.field).optional = true];
  google.protobuf.Duration ttl = 25 [(api.mcp.options.v1.field).optional = true];
  google.protobuf.FieldMask mask = 26 [(api.mcp.options.v1.field).optional = true];
  google.protobuf.Struct filters = 27 [(api.mcp.options.v1.field).optional = true];
  google.protobuf.ListValue items = 28 [(api.mcp.options.v1.field).optional = true];
  google.protobuf.Value dynamic = 29 [(api.mcp.options.v1.field).optional = true];
  google.protobuf.BoolValue enabled = 30 [(api.mcp.options.v1.field).optional = true];
  google.protobuf.StringValue note = 31 [(api.mcp.options.v1.field).optional = true];
  google.protobuf.BytesValue blob = 32 [(api.mcp.options.v1.field).optional = true];
  google.protobuf.Int32Value small_total = 33 [(api.mcp.options.v1.field).optional = true];
  google.protobuf.UInt32Value uint_total = 34 [(api.mcp.options.v1.field).optional = true];
  google.protobuf.Int64Value total = 35 [(api.mcp.options.v1.field).optional = true];
  google.protobuf.UInt64Value huge_total = 36 [(api.mcp.options.v1.field).optional = true];
  google.protobuf.FloatValue weight = 37 [(api.mcp.options.v1.field).optional = true];
  google.protobuf.DoubleValue special_ratio = 38 [(api.mcp.options.v1.field).optional = true];

  // Example: {"@type":"type.googleapis.com/api.weather.v1.ForecastDetails","label":"from-any"}
  google.protobuf.Any detail_any = 39 [(api.mcp.options.v1.field).optional = true];

  RecursiveNode tree = 40 [(api.mcp.options.v1.field).optional = true];

  oneof selector {
    string city_alias = 41;
    int64 city_id = 42;
    ForecastDetails city_details = 43;
  }

  optional ForecastMode optional_mode = 44;
  optional double optional_ratio = 45;
}

message GetForecastResponse {
  string report_id = 1;
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

message DeprecatedProbeRequest {}

message DeprecatedProbeResponse {}

message ForecastDetails {
  string label = 1;
}

message RecursiveNode {
  string name = 1;
  RecursiveNode child = 2 [(api.mcp.options.v1.field).optional = true];
  repeated RecursiveNode children = 3;
}

enum ForecastMode {
  FORECAST_MODE_NONE = 0;
  FORECAST_MODE_DAILY = 1;
  FORECAST_MODE_HOURLY = 2;
}
```

## Metadata Options

Import `api/mcp/options/v1/options.proto` in your `.proto` files to override
generation behavior.

Service options:

- `namespace`
  Used as the tool-name prefix. `weather` + `GetForecast` becomes
  `weather_GetForecast`.
  Example:
  `option (api.mcp.options.v1.service) = { namespace: "weather" };`
- `description`
  Overrides the service description inferred from comments.
  Example:
  `option (api.mcp.options.v1.service) = { description: "Weather tools exported as MCP tools." };`

Method options:

- `name`
  Overrides the RPC segment of the generated tool name.
  Example:
  `option (api.mcp.options.v1.method) = { name: "GetForecast" };`
- `title`
  Sets the MCP tool title.
  Example:
  `option (api.mcp.options.v1.method) = { title: "Get forecast" };`
- `description`
  Overrides the RPC description inferred from comments.
  Example:
  `option (api.mcp.options.v1.method) = { description: "Fetch the forecast for a city." };`
- `hidden`
  Suppresses tool generation for that RPC.
  Example:
  `option (api.mcp.options.v1.method) = { hidden: true };`
- `disabled`
  Also suppresses tool generation for that RPC. Use this when the method still
  exists in protobuf for compatibility, but should not be exposed as an MCP
  tool.
  Example:
  `option (api.mcp.options.v1.method) = { disabled: true };`
- `examples`
  Adds explicit JSON Schema examples for the generated tool input. These are
  preferred over synthesized examples.
  Example:
  `option (api.mcp.options.v1.method) = { examples: "{\"city\":\"Paris\"}" };`

Field options:

- `required`
  Forces a field to be schema-required even if it would otherwise be optional.
  Useful for `proto3 optional` or WKT fields that your tool contract still
  wants to require.
  Example:
  `optional string units = 2 [(api.mcp.options.v1.field).required = true];`
- `optional`
  Forces a field to be schema-optional even if it is a singular non-`optional`
  protobuf field.
  Example:
  `bool include_alerts = 8 [(api.mcp.options.v1.field).optional = true];`
- `description`
  Overrides the field description inferred from comments.
  Example:
  `string city = 1 [(api.mcp.options.v1.field).description = "City name accepted by the upstream weather provider."];`
- `examples`
  Adds explicit JSON Schema examples for that field. If the example is valid
  JSON, it is emitted into the schema as a JSON literal instead of a plain
  string.
  Example:
  `google.protobuf.Any detail_any = 39 [(api.mcp.options.v1.field).examples = "{\"@type\":\"type.googleapis.com/api.weather.v1.ForecastDetails\",\"label\":\"from-any\"}"];`

Comments are also used as metadata:

- plain comment lines become descriptions
- `Example: ...` adds one schema example
- `Examples: ... | ...` adds multiple schema examples

## Generated API And Server Integration

After `easyp --cfg easyp.yaml generate -p api -r .`, the generated package
exposes a typed handler interface plus a registration helper:

```go
type WeatherAPIToolHandler interface {
	Forecast(ctx context.Context, req *GetForecastRequest) (*GetForecastResponse, error)
	Health(ctx context.Context, req *emptypb.Empty) (*HealthResponse, error)
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

	weatherv1 "github.com/acme/weather/gen/api/weather/v1"
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
- `DeprecatedProbe` is omitted because `disabled = true`

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
- generated MCP schema requiredness is tool-first: singular non-`optional`
  fields are required by default unless they are `repeated`, `map`, `oneof`,
  or explicitly relaxed through MCP field options
- fields that are not required by that generated MCP schema accept explicit
  JSON `null` to match ProtoJSON parser behavior for unset values
- unsupported and required to fail fast:
  non-unary protobuf RPC methods and unsupported `google.protobuf` message
  types
