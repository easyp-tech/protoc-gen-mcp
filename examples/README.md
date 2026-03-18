# protoc-gen-mcp Examples

This directory contains standalone, runnable examples of how to build and expose Model Context Protocol (MCP) tools using Protocol Buffers and the Go SDK.

## Prerequisites
- **easyp**: Ensure you have [easyp](https://github.com/easyp-tech/easyp) installed for code generation.

## Running the Examples
1. Generate the Go code from the `.proto` definitions. Run this from the root of the repository or from the examples directory:
   ```bash
   cd examples
   make generate
   ```
2. Run any of the servers (e.g. `1-helloworld`):
   ```bash
   cd examples/1-helloworld
   go run main.go
   ```

## Included Examples

### [1-helloworld](./1-helloworld)
A minimal Quickstart example. It contains a single RPC method that takes a name and returns a greeting. Demonstrates the simplest way to configure `easyp.yaml` and initialize `mcp.Server` with the generated bindings.

### [2-weather-api](./2-weather-api) (Validation & Safe Read-Only Queries)
A mock weather service demonstrating input safety limits and schema hints.
- **Validation**: Strict limits on parameters (e.g. `minimum`, `maximum` degrees).
- **Oneof**: Allows lookup either by `city` or by `coordinates`.
- **Annotations**: Sets `read_only_hint: true` and `open_world_hint: true` so the LLM knows it is harmless to aggressively scan locations without altering state.

### [3-file-manager](./3-file-manager) (Destructive Operations)
A straightforward demo for operating on a temporary directory.
- **Security Check**: Strings validated by `pattern: "^[a-zA-Z0-9_\\-\\.]+$"` to guarantee LLMs don't trigger path injections.
- **Destructive Operations**: Explicitly flags the `DeleteFile` tool with `destructive_hint: true` so MCP hosts demand user confirmation before dropping files.

### [4-crm-system](./4-crm-system) (Enterprise Kitchen Sink)
A full-featured (mocked) customer relationship management service covering complex configurations.
- **Icons**: Distinct tool and service icons encoded as JSON.
- **Well-Known Types (WKTs)**: `google.protobuf.Timestamp` emitted into schemas and correctly initialized, and `google.protobuf.FieldMask` for targeted partial updates.
- **Advanced Constraints**: Ensures slice entries hold unique, non-empty values (`min_items: 1`, `unique_items: true`).
- **Rich Output**: Complex responses carrying arrays of message objects back to the LLM context.
