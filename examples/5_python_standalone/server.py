from __future__ import annotations

import argparse
import socket
import time
from datetime import datetime, timezone

import anyio
import mcp.server.lowlevel
import mcp.server.stdio
import mcp.shared.exceptions
import mcp.types
import uvicorn
from mcp.server.auth.provider import AccessToken

from proto import notebook_mcp


def _now_protojson() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


class Notebook:
    def __init__(self) -> None:
        self._next_id = 1
        self._notes: dict[str, notebook_mcp.Note] = {}

    def create_note(
        self,
        _ctx: notebook_mcp.ToolRequestContext,
        req: notebook_mcp.CreateNoteRequest,
    ) -> notebook_mcp.CreateNoteResponse:
        note_id = f"note-{self._next_id}"
        self._next_id += 1

        note = notebook_mcp.Note(
            id=note_id,
            title=req.title,
            body=req.body,
            tags=list(req.tags),
            due_date=req.due_date,
            created_at=_now_protojson(),
        )
        self._notes[note.id] = note
        return notebook_mcp.CreateNoteResponse(note=note)

    def search_notes(
        self,
        _ctx: notebook_mcp.ToolRequestContext,
        req: notebook_mcp.SearchNotesRequest,
    ) -> notebook_mcp.SearchNotesResponse:
        query = "" if req.query is notebook_mcp.UNSET else req.query.casefold()
        required_tags = set(req.tags)
        limit = 10 if req.limit is notebook_mcp.UNSET else req.limit

        notes: list[notebook_mcp.Note] = []
        for note in self._notes.values():
            haystack = f"{note.title}\n{note.body}".casefold()
            if query and query not in haystack:
                continue
            if required_tags and not required_tags.issubset(set(note.tags)):
                continue
            notes.append(note)
            if len(notes) >= limit:
                break

        return notebook_mcp.SearchNotesResponse(notes=notes)

    def delete_note(
        self,
        _ctx: notebook_mcp.ToolRequestContext,
        req: notebook_mcp.DeleteNoteRequest,
    ) -> notebook_mcp.DeleteNoteResponse:
        return notebook_mcp.DeleteNoteResponse(deleted=self._notes.pop(req.id, None) is not None)

    def health(
        self,
        _ctx: notebook_mcp.ToolRequestContext,
        _req: notebook_mcp.HealthRequest,
    ) -> notebook_mcp.HealthResponse:
        return notebook_mcp.HealthResponse(ok=True, note_count=len(self._notes))

    def list_notes(self) -> list[mcp.types.Resource]:
        return [
            mcp.types.Resource(name="note", uri=f"notebook://notes/{note.id}", title=note.title)
            for note in self._notes.values()
        ]

    def read_note(self, id: str) -> notebook_mcp.Note:
        note = self._notes.get(id)
        if note is None:
            raise mcp.shared.exceptions.McpError(
                mcp.types.ErrorData(code=mcp.types.INVALID_PARAMS, message=f"unknown note {id!r}")
            )
        return note

    def review_note(self, arguments: dict[str, str]) -> mcp.types.GetPromptResult:
        note = self.read_note(arguments["noteId"])
        focus = arguments.get("focus") or "clarity"
        text = f"Review this note for {focus} and suggest a clearer version.\n\n# {note.title}\n\n{note.body}"
        return mcp.types.GetPromptResult(
            messages=[mcp.types.PromptMessage(role="user", content=mcp.types.TextContent(type="text", text=text))]
        )


def new_server() -> mcp.server.lowlevel.Server:
    notebook = Notebook()
    return notebook_mcp.create_notebook_mcp_server(
        notebook_mcp.NotebookMCPHandlers(notebook_api=notebook, prompts=notebook, resources=notebook)
    )


# Local development tokens for --demo-auth. Never accept static tokens in
# production: without --demo-auth the server verifies RS256 JWTs against the
# JWKS declared in notebook.proto.
_DEMO_SCOPES = {
    "demo-read": ["notebook:read"],
    "demo-write": ["notebook:read", "notebook:write"],
}


class DemoVerifier:
    async def verify_token(self, token: str) -> AccessToken | None:
        scopes = _DEMO_SCOPES.get(token)
        if scopes is None:
            return None
        return AccessToken(token=token, client_id="demo", scopes=scopes, expires_at=int(time.time()) + 3600)


async def run_http_server(addr: str, demo_auth: bool) -> None:
    host, _, port = addr.rpartition(":")
    if demo_auth and host not in ("127.0.0.1", "localhost", "::1"):
        raise SystemExit("--demo-auth only listens on loopback addresses")
    app = notebook_mcp.create_notebook_mcp_http_app(new_server(), DemoVerifier() if demo_auth else None)
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.bind((host, int(port)))
    bound_host, bound_port = sock.getsockname()[:2]
    config = uvicorn.Config(app, log_level="warning", lifespan="on")
    config.load()
    print(f"listening on http://{bound_host}:{bound_port}", flush=True)
    await uvicorn.Server(config).serve(sockets=[sock])


async def run_stdio_server() -> None:
    server = new_server()
    async with mcp.server.stdio.stdio_server() as (read_stream, write_stream):
        await server.run(
            read_stream,
            write_stream,
            server.create_initialization_options(),
        )


def main() -> None:
    parser = argparse.ArgumentParser(description="Notebook MCP server generated from proto/notebook.proto.")
    parser.add_argument("--http", action="store_true", help="serve stateless Streamable HTTP instead of stdio")
    parser.add_argument("--addr", default="127.0.0.1:8080", help="HTTP listen address")
    parser.add_argument("--demo-auth", action="store_true", help="accept the local tokens demo-read and demo-write")
    args = parser.parse_args()
    if args.http:
        anyio.run(run_http_server, args.addr, args.demo_auth)
    else:
        anyio.run(run_stdio_server)


if __name__ == "__main__":
    main()
