# Python Target Design

**Date:** 2026-04-08

**Status:** Approved in brainstorming, pending spec review and user review

## Goal

Extend `protoc-gen-mcp` so a single plugin can generate MCP server bindings for
Python with functional parity to the existing Go target at the MCP boundary.

The Python target must preserve the current repository contract:

- protobuf remains the source of truth
- ProtoJSON remains the JSON contract
- generated MCP schemas remain the validation contract
- the generated API remains registration-first: user passes a server and a
  handler implementation, generated code registers tools on that server

## Success Criteria

The Python target is successful when all of the following are true:

- one binary `protoc-gen-mcp` supports both Go and Python generation
- language selection is explicit through plugin options
- Python generated tools expose the same tool names, schemas, metadata, and
  runtime behavior as the Go target for the same `.proto`
- the Python target uses the official MCP Python SDK, not an alternate MCP
  runtime
- the Python target is self-contained at the generated-file level and does not
  require an installable `protoc-gen-mcp` Python runtime package
- the Python target works with standard `google.protobuf` generated modules
- the repository can verify Python parity through golden and integration tests

## Non-Goals

The first Python MVP does not include:

- a `betterproto` or `grpclib` runtime implementation
- a separate installable Python runtime package owned by this repository
- FastMCP-specific bindings
- Python-specific features that change the observable MCP contract relative to
  the Go target
- support for protobuf generators other than standard `google.protobuf`

## Product Decisions

### CLI Contract

The repository continues to ship a single binary, `protoc-gen-mcp`.

Language selection is explicit through plugin options:

- `lang=go`
- `lang=python`

Backward compatibility rule:

- if `lang` is omitted, generation defaults to `go`
- `lang=go` preserves the current plugin behavior
- `lang=python` selects the Python renderer

Python runtime selection is also explicit through plugin options:

- `python_runtime=google.protobuf`
- `python_runtime=betterproto`
- `python_runtime=grpclib`

Only `python_runtime=google.protobuf` is supported in the MVP. The other
declared values are reserved for future expansion and must fail fast during
generation with a clear error.

Runtime option compatibility rule:

- if `lang=python` and `python_runtime` is omitted, it defaults to
  `google.protobuf`
- if `lang=go`, any supplied `python_runtime` value is a generation error

### Python Baseline

The Python target supports `Python 3.10+`.

This matches the minimum version supported by the official MCP Python SDK and
keeps the generated code aligned with the SDK's asynchronous server model.

### Official MCP SDK Target

The MVP pins the Python MCP dependency contract to:

- package: `mcp`
- supported version range for generated-code compatibility: `>=1.27,<2`

The generated code targets the low-level API surface from that SDK:

- `mcp.server.Server`
- `mcp.shared.context.RequestContext`
- `mcp.types.Tool`
- `mcp.types.CallToolResult`
- `mcp.types.TextContent`

The MVP does not target `FastMCP` or any higher-level wrapper API.

### Python Runtime Model

The generated Python target is self-contained. For each proto file with MCP
tools, the plugin emits a `*_mcp.py` module that imports only public external
dependencies such as:

- the official MCP Python SDK
- standard `google.protobuf` generated modules and JSON formatting helpers
- the `jsonschema` Python package used for JSON Schema validation

The generated `*_mcp.py` module must not import a repository-owned installable
Python runtime package.

The generated code assumes that standard protobuf Python output and MCP Python
output share the same output root and package structure.

### Handler Model

The generated Python API mirrors the Go DX model instead of Go syntax.

Each generated module exports:

- a typed `Protocol` named `<ServiceName>ToolHandler`
- a type alias `ToolRequestContext` for the concrete official MCP Python SDK
  request context type used by generated handlers
- module-level schema JSON constants in upper snake case:
  `<SERVICE_NAME>_<METHOD_NAME>_INPUT_SCHEMA_JSON` and
  `<SERVICE_NAME>_<METHOD_NAME>_OUTPUT_SCHEMA_JSON`
- a registration helper named `register_<service_name>_tools`

The registration helper accepts:

- a server instance from the official low-level MCP Python SDK
- a handler implementation object
- a keyword-only `namespace: str | None = None` override

The namespace keyword is required in MVP and is the Python equivalent of Go
`mcpruntime.WithNamespace`:

- `None` means use the generated default namespace
- a provided string is trimmed, normalized, and used instead of the generated
  namespace
- normalization rules must match the Go runtime exactly

The generated handler protocol is explicit:

