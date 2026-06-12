# Phase 2: MCP Prompts — Walkthrough

## Summary

Added MCP Prompt support to protoc-gen-mcp. Users can now annotate protobuf messages with `(mcp.options.v1.prompt)` to generate prompt handlers across all 5 supported languages (Go, Python, Kotlin, Java, TypeScript).

## Changes

### Proto Options
- [options.proto](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/mcp/options/v1/options.proto) — Added `PromptOptions` message and extension `91008` for `google.protobuf.MessageOptions`
- Fields: `name`, `title`, `description`, `icons[]`

---

### IR Model
- [model.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/model.go) — Added `PromptModel` and `PromptArgumentModel` structs
- `PromptModel`: Name, Title, Description, ProtoName, Icons, Input (TypeRef), Arguments
- `PromptArgumentModel`: Name, Description, Required, ProtoName, ProtoKind

---

### Runtime
- [prompt.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/mcpruntime/prompt.go) — `ParsePromptArguments(args, msg, required)` using protobuf reflection
  - Supports: string, int32/64, uint32/64, sint32/64, fixed/sfixed, float, double, bool, bytes (base64), enums
  - Required field validation
  - Unknown args silently ignored

---

### Collector
- [collect_prompt.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/collect_prompt.go) — Validates and collects `PromptModel` from proto descriptors
  - Rejects: message fields, repeated fields, map fields, oneof fields
  - Accepts: scalar and enum types only
  - Uses camelCase for argument names (ProtoJSON convention)
- [collect.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/collect.go) — Integrated `collectPrompts` into `CollectFileModel`
- [metadata.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/metadata.go) — Added `getPromptOptions`

---

### Go Renderer
- [render_go.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/render_go.go) — Added prompt rendering block
  - Generates `<File>PromptHandler` interface with `(ctx, *Msg) ([]*PromptMessage, error)` methods
  - Generates `Register<File>Prompts(server, impl, opts...)` using `server.AddPrompt`
  - Each handler: creates proto message, calls `ParsePromptArguments`, delegates to impl

### Python Renderer
- [render_python.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/render_python.go) — Added `renderPythonPrompts`
  - `<File>PromptHandler` Protocol class with `async def` methods
  - `register_<file>_prompts(server, impl, *, namespace=None)` with `@server.add_prompt()`

### Kotlin Renderer
- [render_kotlin.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/render_kotlin.go) — Added `renderKotlinPrompts`
  - `interface <File>PromptHandler` with `suspend fun` methods returning `List<PromptMessage>`
  - `register<File>Prompts(server, impl, namespace?)` with `server.addPrompt`

### Java Renderer
- [render_java.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/render_java.go) — Added `renderJavaPrompts`
  - Nested `interface <File>PromptHandler` inside sidecar class
  - `static void register<File>Prompts(transport, impl, namespace)`

### TypeScript Renderer
- [render_typescript.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/render_typescript.go) — Added `renderTypeScriptPrompts`
  - `export interface <File>PromptHandler` with methods returning `Promise<GetPromptResult>`
  - `export function register<File>Prompts(server, impl, namespace?)`

---

### Generator & Models
- [generator.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/generator.go) — Updated skip conditions: files with prompts but no services still generate output
- [jvm_model.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/jvm_model.go) — Added `Prompts []PromptModel` to `JVMFileModel`
- [typescript_model.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/typescript_model.go) — Added `Prompts []PromptModel` to `TypeScriptFileModel`
- [jvm_collect.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/jvm_collect.go) — Passes `model.Prompts` through to JVM model
- [typescript_collect.go](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/codegen/typescript_collect.go) — Passes `model.Prompts` through to TS model

---

### Test Fixture
- [prompts.proto](file:///Users/zergslaw/Documents/Projects/easyp-tech/protoc-gen-mcp/internal/testproto/prompts/v1/prompts.proto) — 3 prompts (CodeReview, Summarize, ExplainError) + 1 non-prompt message + enum

## Verification

| Check | Result |
|---|---|
| `go build ./cmd/protoc-gen-mcp` | ✅ PASS |
| 5 golden tests (Go/Python/Kotlin/Java/TS) | ✅ PASS |
| `mcpruntime` prompt tests (14) | ✅ PASS |
| Collector prompt tests (9) | ✅ PASS |
| `easyp generate` (Go + Python) | ✅ PASS |
| `go build ./internal/testproto/prompts/...` | ✅ PASS |
| `internal/examplemcp` integration | ✅ PASS |
| TypeScript compile tests | ⚠️ SKIP (env: no `node_modules`) |
