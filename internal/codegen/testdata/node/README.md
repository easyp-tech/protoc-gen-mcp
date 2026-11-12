# TypeScript test harness

Pinned Node package scope used by the `internal/codegen` TypeScript tests. They
create temporary projects inside this directory, compile generated `*_mcp.ts`
sidecars against the official `@modelcontextprotocol/sdk`, Protobuf-ES and Ajv
with strict NodeNext settings, and run the result over stdio.

```bash
npm ci
npm run typecheck
```
