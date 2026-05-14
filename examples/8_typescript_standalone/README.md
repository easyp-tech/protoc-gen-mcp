# Standalone TypeScript MCP Server

This example is structured like a user-owned TypeScript Node project rather
than a repository fixture. It has its own `package.json`, `tsconfig.json`,
`easyp.yaml`, protobuf contract, generated TypeScript sources, and stdio
server.

The generated code is imported as normal local ESM modules:

```ts
import { registerNotebookAPITools } from "./generated/proto/notebook_mcp.js";
```

## Generate

```bash
make generate
```

`easyp.yaml` does two jobs:

- runs `@bufbuild/protoc-gen-es` with `target=ts` and `import_extension=js` to
  generate Protobuf-ES `_pb.ts` files
- runs `protoc-gen-mcp` with `lang=typescript` to generate MCP sidecars

The checked-in local repository version uses:

```yaml
command: ["go", "run", "../../cmd/protoc-gen-mcp"]
```

In an external project, replace that command with a released
`protoc-gen-mcp` binary or a pinned module entrypoint.

## Build And Run

```bash
make build
make run
```

The server exposes:

- `notebook_CreateNote`
- `notebook_SearchNotes`
- `notebook_Health`

Handlers implement generated Protobuf-ES types and the generated
`NotebookAPIToolHandler` interface from `src/generated/proto/notebook_mcp.ts`.
