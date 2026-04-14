from __future__ import annotations

from datetime import datetime, timezone

import anyio
import mcp.server.lowlevel
import mcp.server.stdio

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

    def health(
        self,
        _ctx: notebook_mcp.ToolRequestContext,
        _req: notebook_mcp.HealthRequest,
    ) -> notebook_mcp.HealthResponse:
        return notebook_mcp.HealthResponse(ok=True, note_count=len(self._notes))


def new_server() -> mcp.server.lowlevel.Server:
    server = mcp.server.lowlevel.Server("standalone-python-notebook", version="0.1.0")
    notebook_mcp.register_notebook_api_tools(server, Notebook())
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
