from __future__ import annotations

from datetime import datetime, timezone

import anyio
import mcp.server.lowlevel
import mcp.server.stdio
from google.protobuf import timestamp_pb2

from proto import tasks_mcp_pb, tasks_pb2


def _now_timestamp() -> timestamp_pb2.Timestamp:
    timestamp = timestamp_pb2.Timestamp()
    timestamp.FromDatetime(datetime.now(timezone.utc))
    return timestamp


class TaskStore(tasks_mcp_pb.TaskAPIToolHandler):
    def __init__(self) -> None:
        self._next_id = 1
        self._tasks: dict[str, tasks_pb2.Task] = {}

    def create_task(
        self,
        _ctx: tasks_mcp_pb.ToolRequestContext,
        req: tasks_pb2.CreateTaskRequest,
    ) -> tasks_pb2.CreateTaskResponse:
        task_id = f"task-{self._next_id}"
        self._next_id += 1

        task = tasks_pb2.Task(
            id=task_id,
            title=req.title,
            completed=False,
            tags=list(req.tags),
        )
        task.created_at.CopyFrom(_now_timestamp())
        self._tasks[task.id] = task
        return tasks_pb2.CreateTaskResponse(task=task)

    def list_tasks(
        self,
        _ctx: tasks_mcp_pb.ToolRequestContext,
        req: tasks_pb2.ListTasksRequest,
    ) -> tasks_pb2.ListTasksResponse:
        include_completed = req.include_completed if req.HasField("include_completed") else True
        limit = req.limit if req.HasField("limit") else 10
        required_tags = set(req.tags)

        matches: list[tasks_pb2.Task] = []
        for task in self._tasks.values():
            if not include_completed and task.completed:
                continue
            if required_tags and not required_tags.issubset(set(task.tags)):
                continue
            matches.append(task)
            if len(matches) >= limit:
                break

        return tasks_pb2.ListTasksResponse(tasks=matches)

    def health(
        self,
        _ctx: tasks_mcp_pb.ToolRequestContext,
        _req: tasks_pb2.HealthRequest,
    ) -> tasks_pb2.HealthResponse:
        return tasks_pb2.HealthResponse(ok=True, task_count=len(self._tasks))


def new_server() -> mcp.server.lowlevel.Server:
    server = mcp.server.lowlevel.Server("standalone-python-protobuf-tasks", version="0.1.0")
    tasks_mcp_pb.register_task_api_tools(server, TaskStore())
    return server


async def run_stdio_server() -> None:
    server = new_server()
    async with mcp.server.stdio.stdio_server() as (read_stream, write_stream):
        await server.run(
            read_stream,
            write_stream,
            server.create_initialization_options(),
        )


def main() -> None:
    anyio.run(run_stdio_server)


if __name__ == "__main__":
    main()
