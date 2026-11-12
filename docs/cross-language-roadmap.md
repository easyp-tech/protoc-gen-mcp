# Roadmap: protobuf-first MCP Apps, OAuth and resources in other languages

**Status:** proposal / implementation plan. **No dates or releases are
committed.** The Go implementation and runnable
[Go showcase](../examples/11_protobuf_first_mcp/README.md) are the reference
for behavioral contracts, not a guarantee that identical generated APIs
already exist for other languages.

This document describes parity targets, their dependencies and the tests
required before advertising support. It is not a list of features already
shipped.

## Verified baseline

| Capability | Go | Python | TypeScript | Java | Kotlin |
| --- | --- | --- | --- | --- | --- |
| Generated unary tools | Implemented | Implemented | Implemented | Implemented | Implemented |
| Generated prompts | Implemented | Implemented | Implemented | Implemented | Implemented |
| Resources: typed handler declarations | Implemented | Present | Present | Present | Present |
| Resources: full registration + read/templates | Implemented | **Not implemented** | **Not implemented** | **Not implemented** | **Not implemented** |
| Protobuf-first server factory | Implemented | Planned | Planned | Planned | Planned |
| `app_ui` tool link + UI resource metadata | Implemented | Planned | Planned | Planned | Planned |
| Embedded `source_file`, raw `content_field` | Implemented | Planned | Planned | Planned | Planned |
| Auto-negotiated MCP Apps capabilities | Implemented | Planned | Planned | Planned | Planned |
| OAuth protected resource + per-tool scopes | Implemented | Planned | Planned | Planned | Planned |
| JWT/JWKS verification and custom verifier | Implemented (RS256) | Planned | Planned | Planned | Planned |

The non-Go resource renderers still contain explicit TODO/stub code.
See `internal/codegen/render_python.go`,
`internal/codegen/render_typescript.go`,
`internal/codegen/render_java.go` and
`internal/codegen/render_kotlin.go`. A generated interface is not
functioning `resources/list`/`resources/read` support.

## Shared contract (before porting)

Use the **existing options** in
[`mcp/options/v1/options.proto`](../mcp/options/v1/options.proto) across
targets: `FileOptions.server`, `MethodOptions.app_ui`,
`MethodOptions.required_scopes`, `ResourceOptions.source_file`,
`ResourceOptions.content_field`, and `ResourceOptions.app_ui`.

Required cross-language behavior:

1. Protobuf is the **source of truth** for names, descriptions, generated
   JSON Schema, tool visibility and resource metadata. Avoid duplicating
   configuration in language-specific code or config files.
2. Emit the standard `Tool._meta.ui`, `resources/list` and
   `resources/read` metadata. Preserve CSP domain lists, permission
   requests, border preference, and optional sandbox origin without
   silently dropping fields.
3. `source_file` embeds or packages assets *inside the generated package*,
   never reads paths outside its boundary; raw text must not be wrapped as
   ProtoJSON. A bytes field becomes MCP binary content.
4. OAuth server-wide scopes and method-level `required_scopes` are both
   enforced, **fail closed**, and cannot be bypassed via an alternate
   generated transport entrypoint. JWT tokens validate signature, allowed
   algorithm(s), issuer, audience, expiration and scopes. Reject insecure
   HTTP JWKS or incorrect trust boundaries.
5. Keep OAuth Authorization Server, client secrets, UI frontend bundling,
   user storage and business logic **outside the generator**. Accept a
   provider-specific verifier when the built-in JWT/JWKS path is unsuitable.
6. Prefer official MCP SDKs over custom JSON-RPC/session/HTTP
   implementations. Each language may expose idiomatic APIs, but all must
   pass the same protocol fixtures.
7. Unsupported combinations must fail with an actionable error at
   generation time instead of emitting a successful-looking stub.

## Implementation sequence

### Foundation: cross-language fixtures and parity gate

Before shipping another target, reuse the Go showcase protobuf contract
to define language-neutral expectations for tools, prompts, JSON/raw/binary
resources, OAuth, and MCP Apps metadata. Create a shared conformance corpus:
known input/output JSON, expected resource URI/MIME, blob bytes, OAuth
challenges, and negative scope cases. Port the `source_file` package-boundary
checks to each generator and define how source paths are resolved.

