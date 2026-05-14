// @ts-check

import { create } from "@bufbuild/protobuf";
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";

import { TimestampSchema } from "../../8_typescript_standalone/dist/generated/google/protobuf/timestamp_pb.js";
import { registerNotebookAPITools } from "../../8_typescript_standalone/dist/generated/proto/notebook_mcp.js";
import {
  CreateNoteResponseSchema,
  HealthResponseSchema,
  NoteSchema,
  SearchNotesResponseSchema,
} from "../../8_typescript_standalone/dist/generated/proto/notebook_pb.js";

/**
 * @typedef {import("../../8_typescript_standalone/dist/generated/proto/notebook_mcp.js").NotebookAPIToolHandler} NotebookAPIToolHandler
 * @typedef {import("../../8_typescript_standalone/dist/generated/proto/notebook_mcp.js").ToolRequestContext} ToolRequestContext
 * @typedef {import("../../8_typescript_standalone/dist/generated/proto/notebook_pb.js").CreateNoteRequest} CreateNoteRequest
 * @typedef {import("../../8_typescript_standalone/dist/generated/proto/notebook_pb.js").CreateNoteResponse} CreateNoteResponse
 * @typedef {import("../../8_typescript_standalone/dist/generated/proto/notebook_pb.js").HealthRequest} HealthRequest
 * @typedef {import("../../8_typescript_standalone/dist/generated/proto/notebook_pb.js").HealthResponse} HealthResponse
 * @typedef {import("../../8_typescript_standalone/dist/generated/proto/notebook_pb.js").Note} Note
 * @typedef {import("../../8_typescript_standalone/dist/generated/proto/notebook_pb.js").SearchNotesRequest} SearchNotesRequest
 * @typedef {import("../../8_typescript_standalone/dist/generated/proto/notebook_pb.js").SearchNotesResponse} SearchNotesResponse
 */

function nowTimestamp() {
  const millis = Date.now();
  return create(TimestampSchema, {
    seconds: BigInt(Math.floor(millis / 1000)),
    nanos: (millis % 1000) * 1_000_000,
  });
}

/** @implements {NotebookAPIToolHandler} */
class Notebook {
  constructor() {
    this.nextID = 1;
    /** @type {Map<string, Note>} */
    this.notes = new Map();
  }

  /**
   * @param {ToolRequestContext} _ctx
   * @param {CreateNoteRequest} request
   * @returns {CreateNoteResponse}
   */
  createNote(_ctx, request) {
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

  /**
   * @param {ToolRequestContext} _ctx
   * @param {SearchNotesRequest} request
   * @returns {SearchNotesResponse}
   */
  searchNotes(_ctx, request) {
    const query = request.query?.toLocaleLowerCase() ?? "";
    const requiredTags = new Set(request.tags);
    const limit = request.limit ?? 10;
    /** @type {Note[]} */
    const notes = [];

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

  /**
   * @param {ToolRequestContext} _ctx
   * @param {HealthRequest} _request
   * @returns {HealthResponse}
   */
  health(_ctx, _request) {
    return create(HealthResponseSchema, {
      ok: true,
      noteCount: this.notes.size,
    });
  }
}

const server = new Server(
  { name: "standalone-javascript-notebook", version: "0.1.0" },
  { capabilities: { tools: {} } },
);
const notebook = new Notebook();
registerNotebookAPITools(server, notebook, "notebook");

await server.connect(new StdioServerTransport());
