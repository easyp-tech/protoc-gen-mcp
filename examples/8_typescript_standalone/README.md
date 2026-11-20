# Standalone TypeScript MCP Server

This example is structured like a user-owned TypeScript Node project rather
than a repository fixture. It has its own `package.json`, `tsconfig.json`,
`easyp.yaml`, protobuf contract, generated TypeScript sources, and stdio
server.

The generated code is imported as normal local ESM modules:

```ts
import {
  createNotebookMCPHttpHandler,
  createNotebookMCPServer,
} from "./generated/proto/notebook_mcp.js";
```

`createNotebookMCPServer(handlers)` is the server factory generated from
`option (mcp.options.v1.server)`; it registers the file's tools, prompts and
resources. `createNotebookMCPHttpHandler(handlers, verifier?)` serves the same
handlers over stateless Streamable HTTP.

The generated sidecar imports `@easyp-tech/protoc-gen-mcp-runtime`, which
`package.json` takes from `file:../../runtime/typescript` (with
`install-links=true` in `.npmrc`) because the runtime is not on npm yet.
Build it first:

```bash
(cd ../../runtime/typescript && npm ci && npm run build)
```

Outside this repository, install the runtime tarball attached to the GitHub
release instead, for example
`npm install https://github.com/easyp-tech/protoc-gen-mcp/releases/download/v1.0.0/easyp-tech-protoc-gen-mcp-runtime-1.0.0.tgz`.

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
- `notebook_DeleteNote` (needs `notebook:write`)
- `notebook_Health`

Handlers implement generated Protobuf-ES types and the generated
`NotebookAPIToolHandler` interface from `src/generated/proto/notebook_mcp.ts`.

## OAuth Over HTTP

`option (mcp.options.v1.server)` in the proto makes the server an OAuth
protected resource over stateless Streamable HTTP: every request needs
`notebook:read`, and `notebook_DeleteNote` also needs `notebook:write`. Over stdio there is no
token, so `notebook_DeleteNote` is denied inside the call. Start the server with local
demonstration tokens (loopback only, never in production):

```bash
node dist/server.js --http --demo-auth
```

Then walk through discovery and the scope step-up:

```bash
curl -s http://127.0.0.1:8080/.well-known/oauth-protected-resource/mcp
for token in demo-read demo-write; do
  curl -si http://127.0.0.1:8080/mcp \
    -H "Authorization: Bearer $token" \
    -H 'Content-Type: application/json' \
    -H 'Accept: application/json, text/event-stream' \
    -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"notebook_DeleteNote","arguments":{"id":"note-1"}}}'
done
```

`demo-read` gets `403` with `WWW-Authenticate: Bearer error="insufficient_scope",
scope="notebook:read notebook:write"` before the handler runs; `demo-write` gets the
tool result. Without `--demo-auth` the server verifies RS256 JWTs against the
issuer and JWKS declared in the proto.