- the protocol name is `<ServiceName>ToolHandler`
- handler method names are the proto RPC method names converted to snake case
- each method signature is:
  `def method(self, ctx: ToolRequestContext, req: <RequestMessage>) -> <ResponseMessage> | Awaitable[<ResponseMessage>]`

The generated glue must accept both synchronous and asynchronous handler
methods. The surrounding server model remains asynchronous because that is the
official SDK model.

## Architecture

### Internal Pipeline

The generator is refactored around a shared internal intermediate
representation, referred to here as the semantic IR.

Pipeline:

1. `protogen` loads protobuf descriptors and plugin options
2. a semantic collector normalizes services, methods, comments, options, tool
   names, schemas, annotations, icons, deprecation state, and diagnostics into
   a language-neutral IR
3. a language-specific renderer emits Go or Python code from the same IR

This is an in-memory internal IR only. It is not emitted as a user-facing
manifest file.

### Why Shared IR

The current Go implementation already contains the repository's observable
contract. Adding Python by duplicating that logic would create drift risk in:

- tool naming
- method filtering
- metadata inheritance
- hidden and deprecated handling
- schema constants
- nullable semantics
- future feature additions

Using one semantic IR means the repository has one place where generator
semantics are decided and two places where code is rendered.

### Existing Shared Components

`internal/schema` remains the source of truth for schema generation and
ProtoJSON-driven schema semantics.

The existing Go target is migrated to render from the same shared IR so both
languages depend on the same semantic decisions.

### Python Server Composition Model

The official low-level MCP Python SDK exposes one `tools/list` handler and one
`tools/call` handler per server. Therefore, the Python target cannot register
each generated service independently through isolated decorator state.

Instead, generated Python modules cooperate through a server-scoped internal
registry:

- the first `register_<service_name>_tools(...)` call installs shared
  `tools/list` and `tools/call` handlers onto the server
- subsequent registration calls add tool definitions and dispatch callbacks into
  that same registry
- duplicate tool names on the same server fail fast, matching the Go runtime

This preserves the Go-style workflow where multiple generated registration
helpers can target the same server instance.

## Python Generated API

For a proto file `foo.proto`, the Python target emits `foo_mcp.py` alongside
the standard protobuf-generated `foo_pb2.py`.

Output/import contract for MVP:

- the Python protobuf generator and `protoc-gen-mcp` with `lang=python` must
  write into the same Python package tree
- if protobuf generation creates `pkg/foo_pb2.py`, MCP generation creates
  `pkg/foo_mcp.py`
- `foo_mcp.py` imports the corresponding `foo_pb2` module using that same
  package/module path
- generating `*_pb2.py` and `*_mcp.py` into different output roots is out of
  scope for MVP

The generated module contains:

- tool schema JSON constants as module-level strings
- helper functions for JSON Schema loading and validation using the `jsonschema`
  package
- helper functions for ProtoJSON parsing and serialization
- a `Protocol`-based service handler contract
- a registration function that binds protobuf-backed MCP tools onto the
  official low-level MCP server

The generated code is self-contained. Shared helper logic may be duplicated
between generated files in the MVP if that keeps integration simple and avoids a
repository-owned Python runtime dependency.

## Runtime Behavior

### Registration

The Python registration helper must register the same tools as the Go target
for the same descriptors.

It must preserve:

- tool name normalization rules
- service namespace behavior
- title and description propagation
- annotations and icons
- hidden method suppression
- deprecated schema marking
- namespace override behavior through the `namespace=` keyword argument

Registration API shape is fixed for MVP:

- function name: `register_<service_name>_tools`
- return type: `None`
- keyword override surface: `namespace: str | None = None`
- no variadic Python option callbacks in MVP

### Request Flow

For each tool call:

1. collect raw MCP tool arguments
2. validate arguments against the generated input JSON Schema with the
   `jsonschema` package
3. parse arguments into the appropriate protobuf request message using official
   `google.protobuf` ProtoJSON behavior
4. invoke the handler implementation
5. serialize the protobuf response back to ProtoJSON
6. validate the serialized JSON against the generated output schema with the
   `jsonschema` package
7. return both text content and structured content consistent with the Go
   target

### Error Behavior

The Python target must match the Go target's behavior at the MCP boundary:

- invalid input schema data becomes an invalid-params style MCP error
- ProtoJSON parse failures become invalid-params style MCP errors
- any exception raised by the user handler becomes a tool error result
- output schema mismatches fail as server-side runtime errors

Python handler error contract:

