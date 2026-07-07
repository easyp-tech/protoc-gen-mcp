# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.6.0] - 2026-07-08

### Added

- MCP **Prompts** generation from message-level `option (mcp.options.v1.prompt)`.
  Go emits `<File>PromptHandler` and
  `Register<File>Prompts(server, impl, opts...) error`, with one prompt argument
  per message field (scalar/enum; `optional` ⇒ not required).
- MCP **Resources** generation from message-level
  `option (mcp.options.v1.resource)` with either `uri` (static) or `uri_template`
  (templated, RFC-6570 `{param}`). Go emits `<File>ResourceHandler` and
  `Register<File>Resources(ctx, server, impl, opts...) error`; resource output is
  serialized as ProtoJSON. Full Go support; Python/Kotlin/Java/TypeScript emit
  interfaces with stub registration.
- `mcpruntime.Ptr[T]` helper for populating optional pointer fields from values
  in generated code.
- TypeScript MCP sidecar generation through `lang=typescript`, targeting
  `@modelcontextprotocol/sdk`, Protobuf-ES `_pb.ts` output, NodeNext `.js`
  imports, and Ajv-backed raw JSON Schema validation.
- Generated TypeScript runtime wiring for low-level `tools/list` and
  `tools/call` handlers, typed `<Service>ToolHandler` interfaces,
  namespace-aware `register<Service>Tools(server, impl, namespace?)`, ProtoJSON
  conversion, and structured output validation.
- Standalone Node examples:
  `examples/8_typescript_standalone` for a user-style TypeScript project and
  `examples/9_javascript_standalone` for JavaScript consumption of compiled
  TypeScript target `.js` plus `.d.ts` output.
- Node verification gates covering local npm compile/build checks, generated
  Node stdio behavior, and standalone TypeScript/JavaScript stdio parity.

### Changed

- **Breaking (Go):** the Go target no longer depends on
  `github.com/modelcontextprotocol/go-sdk`. Generated Go code and `mcpruntime`
  now use the in-repo, self-contained runtime: `RegisterProtoTool`/`ToolSpec`
  take `*mcpruntime.Server` (not `*mcp.Server`), annotation/icon types are
  `mcpruntime.*`, and servers run over stdio via `mcpruntime.ServeStdio`.
  Python, JVM, and TypeScript targets are unaffected and keep their native SDKs.
- The `initialize` handler now parses the request, negotiates the client's
  `protocolVersion`, and rejects a second `initialize` on an already-initialized
  session.
- Documentation now explains that direct `lang=javascript` generation is
  deferred; JavaScript users consume compiled TypeScript target output.
- Release messaging now states that tagged releases publish the
  `protoc-gen-mcp` binary, while downstream Node projects compile generated
  TypeScript against npm dependencies rather than using repository-published
  Node runtime artifacts.

### Fixed

- Resource generation emitted `Priority: <float>` into the `*float64`
  `Annotations.Priority` field, which did not compile; the value is now wrapped
  with `mcpruntime.Ptr`.
- Tool-annotation generation emitted bare `read_only_hint` / `idempotent_hint`
  bools into `*bool` fields, which did not compile; both are now wrapped with
  `proto.Bool` (matching `destructive_hint` / `open_world_hint`).
- `resources/read` held the server read lock while invoking the resource
  handler, deadlocking when a handler registered another primitive; the lock is
  now released before the handler runs.

## [0.3.0] - 2026-03-29

### Changed

- Renamed binary from `protoc-gen-mcp-go` to `protoc-gen-mcp` to prepare for
  future multi-language code generation support.
- Updated all easyp configs, GoReleaser manifest, CI workflows, generated code
  headers, golden snapshots, and documentation to reflect the new binary name.

### Added

- Agent skill for [skills.sh](https://skills.sh) in a dedicated repository
  [easyp-tech/protoc-gen-mcp-skill](https://github.com/easyp-tech/protoc-gen-mcp-skill)
  that teaches AI agents how to build MCP servers with this tool.
- Internal developer skill in `.github/skills/protoc-gen-mcp/` with
  troubleshooting and schema generation references.
- "Agent Skill" section in README with one-line install command.

### Fixed

- CI workflow lint and generate steps now use correct `-p mcp` path instead of
  stale `-p api`.

## [0.2.0] - 2026-03-17

### Changed

- Migrated options package to `mcp/options/v1` and added typed field
  customization options.

## [0.1.1] - 2026-03-17

### Fixed

- Escape generated schema string literals so proto comments containing
  backticks do not break `*.mcp.go` output.

## [0.1.0] - 2026-03-17

### Added

- Initial protobuf-first MCP generator MVP.
- `protoc-gen-mcp` protoc plugin and `mcpruntime` registration package.
- Support for proto3 scalars, enums, nested messages, repeated fields, oneof,
  optional, maps, recursive schemas, and well-known types.
- Custom MCP protobuf options (`mcp.options.v1`).
- Four standalone examples (helloworld, weather API, file manager, CRM system).
- Golden snapshot testing and stdio smoke tests.

## [0.0.0] - 2026-03-16

- Initial commit.

[Unreleased]: https://github.com/easyp-tech/protoc-gen-mcp/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/easyp-tech/protoc-gen-mcp/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/easyp-tech/protoc-gen-mcp/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/easyp-tech/protoc-gen-mcp/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/easyp-tech/protoc-gen-mcp/compare/v0.0.0...v0.1.0
[0.0.0]: https://github.com/easyp-tech/protoc-gen-mcp/releases/tag/v0.0.0
