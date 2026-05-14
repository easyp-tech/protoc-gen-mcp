import { create } from "@bufbuild/protobuf";
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";

import { TimestampSchema } from "./generated/google/protobuf/timestamp_pb.js";
import { registerNotebookAPITools, type NotebookAPIToolHandler, type ToolRequestContext } from "./generated/proto/notebook_mcp.js";
import {
  CreateNoteResponseSchema,
  HealthResponseSchema,
  NoteSchema,
  SearchNotesResponseSchema,
  type CreateNoteRequest,
  type CreateNoteResponse,
  type HealthRequest,
  type HealthResponse,
  type Note,
  type SearchNotesRequest,
  type SearchNotesResponse,
} from "./generated/proto/notebook_pb.js";

function nowTimestamp() {
  const millis = Date.now();
  return create(TimestampSchema, {
    seconds: BigInt(Math.floor(millis / 1000)),
    nanos: (millis % 1000) * 1_000_000,
  });
}

class Notebook implements NotebookAPIToolHandler {
  private nextID = 1;
  private readonly notes = new Map<string, Note>();

  createNote(_ctx: ToolRequestContext, request: CreateNoteRequest): CreateNoteResponse {
    const note = create(NoteSchema, {
      id: `note-${this.nextID}`,
      title: request.title,
      body: request.body,
      tags: [...request.tags],
      dueDate: request.dueDate,
      createdAt: nowTimestamp(),
    });

    this.nextID += 1;
    this.notes.set(note.id, note);
    return create(CreateNoteResponseSchema, { note });
  }

  searchNotes(_ctx: ToolRequestContext, request: SearchNotesRequest): SearchNotesResponse {
    const query = request.query?.toLocaleLowerCase() ?? "";
    const requiredTags = new Set(request.tags);
    const limit = request.limit ?? 10;
    const notes: Note[] = [];

    for (const note of this.notes.values()) {
      const haystack = `${note.title}\n${note.body}`.toLocaleLowerCase();
      if (query !== "" && !haystack.includes(query)) {
        continue;
      }
      if (requiredTags.size > 0 && ![...requiredTags].every((tag) => note.tags.includes(tag))) {
        continue;
      }
      notes.push(note);
      if (notes.length >= limit) {
        break;
      }
    }

    return create(SearchNotesResponseSchema, { notes });
  }

  health(_ctx: ToolRequestContext, _request: HealthRequest): HealthResponse {
    return create(HealthResponseSchema, {
      ok: true,
      noteCount: this.notes.size,
    });
  }
}

const server = new Server(
  { name: "standalone-typescript-notebook", version: "0.1.0" },
  { capabilities: { tools: {} } },
);
const notebook = new Notebook();
registerNotebookAPITools(server, notebook, "notebook");

await server.connect(new StdioServerTransport());