**Done when:** a missing feature is classified explicitly as unsupported,
and every implemented renderer has contract tests proving that its
output matches these fixtures.

### 1. TypeScript / JavaScript

**Why first:** official `@modelcontextprotocol/sdk` is already used in the
repository, and Node is a natural fit for packaging frontend bundles.

- Implement resources (static and URI templates, list/read) using the
  official SDK, with ProtoJSON and raw text/blob conversion.
- Generate the equivalent server setup, tool `_meta.ui`,
  `Resource._meta.ui`, MCP Apps extension capabilities and scoped tool
  handlers. Use the official MCP Apps extension semantics.
- Resolve `source_file` into a build-time asset or bundled import
  compatible with NodeNext and both TS and JS consumers; never assume a
  browser bundler is available inside the protoc plugin.
- Add Streamable HTTP OAuth middleware, RS256/JWKS verifier and custom
  verification hook; distinguish resource-wide and method-specific scopes.
- Cover generated `.js` + `.d.ts` consumption as well as direct TS.

**Exit gate:** launch a generated Node server, invoke it through a real MCP
SDK client over stdio/HTTP, verify Apps metadata, resources and both
authorized and unauthorized tool calls.

### 2. Python

**Why next:** the official Python MCP SDK already backs generated tools.
The important missing piece is real resource registration, not just types.

- Replace `NotImplementedError` in generated resource registration with
  correct SDK resource and resource-template handlers.
- Support both existing generated dataclass and raw protobuf handler
  styles; maintain Python package coexistence with `mcp.options`.
- Bundle `source_file` through package resources rather than importing a
  user filesystem path at runtime. Return raw text/blob appropriately.
- Emit declarative server setup and MCP Apps capabilities/UI metadata.
- Implement a standard ASGI/HTTP OAuth protected resource with scopes,
  JWKS-based JWT verification and a pluggable verifier; separate this
  from the identity provider.

**Exit gate:** a pip-installable example with isolated dependencies
serves the same fixtures; no resource stubs remain; full HTTP OAuth
positive/negative tests pass.

### 3. Kotlin and Java

**Why together:** both use JVM protobuf output and Gradle-based SDK
integration tests, but their handler APIs differ.

- Implement resource list/read/template registration against each
  language's official MCP SDK. Maintain type-safe handler contracts.
- Use classpath resources or packaging tasks for `source_file` and ensure
  reproducible JAR builds; do not rely on local working-directory paths.
- Generate factory/registration wiring, Apps capabilities, UI/CSP/permission
  metadata and scoped tools, including HTTP middleware and JWKS/custom
  verifiers as the SDKs permit.
- Keep the generated APIs idiomatic: Kotlin suspend handlers versus Java
  callback/future contracts where required by their SDK versions.

**Exit gate:** `installDist`/JAR execution of separate Java and Kotlin
examples passes stdio, HTTP OAuth, resource, and metadata conformance
tests in CI, not only compile checks.

## Delivery criteria for each language

Do not mark an item **implemented** in the README or release notes until:

- Generated bindings compile under the pinned official MCP SDK and runtime.
- The example can be started from a clean checkout with documented commands.
- An official MCP client can initialize, list/call tools, list/get prompts
  and list/read both static and templated resources.
- Tests verify exact raw Markdown, text, bytes, `_meta.ui`, CSP/permissions,
  extension capabilities and generated asset paths.
- Both anonymous access and insufficient scopes are denied; valid scopes
  succeed. Incorrect issuer, audience, algorithm, signature and expiration
  must fail. No production demo tokens or secrets are emitted.
- CI includes generated-artifact freshness and regression tests, not
  just golden-source snapshots. The README matrix is updated on the
  **same PR** that enables support.

## Scope and dependencies

The Go target is the reference implementation; other targets may use
different SDK versions and different APIs for resource metadata, bearer
middleware or MCP Apps extension negotiation. Each target needs a short
SDK compatibility audit **before** code changes. If an official SDK cannot
represent a required standard field, report the gap explicitly and
prefer narrow adapters rather than maintaining an independent MCP wire
protocol.

Frontend authoring and deployment, complete OAuth login/consent flows,
identity-provider user management and automatic installation of MCP
`SKILL.md` resources as host-native Skills are **not** part of this
cross-language parity roadmap. Those belong to the app build pipeline,
identity provider and host respectively.
