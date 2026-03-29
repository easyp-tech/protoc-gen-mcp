# AGENTS

## Scope

This repository implements a protobuf-first MCP generator and runtime for Go.
The MVP is intentionally narrow and must stay decision-consistent with the
current architecture unless explicitly revised.

## Stack

- Go 1.24+
- `easyp v0.15.2-rc1` for repository linting and code generation workflows
- `google.golang.org/protobuf` for code generation, reflection, and ProtoJSON
- `github.com/modelcontextprotocol/go-sdk/mcp` as the MCP runtime
- `github.com/google/jsonschema-go/jsonschema` for JSON Schema parsing and
  validation

## Layout

- `cmd/protoc-gen-mcp`: protoc plugin entrypoint for `--mcp_out`
- `cmd/example-mcp-server`: runnable stdio MCP server for manual agent/client checks
- `mcpruntime`: public runtime helpers used by generated code
- `.github/workflows`: GitHub Actions CI and release workflows
- `.goreleaser.yaml`: release packaging for the plugin binary
- `easyp.yaml`: main repository config for shipped protobuf APIs
- `easyp.test.yaml`: development and test config for fixture generation
- `mcp/options/v1/options.proto`: custom protobuf options for MCP metadata
- `internal/codegen`: code generation logic
- `internal/examplemcp`: reusable example MCP server wiring and stdio smoke test
- `internal/schema`: protobuf descriptor to JSON Schema conversion
- `internal/testproto`: protobuf fixtures and generated code used in repository tests
- `testdata/golden`: golden snapshots for generated `*.mcp.go` files
- `testdata/unsupported`: negative fixtures for fail-fast generator coverage

## MVP Rules

- Target only `proto3`
- Support only MCP tools with unary request/response
- Supported protobuf features: scalar, enum, nested message, repeated,
  `oneof`, `optional`, maps, recursive message schemas via `$defs`/`$ref`, and
  these well-known types:
  `google.protobuf.Any`, `Empty`, `Timestamp`, `Duration`, `FieldMask`,
  `Struct`, `Value`, `ListValue`, `BoolValue`, `StringValue`, `BytesValue`,
  `Int32Value`, `UInt32Value`, `Int64Value`, `UInt64Value`, `FloatValue`,
  and `DoubleValue`
- Unsupported and required to fail fast at generation time:
  non-unary protobuf RPC methods and unsupported `google.protobuf` message
  types
- JSON contract is ProtoJSON-first
- Tool input requiredness is a generated MCP schema policy, not the protobuf
  `required` label: singular fields are marked as required in the tool schema
  by default unless they are `proto3 optional`, `repeated`, `map`, `oneof`,
  or explicitly marked optional through MCP field options
- Generated schemas must accept explicit JSON `null` for fields that are not
  marked as required by that generated MCP tool schema so MCP clients that
  validate cached `inputSchema` remain compatible with ProtoJSON unset
  semantics
- Generated schemas should carry agent-facing examples for complex ProtoJSON
  shapes. Explicit JSON examples from comments/options are materialized as JSON
  literals when possible, and fallback examples are synthesized for advanced
  forms such as maps, `Any`, recursive messages, and ProtoJSON special scalar
  encodings
- `mcp.options.v1` must declare its `go_package` directly in
  `mcp/options/v1/options.proto` so downstream Easyp consumers do not need
  a special `go_package_prefix` override

## Public API

- Generated code exposes `<Service>ToolHandler`
- Generated code exposes `Register<Service>Tools(server, impl, opts...) error`
- Runtime exposes only the minimal registration options used by generated code
- Generated MCP tool names must not contain dots; namespace prefixes and method
  names are joined with underscores, and any dots in configured segments are
  normalized to underscores

## Current Status

