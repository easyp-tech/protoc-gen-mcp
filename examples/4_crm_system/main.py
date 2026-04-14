from __future__ import annotations

from datetime import datetime, timedelta, timezone
from pathlib import Path
import sys
import threading

_EXAMPLES_ROOT = Path(__file__).resolve().parents[1]
if str(_EXAMPLES_ROOT) not in sys.path:
    sys.path.insert(0, str(_EXAMPLES_ROOT))

import anyio
import mcp.server.lowlevel
import mcp.server.stdio

from proto import crm_mcp


def _timestamp(value: datetime) -> crm_mcp.Timestamp:
    return (
        value.astimezone(timezone.utc)
        .replace(microsecond=0)
        .isoformat()
        .replace("+00:00", "Z")
    )


def _clone_user(user: crm_mcp.User) -> crm_mcp.User:
    return crm_mcp.User(
        id=user.id,
        name=user.name,
        registered_at=user.registered_at,
        tags=list(user.tags),
    )


class CRMAPI:
    def __init__(self) -> None:
        now = datetime.now(timezone.utc)
        self._mu = threading.RLock()
        self._users = {
            "usr_1": crm_mcp.User(
                id="usr_1",
                name="Alice Smith",
                tags=["premium", "enterprise"],
                registered_at=_timestamp(now - timedelta(hours=24)),
            ),
            "usr_2": crm_mcp.User(
                id="usr_2",
                name="Bob Jones",
                tags=["basic"],
                registered_at=_timestamp(now - timedelta(hours=2)),
            ),
        }

    def list_users(
        self,
        _ctx: crm_mcp.ToolRequestContext,
        req: crm_mcp.ListUsersRequest,
    ) -> crm_mcp.ListUsersResponse:
        with self._mu:
            result = [
                _clone_user(user)
                for user in self._users.values()
                if all(tag in user.tags for tag in req.required_tags)
            ]

        limit = req.limit or 10
        return crm_mcp.ListUsersResponse(users=result[:limit])

    def update_user(
        self,
        _ctx: crm_mcp.ToolRequestContext,
        req: crm_mcp.UpdateUserRequest,
    ) -> crm_mcp.UpdateUserResponse:
        if req.user.id == "":
            raise ValueError("user and user.id must be provided")

        with self._mu:
            existing = self._users.get(req.user.id)
            if existing is None:
                raise ValueError(f'user "{req.user.id}" not found')

            paths = [path for path in req.update_mask.split(",") if path]
            if not paths:
                existing.name = req.user.name
                existing.tags = list(req.user.tags)
            else:
                for path in paths:
                    if path == "name":
                        existing.name = req.user.name
                    elif path == "tags":
                        existing.tags = list(req.user.tags)

            return crm_mcp.UpdateUserResponse(user=_clone_user(existing))


def new_server() -> mcp.server.lowlevel.Server:
    server = mcp.server.lowlevel.Server("crm-mcp-server", version="1.0.0")
    crm_mcp.register_users_api_tools(server, CRMAPI())
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
