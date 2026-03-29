---
name: protoc-gen-mcp
description: "Develop, test, and extend the protoc-gen-mcp protobuf-to-MCP code generator and runtime. Use when: modifying codegen, adding proto features, writing MCP tools, debugging schema generation, running easyp workflows, regenerating fixtures, updating golden snapshots, implementing new well-known types, adding field options, writing handler implementations."
---

# protoc-gen-mcp Development

Protobuf-first MCP tool generator and Go runtime. Converts annotated `.proto`
services into type-safe MCP tool bindings with JSON Schema validation.

## When to Use

- Modifying code generation logic or templates
- Adding support for new protobuf features or well-known types
- Writing or updating MCP field/method/service options
- Debugging JSON Schema generation or ProtoJSON round-trips
- Regenerating test fixtures or golden snapshots
- Implementing MCP tool handlers
- Working with easyp lint/generate workflows
- Reviewing or extending the runtime registration system

## Architecture Overview

```
.proto file ──→ easyp (protoc-gen-go + protoc-gen-mcp) ──→ *.pb.go + *.mcp.go
                                                                       │
                                                       ┌───────────────┘
                                                       ▼
                                        Register<Svc>Tools(server, impl)
                                                       │
                                                       ▼
                                            mcpruntime.RegisterProtoTool
                                              │  schema validation
                                              │  ProtoJSON marshal/unmarshal
                                              ▼
                                         MCP Server (stdio/SSE)
```

**Key modules:**

| Module | Role |
|--------|------|
| `cmd/protoc-gen-mcp` | Protoc plugin entrypoint |
| `internal/codegen` | Generator logic: collects specs, emits `*.mcp.go` |
| `internal/schema` | Descriptor → JSON Schema conversion |
| `mcpruntime` | Runtime: registration, validation, ProtoJSON handling |
| `mcp/options/v1` | Custom protobuf options (`ServiceOptions`, `MethodOptions`, `FieldOptions`, etc.) |
| `internal/testproto` | Test fixtures and generated code |
| `internal/examplemcp` | Reusable example server + stdio smoke tests |

## Essential Commands

```bash
# Lint shipped API
easyp --cfg easyp.yaml lint -p mcp -r .

# Generate shipped API (options.pb.go)
easyp --cfg easyp.yaml generate -p mcp -r .

# Lint test fixtures
easyp --cfg easyp.test.yaml lint -p internal/testproto -r .

# Generate test fixtures (*.mcp.go + *.pb.go)
easyp --cfg easyp.test.yaml generate -p internal/testproto -r .

# Run all tests
go test ./...

# Build plugin binary
go build ./cmd/protoc-gen-mcp

# Build & run example server
go build -o example-mcp-server ./cmd/example-mcp-server/main.go
./example-mcp-server
```

## Development Procedures

### Procedure 1: Modify Code Generation

1. Edit `internal/codegen/generator.go` or `internal/codegen/metadata.go`
2. If schema logic changes, also update `internal/schema/schema.go`
3. Regenerate test fixtures:
   ```bash
   easyp --cfg easyp.test.yaml generate -p internal/testproto -r .
   ```
4. Update golden snapshot — copy the regenerated file:
   ```bash
   cp internal/testproto/example/v1/example.mcp.go testdata/golden/example.mcp.go.golden
   ```
5. Run tests:
   ```bash
   go test ./...
   ```

### Procedure 2: Add a New Protobuf Feature or Well-Known Type

1. Add the type handling in `internal/schema/schema.go` (field → JSON Schema mapping)
2. If the type needs special codegen, update `internal/codegen/generator.go`
3. Add test coverage in `internal/testproto/example/v1/example.proto`:
   - Add fields to existing request/response messages
   - Or create a new RPC method for the feature
4. Regenerate + update golden:
   ```bash
   easyp --cfg easyp.test.yaml generate -p internal/testproto -r .
   cp internal/testproto/example/v1/example.mcp.go testdata/golden/example.mcp.go.golden
   ```
5. Add schema validation test in `internal/testproto/example/v1/example_schema_test.go`
6. If the feature affects the stdio test, update `internal/examplemcp/server.go` and `stdio_test.go`
7. Run full test suite: `go test ./...`
8. Update `AGENTS.md` Supported Features section

### Procedure 3: Add or Modify MCP Options

1. Edit `mcp/options/v1/options.proto`
2. Regenerate shipped API:
   ```bash
   easyp --cfg easyp.yaml generate -p mcp -r .
   ```
3. Update metadata extraction in `internal/codegen/metadata.go`
4. Update schema generation in `internal/schema/schema.go` if the option affects schemas
5. Add test proto fields using the new option in `internal/testproto/example/v1/example.proto`
6. Regenerate test fixtures + golden
7. Run: `go test ./...`

