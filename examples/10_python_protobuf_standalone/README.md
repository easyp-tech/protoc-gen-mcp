# Standalone Python MCP Server With Raw Protobuf Handlers

This example is structured like a user-owned Python project and opts into
`python_handler: protobuf`. Generated handlers receive and return raw
`*_pb2` message classes, represented here by `tasks_pb2.*`, instead of
generated dataclasses.

Use this layout when you already have Python server code written against
standard `google.protobuf` generated classes. If you want the default
dataclass API with `UNSET` and explicit `oneof` wrappers, use
[`../5_python_standalone`](../5_python_standalone/) instead.

## Generate

```bash
make generate
```

The key generator option is:

```yaml
lang: python
python_runtime: google.protobuf
python_handler: protobuf
```

`python_handler: protobuf` is protobuf-only and writes the raw handler sidecar
to `proto/tasks_mcp.py`. If a project wants both public surfaces in one
generation pass, use `python_handler: dataclass+protobuf`; then
`proto/tasks_mcp.py` remains the dataclass sidecar and `proto/tasks_mcp_pb.py`
contains the raw `tasks_pb2.*` handler sidecar.

That generates:

- `proto/tasks_pb2.py`
- `proto/tasks_mcp.py`
- `proto/__init__.py`
- `mcp/__init__.py`
- `mcp/options/v1/options_pb2.py`

The generated MCP sidecar still exposes the normal registration helper:

```python
from proto import tasks_mcp, tasks_pb2


class TaskStore(tasks_mcp.TaskAPIToolHandler):
    def create_task(
        self,
        _ctx: tasks_mcp.ToolRequestContext,
        req: tasks_pb2.CreateTaskRequest,
    ) -> tasks_pb2.CreateTaskResponse:
        ...


tasks_mcp.register_task_api_tools(server, TaskStore())
```

In protobuf mode, generated dataclasses, `UNSET`, and dataclass mapper helpers
are omitted. Runtime validation, ProtoJSON parsing, output validation,
structured content, tool names, annotations, icons, and execution metadata are
still owned by the generated MCP sidecar.

## Run

```bash
make setup
make run
```

The server exposes:

- `tasks_CreateTask`
- `tasks_ListTasks`
- `tasks_Health`

The checked-in `easyp.yaml` runs the local plugin from this repository so the
example always exercises the current source tree. In an external project,
replace that command with a released version such as:

```yaml
command: ["go", "run", "github.com/easyp-tech/protoc-gen-mcp/cmd/protoc-gen-mcp@latest"]
```
