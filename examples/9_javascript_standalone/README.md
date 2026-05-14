# Standalone JavaScript MCP Server

This example proves JavaScript users can consume compiled output from the
TypeScript target. It does not use `lang=javascript` and does not generate a
second JavaScript-specific sidecar.

Instead, it imports compiled `.js` files and generated `.d.ts` metadata from
`../8_typescript_standalone/dist`:

```js
import { registerNotebookAPITools } from "../../8_typescript_standalone/dist/generated/proto/notebook_mcp.js";
```

The handwritten `src/server.js` uses `// @ts-check` and JSDoc imports so
TypeScript verifies the generated handler and protobuf types for plain
JavaScript code.

## Build And Run

```bash
make build
make run
```

`make build` first builds the TypeScript standalone example, then type-checks
this JavaScript server with `checkJs`.

The server exposes:

- `notebook_CreateNote`
- `notebook_SearchNotes`
- `notebook_Health`
