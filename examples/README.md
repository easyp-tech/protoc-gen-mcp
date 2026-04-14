# protoc-gen-mcp Examples

This directory contains standalone, runnable examples of how to build and expose Model Context Protocol (MCP) tools using Protocol Buffers with both the Go SDK and the official Python SDK.

## Prerequisites
- **easyp**: Ensure you have [easyp](https://github.com/easyp-tech/easyp) installed for code generation.

## Running the Examples
1. Generate the Go and Python code from the `.proto` definitions. Run this from the root of the repository or from the examples directory:
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
   `5_python_standalone` has its own `easyp.yaml`, `easyp.lock`, and
   `pyproject.toml` to model a user-owned Python project.
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
