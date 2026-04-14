from __future__ import annotations

from pathlib import Path
import sys
import tempfile

_EXAMPLES_ROOT = Path(__file__).resolve().parents[1]
if str(_EXAMPLES_ROOT) not in sys.path:
    sys.path.insert(0, str(_EXAMPLES_ROOT))

import anyio
import mcp.server.lowlevel
import mcp.server.stdio

from proto import filemanager_mcp


class FileManagerAPI:
    def __init__(self, base_path: Path) -> None:
        self.base_path = base_path
        self.base_path.mkdir(parents=True, exist_ok=True)
        (self.base_path / "example.txt").write_text(
            "Hello from the Python file manager example.\n",
            encoding="utf-8",
        )

    def read_file(
        self,
        _ctx: filemanager_mcp.ToolRequestContext,
        req: filemanager_mcp.ReadFileRequest,
    ) -> filemanager_mcp.ReadFileResponse:
        file_path = self._resolve(req.filename)
        try:
            content = file_path.read_text(encoding="utf-8")
        except FileNotFoundError as err:
            raise ValueError("file not found") from err
        return filemanager_mcp.ReadFileResponse(content=content)

    def delete_file(
        self,
        _ctx: filemanager_mcp.ToolRequestContext,
        req: filemanager_mcp.DeleteFileRequest,
    ) -> filemanager_mcp.DeleteFileResponse:
        file_path = self._resolve(req.filename)
        try:
            file_path.unlink()
        except FileNotFoundError as err:
            raise ValueError("file not found") from err
        return filemanager_mcp.DeleteFileResponse(success=True)

    def _resolve(self, filename: str) -> Path:
        candidate = Path(filename)
        if candidate.name != filename:
            raise ValueError("filename must not contain paths or directories")
        return self.base_path / candidate


def new_server() -> mcp.server.lowlevel.Server:
    base_path = Path(tempfile.gettempdir()) / "protoc-gen-mcp-python-example-files"
    server = mcp.server.lowlevel.Server("filemanager-mcp-server", version="1.0.0")
    filemanager_mcp.register_file_manager_api_tools(server, FileManagerAPI(base_path))
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
