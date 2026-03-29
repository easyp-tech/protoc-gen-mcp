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
- **Cause**: Usually a mismatch between `*.pb.go` and `*.mcp.go`
- **Fix**: Ensure both `protoc-gen-go` and `protoc-gen-mcp` are regenerated together via `easyp generate`.

## Test Failures

### Golden snapshot mismatch
- **Cause**: Generated output changed but golden file was not updated
- **Fix**:
  ```bash
  easyp --cfg easyp.test.yaml generate -p internal/testproto -r .
  cp internal/testproto/example/v1/example.mcp.go testdata/golden/example.mcp.go.golden
  ```

### Schema validation test failure
- **Cause**: JSON Schema doesn't match expected structure for a field type
- **Fix**: Check `internal/schema/schema.go` for the field type mapping. Verify the test expectations in `example_schema_test.go`.

### Stdio smoke test failure
- **Cause**: Tool list, schema, or response format changed
- **Fix**: Check `internal/examplemcp/server.go` handler implementations match the current proto definition. Verify expected tool names and payloads in `stdio_test.go`.

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

## easyp Workflow Issues

### "plugin not found" during generation
- **Cause**: `protoc-gen-mcp` binary not built or not in PATH
- **Fix**: The easyp config uses `go run ./cmd/protoc-gen-mcp` to invoke the plugin directly. Ensure `go` is available and the module dependencies are resolved (`go mod download`).

### Lint failures on options.proto
- **Cause**: `go_package` declaration issue or import path mismatch
- **Fix**: `mcp/options/v1/options.proto` must declare its own `go_package`. Do not use Easyp `go_package_prefix` overrides for this package.

## Debugging Tips

1. **Inspect generated schema**: Find `<Service>_<Method>_ToolSpecInputSchemaJSON` constant in `*.mcp.go`, paste JSON into a formatter
2. **Trace schema generation**: Add logging in `internal/schema/schema.go` `GenerateFieldSchema()` for the specific field type
3. **Test a single schema**: `go test ./internal/testproto/example/v1/ -run TestSchema -v`
4. **Test stdio interaction**: `go test ./internal/examplemcp/ -run TestStdio -v`
5. **Validate easyp configs**: `easyp --cfg easyp.yaml validate-config` and `easyp --cfg easyp.test.yaml validate-config`
