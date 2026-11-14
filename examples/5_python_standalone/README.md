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
- `notebook_DeleteNote` (needs `notebook:write`)
- `notebook_Health`

Handlers implement generated dataclasses from `proto/notebook_mcp.py`. Raw
`*_pb2.py` classes stay internal to the generated runtime.

## OAuth Over HTTP

`option (mcp.options.v1.server)` in the proto makes the server an OAuth
protected resource over stateless Streamable HTTP: every request needs
`notebook:read`, and `notebook_DeleteNote` also needs `notebook:write`. Over stdio there is no
token, so `notebook_DeleteNote` is denied inside the call. Start the server with local
demonstration tokens (loopback only, never in production):

```bash
.venv/bin/python server.py --http --demo-auth
```

Then walk through discovery and the scope step-up:

```bash
curl -s http://127.0.0.1:8080/.well-known/oauth-protected-resource/mcp
for token in demo-read demo-write; do
  curl -si http://127.0.0.1:8080/mcp \
    -H "Authorization: Bearer $token" \
    -H 'Content-Type: application/json' \
    -H 'Accept: application/json, text/event-stream' \
    -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"notebook_DeleteNote","arguments":{"id":"note-1"}}}'
done
```

`demo-read` gets `403` with `WWW-Authenticate: Bearer error="insufficient_scope",
scope="notebook:read notebook:write"` before the handler runs; `demo-write` gets the
tool result. Without `--demo-auth` the server verifies RS256 JWTs against the
issuer and JWKS declared in the proto.

## Handler Mode

This example uses the default dataclass mode. Keep this mode when you want
handler-facing `UNSET`, explicit `oneof` wrappers, and generated mapper helpers
from `*_mcp.py`. If an existing server already works directly with raw `*_pb2`
classes, set `python_handler: protobuf` (handlers take `*_pb2` messages) or
`python_handler: dataclass+protobuf` (both surfaces, the raw one in
`*_mcp_pb.py`) in `easyp.yaml` instead.

If you inspect this server in `@modelcontextprotocol/inspector`, note that the
server forwards tool annotations exactly as declared in `proto/notebook.proto`.
Inspector may render omitted hints like `destructiveHint` using its own
defaults, so set those hints explicitly in the proto if you want the UI badges
to be unambiguous.
