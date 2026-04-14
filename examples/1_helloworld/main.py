from __future__ import annotations

from pathlib import Path
import sys

_EXAMPLES_ROOT = Path(__file__).resolve().parents[1]
if str(_EXAMPLES_ROOT) not in sys.path:
    sys.path.insert(0, str(_EXAMPLES_ROOT))

import anyio
import mcp.server.lowlevel
import mcp.server.stdio

from proto import helloworld_mcp


class Greeter:
    def say_hello(
        self,
        _ctx: helloworld_mcp.ToolRequestContext,
        req: helloworld_mcp.SayHelloRequest,
    ) -> helloworld_mcp.SayHelloResponse:
        return helloworld_mcp.SayHelloResponse(message=f"Hello, {req.name}!")


def new_server() -> mcp.server.lowlevel.Server:
    server = mcp.server.lowlevel.Server("helloworld-mcp-server", version="1.0.0")
    helloworld_mcp.register_greeter_api_tools(server, Greeter())
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
