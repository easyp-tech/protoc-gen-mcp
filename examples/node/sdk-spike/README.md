# TypeScript SDK Spike

This is a compile-only spike for the future generated TypeScript MCP runtime.
It proves that the selected stable Node stack can support the low-level server
seam that `protoc-gen-mcp` needs before the full renderer is implemented.

The spike intentionally uses:

- `@modelcontextprotocol/sdk` low-level `Server.setRequestHandler(...)`
- raw JSON Schema objects for `inputSchema` and `outputSchema`
- Ajv 2020 validation
- Protobuf-ES `fromJson(...)` and `toJson(...)`
- strict NodeNext TypeScript

It intentionally does not use SDK `registerTool(...)`, generated Zod schemas,
private SDK fields, or a direct JavaScript renderer.

## Commands

```bash
npm ci
npm run typecheck
```
