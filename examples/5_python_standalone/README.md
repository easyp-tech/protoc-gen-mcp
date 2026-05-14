# Standalone Python MCP Server

This example is structured like a user-owned Python project rather than a
repository fixture. It has its own `easyp.yaml`, `pyproject.toml`, virtualenv
target, protobuf contract, generated Python bindings, and stdio server.

The generated code is imported as a normal local package:

```python
from proto import notebook_mcp
```

There is no `sys.path` bootstrap in `server.py`. Run commands from this
directory so Python resolves the local `proto/` package and the generated
`mcp/` namespace bridge.

## Generate

```bash
make generate
```

`easyp` downloads `mcp/options/v1/options.proto` through the `deps` section and
generates:

- `proto/notebook_pb2.py`
- `proto/notebook_mcp.py`
- `proto/__init__.py`
- `mcp/__init__.py`
- `mcp/options/v1/options_pb2.py`

The checked-in `easyp.yaml` runs the local plugin from this repository so the
example always exercises the current source tree. In an external project,
replace that command with a released version such as:

```yaml
command: ["go", "run", "github.com/easyp-tech/protoc-gen-mcp/cmd/protoc-gen-mcp@latest"]
```

## Run

```bash
make setup
make run
```

The server exposes:

- `notebook_CreateNote`
- `notebook_SearchNotes`
- `notebook_Health`

Handlers implement generated dataclasses from `proto/notebook_mcp.py`. Raw
`*_pb2.py` classes stay internal to the generated runtime.

## Handler Mode

This example uses the default dataclass mode. Keep this mode when you want
handler-facing `UNSET`, explicit `oneof` wrappers, and generated mapper helpers
from `*_mcp.py`. If an existing server already works directly with raw `*_pb2`
classes, use
[`../10_python_protobuf_standalone`](../10_python_protobuf_standalone/) and
set `python_handler: protobuf` instead.

If you inspect this server in `@modelcontextprotocol/inspector`, note that the
server forwards tool annotations exactly as declared in `proto/notebook.proto`.
Inspector may render omitted hints like `destructiveHint` using its own
defaults, so set those hints explicitly in the proto if you want the UI badges
to be unambiguous.