- Implemented:
  - `cmd/protoc-gen-mcp` plugin scaffold and generated `*.mcp.go` bindings
  - custom MCP protobuf options in `mcp/options/v1/options.proto`
  - generated tool metadata includes `ToolAnnotations` (`read_only_hint`, `destructive_hint`, `idempotent_hint`, `open_world_hint`) and `Icon` mappings directly to the Go SDK
  - dedicated `examples/` directory featuring 4 standalone integration projects spanning quickstarts to complex CRM mocks
  - support for `oneof` explicit requiredness through `mcp.options.v1.oneof` options
  - strict schema generation correctly differentiating zero-values (`0`, `0.0`, `""`) using pointer constraints
  - runtime registration and JSON Schema validation in `mcpruntime`
  - descriptor-to-JSON-Schema generation in `internal/schema`
  - support for maps, recursive message schemas, top-level/nested `oneof`, and
    selected well-known protobuf types in generated schemas
  - tool-first requiredness in generated MCP schemas: singular non-`optional`
    fields are required by default unless they are `repeated`, `map`, `oneof`,
    or relaxed through MCP field options
  - ProtoJSON-compatible nullable support for fields that are not required by
    that generated MCP tool schema, including client-side acceptance of
    explicit JSON `null`
  - auto-generated JSON Schema examples for complex fields, including parsed
    explicit JSON examples for `Any` payloads plus synthesized fallback
    examples for maps, recursive messages, repeated scalars, and special
    ProtoJSON scalar encodings
  - generated schema JSON constants are emitted as interpreted Go string
    literals so proto comments and examples containing backticks do not break
    generated `*.mcp.go` files
  - Easyp main/test configs without any special managed-mode override for
    `mcp.options.v1`, because the options package declares `go_package`
    directly in source
  - GitHub Actions CI in `.github/workflows/tests.yml`
  - GoReleaser-based tagged releases in `.github/workflows/release.yml` and
    `.goreleaser.yaml`
  - runnable `cmd/example-mcp-server` and reusable wiring in
    `internal/examplemcp`
  - underscore-only MCP tool naming; expected example tool names are
    `example_CreateReport`, `example_DescribeAdvancedShapes`,
    `example_DescribeScalarShapes`, and `example_Health`
  - `example_DescribeAdvancedShapes` covers the full current advanced contract
    matrix in the test server: maps, timestamps, durations, field masks,
    `Struct`/`Value`/`ListValue`, `Any`, scalar wrappers, raw float ProtoJSON,
    recursive nodes, and top-level `oneof`
  - `example_DescribeScalarShapes` covers plain protobuf scalar kinds in the
    test server, including `int32`/`sint32`/`sfixed32`, `uint32`/`fixed32`,
    `int64`/`sint64`/`sfixed64`, `uint64`/`fixed64`, `float`, `double`,
    `bool`, `string`, `bytes`, enum, nested message, repeated scalar, and
    proto3 `optional` scalar/enum fields
- Verified:
  - `easyp` lint and generation flows for `mcp` and `internal/testproto`
  - `go test ./...`
  - stdio smoke test via `internal/examplemcp/stdio_test.go`
  - client-side schema acceptance checks against advertised `tools/list`
    `inputSchema` for canonical valid payloads, including recursive objects and
    explicit `null` on fields that are not schema-required
  - manual Cursor validation of `example_Health` and `example_CreateReport`
- Open:
  - automate broader client compatibility checks beyond Go SDK + Cursor manual
    verification
  - manually verify recursive payloads and explicit `null` payloads in external
    agent runtimes beyond repository tests
  - confirm direct external MCP tool invocation in Codex agent runtimes when
    tool wrappers are surfaced by the host session

## Working Rules

- Keep `AGENTS.md` in sync whenever any of these change:
  - technology choices
  - repository layout
  - supported/unsupported protobuf features
  - generated public API
  - build/test commands
- Do not introduce a manual end-user tool-binding API in MVP
- Prefer compile-time generator errors over runtime ambiguity
- When generator output for `internal/testproto` changes, regenerate it through
  `easyp.test.yaml` and refresh the matching golden snapshot
- Repository verification should use `easyp`; avoid ad hoc direct generation
  flows for development checks
- Keep `mcp/options/v1/options.proto` as the source of truth for the
  options package `go_package`; do not reintroduce a special Easyp override
  unless the package layout changes again

## Commands

- Validate configs:
  - `easyp --cfg easyp.yaml validate-config`
  - `easyp --cfg easyp.test.yaml validate-config`
- Lint shipped protobuf API: `easyp --cfg easyp.yaml lint -p mcp -r .`
- Generate shipped protobuf API: `easyp --cfg easyp.yaml generate -p mcp -r .`
- Lint test fixtures: `easyp --cfg easyp.test.yaml lint -p internal/testproto -r .`
- Generate test fixtures: `easyp --cfg easyp.test.yaml generate -p internal/testproto -r .`
- Build plugin: `go build ./cmd/protoc-gen-mcp`
- Validate GoReleaser config: `goreleaser check`
- Build example MCP server binary:
  - `go build -o example-mcp-server ./cmd/example-mcp-server/main.go`
- Run example MCP server: `go -C /abs/path/to/protoc-gen-mcp run ./cmd/example-mcp-server`
- Run built example MCP server binary: `./example-mcp-server`
- Run tests: `go test ./...`
