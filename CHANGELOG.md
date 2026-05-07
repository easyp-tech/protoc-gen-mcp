# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

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

- Documentation now explains that direct `lang=javascript` generation is
  deferred; JavaScript users consume compiled TypeScript target output.
- Release messaging now states that tagged releases publish the
  `protoc-gen-mcp` binary, while downstream Node projects compile generated
  TypeScript against npm dependencies rather than using repository-published
  Node runtime artifacts.

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
