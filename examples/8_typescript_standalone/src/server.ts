import { createServer } from "node:http";
import { parseArgs } from "node:util";

import { create } from "@bufbuild/protobuf";
import { InvalidTokenError } from "@modelcontextprotocol/sdk/server/auth/errors.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { ErrorCode, McpError, type GetPromptResult, type Resource } from "@modelcontextprotocol/sdk/types.js";

import { TimestampSchema } from "./generated/google/protobuf/timestamp_pb.js";
import {
  createNotebookMCPHttpHandler,
  createNotebookMCPServer,
  type NotebookAPIToolHandler,
  type NotebookMCPHandlers,
  type NotebookPromptHandler,
  type NotebookResourceHandler,
  type ToolRequestContext,
} from "./generated/proto/notebook_mcp.js";
import {
  CreateNoteResponseSchema,
  DeleteNoteResponseSchema,
  HealthResponseSchema,
  NoteSchema,
  SearchNotesResponseSchema,
  type CreateNoteRequest,
  type CreateNoteResponse,
  type DeleteNoteRequest,
  type DeleteNoteResponse,
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

class Notebook implements NotebookAPIToolHandler, NotebookPromptHandler, NotebookResourceHandler {
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

  deleteNote(_ctx: ToolRequestContext, request: DeleteNoteRequest): DeleteNoteResponse {
    return create(DeleteNoteResponseSchema, { deleted: this.notes.delete(request.id) });
  }

  health(_ctx: ToolRequestContext, _request: HealthRequest): HealthResponse {
    return create(HealthResponseSchema, {
      ok: true,
      noteCount: this.notes.size,
    });
  }

  listNotes(): Resource[] {
    return [...this.notes.values()].map((note) => ({
      name: "note",
      uri: `notebook://notes/${note.id}`,
      title: note.title,
    }));
  }

  readNote(id: string): Note {
    const note = this.notes.get(id);
    if (note === undefined) {
      throw new McpError(ErrorCode.InvalidParams, `unknown note '${id}'`);
    }
    return note;
  }

  reviewNote(args: Record<string, string>): GetPromptResult {
    const note = this.readNote(args.noteId ?? "");
    const focus = args.focus || "clarity";
    return {
      messages: [
        {
          role: "user",
          content: {
            type: "text",
            text: `Review this note for ${focus} and suggest a clearer version.\n\n# ${note.title}\n\n${note.body}`,
          },
        },
      ],
    };
  }
}

// Local development tokens for --demo-auth. Never accept static tokens in
// production: without --demo-auth the server verifies RS256 JWTs against the
// JWKS declared in notebook.proto.
const demoScopes: Record<string, string[]> = {
  "demo-read": ["notebook:read"],
  "demo-write": ["notebook:read", "notebook:write"],
};

const demoVerifier = {
  async verifyAccessToken(token: string) {
    const scopes = demoScopes[token];
    if (scopes === undefined) {
      throw new InvalidTokenError("unknown demo token");
    }
    return { token, clientId: "demo", scopes, expiresAt: Math.floor(Date.now() / 1000) + 3600 };
  },
};

const { values: flags } = parseArgs({
  options: {
    http: { type: "boolean", default: false },
    addr: { type: "string", default: "127.0.0.1:8080" },
    "demo-auth": { type: "boolean", default: false },
  },
});

const notebook = new Notebook();
const handlers: NotebookMCPHandlers = { notebookAPI: notebook, prompts: notebook, resources: notebook };

if (flags.http) {
  const separator = flags.addr.lastIndexOf(":");
  const host = flags.addr.slice(0, separator);
  const port = Number(flags.addr.slice(separator + 1));
  if (flags["demo-auth"] && !["127.0.0.1", "localhost", "::1"].includes(host)) {
    throw new Error("--demo-auth only listens on loopback addresses");
  }
  const handler = createNotebookMCPHttpHandler(handlers, flags["demo-auth"] ? demoVerifier : undefined);
  const httpServer = createServer(handler);
  httpServer.listen(port, host, () => {
    const address = httpServer.address();
    const bound = typeof address === "object" && address !== null ? address.port : port;
    console.log(`listening on http://${host}:${bound}`);
  });
} else {
  await createNotebookMCPServer(handlers).connect(new StdioServerTransport());
}