### Procedure 4: Write a New MCP Tool (End-User Flow)

1. Define service and messages in a `.proto` file:
   ```proto
   service MyService {
     option (mcp.options.v1.service) = { namespace: "my" };
     rpc DoThing(DoThingRequest) returns (DoThingResponse) {
       option (mcp.options.v1.method) = {
         title: "Do thing"
         description: "Does the thing."
       };
     };
   }
   ```
2. Annotate fields with `mcp.options.v1.field` for descriptions, examples, validation
3. Generate with easyp → produces `*.mcp.go`
4. Implement the `MyServiceToolHandler` interface
5. Register and serve:
   ```go
   server := mcp.NewServer(impl, nil)
   myservice.RegisterMyServiceTools(server, handler)
   server.Run(ctx, &mcp.StdioTransport{})
   ```

### Procedure 5: Debug Schema Generation Issues

1. Check the generated JSON Schema constant in `*.mcp.go`:
   - Look for `<Service>_<Method>_ToolSpecInputSchemaJSON`
2. Parse the JSON and validate structure:
   - `required` array matches expectations (singular non-optional fields)
   - Nullable fields have proper `type: ["<type>", "null"]` or `oneOf` with null
   - `$defs`/`$ref` present for recursive or nested messages
3. Check `internal/schema/schema.go` for the field type mapping
4. Run the specific schema test:
   ```bash
   go test ./internal/testproto/example/v1/ -run TestSchema
   ```
5. For runtime validation issues, check `mcpruntime/register.go` schema validation logic

## Key Invariants

### Requiredness Policy
| Proto Field Pattern | Required in MCP Schema? |
|---|---|
| `string name = 1` | YES (singular, non-optional) |
| `optional string name = 1` | NO |
| `repeated string names = 1` | NO |
| `map<string, string> m = 1` | NO |
| `oneof choice { ... }` | NO (unless `mcp.options.v1.oneof.required = true`) |

### Nullability
Non-required fields accept explicit JSON `null` in generated schemas.
This ensures MCP clients with cached `inputSchema` remain compatible with
ProtoJSON unset semantics.

### Tool Naming
- Pattern: `{namespace}_{MethodName}`
- Dots in namespace are normalized to underscores
- Example: namespace `example` + method `CreateReport` → `example_CreateReport`

### Fail-Fast Rules (Compile-Time Errors)
- Proto2 syntax → generation error
- Streaming RPC (client/server/bidi) → generation error
- Unsupported `google.protobuf.*` types → generation error

### JSON Schema Embedding
Schema JSON constants are emitted as **interpreted Go string literals**
(not raw backtick strings) so proto comments containing backticks don't
break generated code.

## Proto Options Quick Reference

```proto
import "mcp/options/v1/options.proto";

// Service-level
option (mcp.options.v1.service) = {
  namespace: "myapi"
  description: "My API tools."
};

// Method-level
option (mcp.options.v1.method) = {
  name: "CustomName"        // override tool method name
  title: "Human Title"
  description: "What this tool does."
  hidden: true              // exclude from tool list
  annotations: {
    read_only_hint: true
    destructive_hint: false
    idempotent_hint: true
  }
};

// Field-level
(mcp.options.v1.field) = {
  description: "Field purpose."
  examples: [{ string_value: "example" }]
  default_value: { number_value: 42 }
  pattern: "^[A-Z]"
  format: "email"
  min_length: 1
  max_length: 255
  minimum: 0
  maximum: 100
  min_items: 1
  max_items: 50
  unique_items: true
  read_only: true
};

// Message-level
option (mcp.options.v1.message) = {
  title: "My Message"
  description: "What this message represents."
};

// Enum-level
option (mcp.options.v1.enum) = { title: "Status" };

// Enum value: hide sentinel zero-value
UNSPECIFIED = 0 [(mcp.options.v1.enum_value) = { hidden: true }];

// Oneof-level
option (mcp.options.v1.oneof) = { required: true };
```

## Testing Matrix

| Test Layer | Location | What it Validates |
|---|---|---|
| Golden snapshot | `testdata/golden/` | Generated `*.mcp.go` stability |
| Schema validation | `example_schema_test.go` | JSON Schema correctness for all field types |
| Property tests | `internal/codegen/` (rapid) | Edge cases in metadata/property generation |
| Stdio smoke test | `internal/examplemcp/stdio_test.go` | End-to-end: spawn server, list tools, call tools, validate responses |

## Common Mistakes to Avoid

- **Don't regenerate with raw protoc** — always use `easyp` configs
- **Don't forget golden snapshot** after regenerating test fixtures
- **Don't add streaming RPCs** — MVP is unary-only, generator will reject them
- **Don't use `required` proto label** — requiredness is a schema policy, not proto label
- **Don't skip AGENTS.md updates** when changing features, layout, or commands
