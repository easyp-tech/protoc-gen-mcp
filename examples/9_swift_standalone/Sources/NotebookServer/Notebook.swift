import Foundation
import MCP
import ProtocGenMCPRuntime
import SwiftProtobuf

/// The in-memory notes, shared by concurrent HTTP requests.
actor NotebookStore {
    private var nextID = 1
    private var notes: [(String, Standalone_Notebook_V1_Note)] = []

    func add(_ build: (String) -> Standalone_Notebook_V1_Note) -> Standalone_Notebook_V1_Note {
        let note = build("note-\(nextID)")
        nextID += 1
        notes.append((note.id, note))
        return note
    }

    func all() -> [Standalone_Notebook_V1_Note] {
        notes.map(\.1)
    }

    func note(_ id: String) -> Standalone_Notebook_V1_Note? {
        notes.first { $0.0 == id }?.1
    }

    func delete(_ id: String) -> Bool {
        let before = notes.count
        notes.removeAll { $0.0 == id }
        return notes.count != before
    }
}

struct Notebook: NotebookAPIToolHandler, NotebookPromptHandler, NotebookResourceHandler {
    let store = NotebookStore()

    func createNote(_ context: ToolContext, _ request: Standalone_Notebook_V1_CreateNoteRequest) async throws -> Standalone_Notebook_V1_CreateNoteResponse {
        let note = await store.add { id in
            var note = Standalone_Notebook_V1_Note()
            note.id = id
            note.title = request.title
            note.body = request.body
            note.tags = request.tags
            if request.hasDueDate {
                note.dueDate = request.dueDate
            }
            note.createdAt = Google_Protobuf_Timestamp(date: Date())
            return note
        }
        var response = Standalone_Notebook_V1_CreateNoteResponse()
        response.note = note
        return response
    }

    func searchNotes(_ context: ToolContext, _ request: Standalone_Notebook_V1_SearchNotesRequest) async throws -> Standalone_Notebook_V1_SearchNotesResponse {
        let query = request.hasQuery ? request.query.lowercased() : ""
        let limit = request.hasLimit ? Int(request.limit) : 10
        var response = Standalone_Notebook_V1_SearchNotesResponse()
        for note in await store.all() {
            if !query.isEmpty, !"\(note.title)\n\(note.body)".lowercased().contains(query) {
                continue
            }
            if !Set(request.tags).isSubset(of: note.tags) {
                continue
            }
            response.notes.append(note)
            if response.notes.count >= limit {
                break
            }
        }
        return response
    }

    func deleteNote(_ context: ToolContext, _ request: Standalone_Notebook_V1_DeleteNoteRequest) async throws -> Standalone_Notebook_V1_DeleteNoteResponse {
        var response = Standalone_Notebook_V1_DeleteNoteResponse()
        response.deleted = await store.delete(request.id)
        return response
    }

    func health(_ context: ToolContext, _ request: Standalone_Notebook_V1_HealthRequest) async throws -> Standalone_Notebook_V1_HealthResponse {
        var response = Standalone_Notebook_V1_HealthResponse()
        response.ok = true
        response.noteCount = Int32(await store.all().count)
        return response
    }

    func listNotes() async throws -> [Resource] {
        await store.all().map { Resource(name: "note", uri: "notebook://notes/\($0.id)", title: $0.title) }
    }

    func readNote(id: String) async throws -> Standalone_Notebook_V1_Note {
        guard let note = await store.note(id) else {
            throw MCPError.invalidParams("unknown note '\(id)'")
        }
        return note
    }

    func reviewNote(_ arguments: [String: String]) async throws -> GetPrompt.Result {
        let note = try await readNote(id: arguments["noteId"] ?? "")
        let focus = arguments["focus"].flatMap { $0.isEmpty ? nil : $0 } ?? "clarity"
        let text = "Review this note for \(focus) and suggest a clearer version.\n\n# \(note.title)\n\n\(note.body)"
        return GetPrompt.Result(description: nil, messages: [Prompt.Message(role: .user, content: .text(text: text))])
    }
}