- handler methods return only a protobuf response value or an awaitable
  resolving to that value
- handler methods do not return an error object
- user/business errors are represented by raising exceptions
- generated runtime catches exceptions raised from the handler invocation and
  converts them into tool error results
- generated runtime does not convert its own schema/ProtoJSON/registration
  failures into tool error results

### Sync And Async Handlers

The generated glue must support both handler shapes:

- `def method(...)`
- `async def method(...)`

The registration helper detects whether the returned result is awaitable and
awaits it when necessary.

The server itself remains async-first because the official Python MCP SDK is
async-first.

## Parity Contract

Python parity is defined at the observable MCP boundary, not by matching Go
syntax.

The Python target must match the Go target for:

- tool names
- `inputSchema`
- `outputSchema`
- tool titles and descriptions
- annotations
- icons
- ProtoJSON request parsing semantics
- ProtoJSON response serialization semantics
- nullable acceptance for fields that are not schema-required
- supported and unsupported protobuf feature matrix
- error behavior visible to MCP clients

Idiomatic Python differences are allowed for:

- file naming conventions within Python norms
- handler contract expression using Python protocols or base classes
- sync and async handler support

## Python Runtime Selection Strategy

The plugin exposes `python_runtime` now so the public option shape is stable
from the first Python release.

MVP behavior:

- `python_runtime=google.protobuf`: supported
- `python_runtime=betterproto`: fail fast with a clear unsupported-runtime
  error
- `python_runtime=grpclib`: fail fast with a clear unsupported-runtime error

This keeps the design extensible without weakening the first implementation.

## Repository Layout Changes

The repository keeps its current top-level shape and adds Python-target support
inside the existing generator.

Expected additions:

- shared semantic IR and collector code in or near `internal/codegen`
- a Go renderer using the shared IR
- a Python renderer using the shared IR
- Python golden fixtures
- Python example server files
- Python integration tests
- documentation updates for Python generation and usage

No separate Python package owned by this repository is required for MVP.

## Testing Strategy

### Golden Tests

Add Python golden files for generated `*_mcp.py` output, parallel to the
existing Go golden workflow.

These tests verify:

- stable generated surface
- stable imports
- stable schema constant emission
- stable public registration helpers

### Cross-Target Contract Tests

Add tests that compare Go and Python render outputs at the semantic contract
level for the same proto input.

These tests verify:

- same tool names
- same input and output schema JSON
- same titles, descriptions, annotations, and icons
- same hidden and deprecated handling

### Python Runtime Integration Tests

Add Python integration tests using the official MCP Python SDK and a generated
Python example server.

These tests verify:

- `tools/list` exposes the same tools as Go
- valid calls succeed
- nullable accepted payloads succeed for non-required fields
- ProtoJSON special scalar forms behave correctly
- supported well-known types behave correctly
- recursive payloads behave correctly
- `Any` payloads behave correctly
- output schema validation is enforced

### Negative Tests

Add fail-fast tests for:

- unsupported `python_runtime` values
- streaming RPCs
- unsupported protobuf well-known types
- invalid plugin option combinations

## Examples And Documentation

Documentation must be updated to explain:

- how to select `lang=python`
- how to select `python_runtime`
- that only `google.protobuf` is implemented in the MVP
- the generated Python file naming and usage model
- the requirement to also generate standard `*_pb2.py` files

Examples must show the same registration-first workflow already used by Go:

1. generate protobuf code
2. generate MCP bindings
3. implement the handler object
4. create an official MCP Python SDK server
5. call the generated registration helper
6. serve over stdio or another supported transport

## Risks And Mitigations

### Drift Between Go And Python

Risk: renderer-specific behavior diverges over time.

Mitigation: shared semantic IR, cross-target contract tests, and golden tests.

### Python Dependency Surface

Risk: self-contained generated modules may duplicate helper logic.

Mitigation: accept small duplication in MVP to avoid a repository-owned runtime
package dependency and keep install/use simple.

### Async Integration Complexity

Risk: sync and async handlers may complicate invocation flow.

Mitigation: keep the server async-first and only adapt the handler call site,
not the overall runtime model.

## Implementation Boundaries For Planning

The implementation plan should treat this as a single project with these major
workstreams:

- shared semantic IR refactor
- Go renderer migration
- Python renderer implementation
- Python example and integration coverage
- documentation and repository workflow updates

The plan should not include real support for `betterproto` or `grpclib` beyond
declaring the option and producing explicit unsupported errors.
