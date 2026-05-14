# Troubleshooting Guide

## Generation Errors

### "proto2 syntax is not supported"
- **Cause**: A `.proto` file uses `syntax = "proto2"`
- **Fix**: Only `proto3` is supported. Convert to proto3 syntax.

### "streaming RPC is not supported"
- **Cause**: A service method uses `stream` keyword
- **Fix**: MVP supports only unary request/response. Remove `stream` from method signatures.

### "unsupported well-known type"
- **Cause**: Using a `google.protobuf.*` type not in the supported list
- **Fix**: Check AGENTS.md for supported well-known types. Use a supported alternative or add support in `internal/schema/schema.go`.

### Generated code has compile errors after regeneration
- **Cause**: Usually a mismatch between protobuf output and MCP sidecar output.
- **Fix**: Regenerate the affected package through the matching `easyp` config,
  then refresh the relevant golden snapshot. Do not patch generated files by
  hand except while diagnosing the renderer.

### Unknown `protoc-gen-mcp` parameter
- **Cause**: Plugin params are parsed through a typed single-source option path.
- **Fix**: Use supported params only:
  `lang=go|python|kotlin|java|typescript`,
  `python_runtime=google.protobuf|betterproto|grpclib`, and
  `python_handler=dataclass|protobuf|dataclass+protobuf`.

## Test Failures

### Golden snapshot mismatch
- **Cause**: Generated output changed but golden file was not updated
- **Fix**:
  ```bash
  easyp --cfg easyp.test.yaml generate -p internal/testproto -r .
  ```
  Then refresh the matching file under `testdata/golden/` for the language that
  changed, such as `example.mcp.go.golden`, `example_mcp.py.golden`,
  `example_mcp_pb.py.golden`, `example_mcp.java.golden`,
  `example_mcp.kt.golden`, or `example_mcp.ts.golden`.

### Schema validation test failure
- **Cause**: JSON Schema doesn't match expected structure for a field type
- **Fix**: Check `internal/schema/schema.go` for the field type mapping. Verify the test expectations in `example_schema_test.go`.

### Stdio smoke test failure
- **Cause**: Tool list, schema, or response format changed
- **Fix**: Check `internal/examplemcp/server.go` handler implementations match the current proto definition. Verify expected tool names and payloads in `stdio_test.go`.

### Go tests fail because `protoc` is missing
- **Cause**: A test or helper reintroduced shelling out to external `protoc`.
- **Fix**: Generator tests should build descriptor requests in-process through
  `github.com/bufbuild/protocompile`. Local `go test ./...` must not require a
  `protoc` binary in `PATH`.

### Python tests fail because `protobuf` or `mcp` is missing globally
- **Cause**: A test bypassed the hermetic Python bootstrap.
- **Fix**: Use `internal/pythontest` so tests create an isolated virtualenv and
  install pinned runtime dependencies instead of relying on global packages.

## Runtime Issues

### "duplicate tool name" error at registration
- **Cause**: Two services registered the same tool name on the same server
- **Fix**: Use different `namespace` values in `ServiceOptions` or `WithNamespace()` option.

### Schema validation rejects valid input
- **Cause**: Field incorrectly marked required, or null not accepted
- **Fix**: Check requiredness policy. Ensure `optional` keyword is present on fields that should be optional. Verify nullable wrapping in schema output.

### ProtoJSON unmarshal error
- **Cause**: Input JSON doesn't match ProtoJSON conventions
- **Common issues**:
  - `int64`/`uint64` must be JSON strings, not numbers
  - Enum values must be string names, not integers
  - `Timestamp` must be RFC 3339 format
  - `bytes` must be base64 encoded

### Python raw protobuf handlers are missing
- **Cause**: The example or Easyp config generated only dataclass mode.
- **Fix**: Use `python_handler=protobuf` for protobuf-only output in
  `*_mcp.py`, or `python_handler=dataclass+protobuf` for dataclass output in
  `*_mcp.py` plus raw protobuf output in `*_mcp_pb.py`.

### JVM server compiles but behavior does not match Go/Python
- **Cause**: Renderer delegated schema or ProtoJSON behavior to SDK-level
  helpers instead of the generator-owned low-level contract.
- **Fix**: Keep Java/Kotlin registration on the low-level request-handler seam
  and reuse generated raw schema JSON plus ProtoJSON parse/marshal helpers.

### TypeScript NodeNext compile errors
- **Cause**: Generated imports are missing `.js` specifiers or mix type/value
  imports under `verbatimModuleSyntax`.
- **Fix**: Keep Protobuf-ES imports `.js`-suffixed and split `import type`
  from value imports in `render_typescript.go`.

## easyp Workflow Issues

### "plugin not found" during generation
- **Cause**: `protoc-gen-mcp` binary not built or not in PATH
- **Fix**: Repository Easyp configs should invoke the plugin through the local
  Go module. Standalone examples may build or install the local binary as part
  of their Makefile/Gradle/npm flow; run the example's documented `make setup`,
  `make generate`, or `make build` target first.

### Lint failures on options.proto
- **Cause**: `go_package` declaration issue or import path mismatch
- **Fix**: `mcp/options/v1/options.proto` must declare its own `go_package`. Do not use Easyp `go_package_prefix` overrides for this package.

## Debugging Tips

1. **Inspect generated schema**: find the generated schema JSON constant in the
   sidecar for the target language and paste JSON into a formatter.
2. **Trace schema generation**: inspect `internal/schema` first; renderers
   should consume schema semantics instead of recomputing them.
3. **Test one target**: run the focused `internal/codegen` contract test for
   Python, Java, Kotlin, or TypeScript before running `go test ./...`.
4. **Test stdio interaction**: use `internal/examplemcp` tests, standalone
   `examples` tests, or MCP Inspector for a user-facing check.
5. **Validate easyp configs**: `easyp --cfg easyp.yaml validate-config` and
   `easyp --cfg easyp.test.yaml validate-config`.
