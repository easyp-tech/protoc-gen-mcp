# Standalone Python MCP Server With Raw Protobuf Handlers

This example is structured like a user-owned Python project and opts into
`python_handler: dataclass+protobuf`. Generation emits both public surfaces:
`proto/tasks_mcp.py` for dataclass handlers and `proto/tasks_mcp_pb.py` for
raw `*_pb2` handlers. The handwritten server uses the raw sidecar with
`tasks_pb2.*` message classes.

Use this layout when you already have Python server code written against
standard `google.protobuf` generated classes and also want dataclass bindings
available. If you only want the default dataclass API with `UNSET` and
explicit `oneof` wrappers, use
[`../5_python_standalone`](../5_python_standalone/) instead.

## Generate

```bash
make generate
```

The key generator option is:

```yaml
lang: python
python_runtime: google.protobuf
python_handler: dataclass+protobuf
```

For protobuf-only projects, `python_handler: protobuf` still writes the raw
handler sidecar to the normal `proto/tasks_mcp.py` path for backward
compatibility. This example uses `python_handler: dataclass+protobuf`, so the
dataclass sidecar uses `proto/tasks_mcp.py` and the raw protobuf sidecar uses
`proto/tasks_mcp_pb.py`.

That generates:

- `proto/tasks_pb2.py`
- `proto/tasks_mcp.py`
- `proto/tasks_mcp_pb.py`
- `proto/__init__.py`
- `mcp/__init__.py`
- `mcp/options/v1/options_pb2.py`

The raw generated MCP sidecar still exposes the normal registration helper:

```python
from proto import tasks_mcp_pb, tasks_pb2


class TaskStore(tasks_mcp_pb.TaskAPIToolHandler):
    def create_task(
        self,
        _ctx: tasks_mcp_pb.ToolRequestContext,
        req: tasks_pb2.CreateTaskRequest,
    ) -> tasks_pb2.CreateTaskResponse:
        ...


tasks_mcp_pb.register_task_api_tools(server, TaskStore())
```

In the protobuf sidecar, generated dataclasses, `UNSET`, and dataclass mapper
helpers are omitted. Runtime validation, ProtoJSON parsing, output validation,
structured content, tool names, annotations, icons, and execution metadata are
still owned by the generated MCP sidecar. The dataclass sidecar remains
available in `proto/tasks_mcp.py` for users who prefer wrapper types.

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
