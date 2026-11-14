import { requireBearerAuth } from "@modelcontextprotocol/sdk/server/auth/middleware/bearerAuth.js";
import type { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import express from "express";
import type { NextFunction, Request, Response } from "express";

import { JWKSTokenVerifier } from "./jwks.js";
import type { TokenVerifier } from "./jwks.js";
import { toolScopes } from "./tools.js";

export const PROTECTED_RESOURCE_METADATA_PATH = "/.well-known/oauth-protected-resource";
const MAX_REQUEST_BODY_BYTES = 4 << 20;

/** Public OAuth policy from `option (mcp.options.v1.server).oauth`. */
export type OAuthResourceServer = {
  resourceUrl: string;
  authorizationServers: string[];
  scopes?: string[];
  issuer?: string;
  audience?: string;
  jwksUrl?: string;
  mcpPath?: string;
};

/**
 * A Node request listener for node:http createServer or an express app. It is
 * typed structurally so consumers need no Node typings to import the runtime.
 */
export type HttpHandler = (req: object, res: object) => void;

type ResolvedOAuth = OAuthResourceServer & { mcpPath: string; metadataPath: string; metadataUrl: string; scopes: string[] };

function resolveOAuth(oauth: OAuthResourceServer): ResolvedOAuth {
  const resource = new URL(oauth.resourceUrl);
  if (resource.protocol !== "https:" || resource.host === "") {
    throw new Error(`mcpruntime: invalid HTTPS resource URL "${oauth.resourceUrl}"`);
  }
  if (oauth.authorizationServers.length === 0) {
    throw new Error("mcpruntime: at least one authorization server is required");
  }
  for (const server of oauth.authorizationServers) {
    const url = new URL(server);
    if (url.protocol !== "https:" || url.host === "") {
      throw new Error(`mcpruntime: invalid HTTPS authorization server "${server}"`);
    }
  }
  const mcpPath = oauth.mcpPath || "/mcp";
  if (!mcpPath.startsWith("/") || mcpPath === "/" || mcpPath.startsWith("/.well-known/")) {
    throw new Error(`mcpruntime: invalid MCP path "${mcpPath}"`);
  }
  const metadataPath = PROTECTED_RESOURCE_METADATA_PATH + resource.pathname.replace(/\/+$/, "");
  return {
    ...oauth,
    scopes: [...(oauth.scopes ?? [])],
    mcpPath,
    metadataPath,
    metadataUrl: `${resource.protocol}//${resource.host}${metadataPath}`,
  };
}

/**
 * Serves MCP over stateless Streamable HTTP, protected when oauth is set.
 * Ports mcpruntime.NewConfiguredHTTPHandler: the SDK authenticates bearer
 * tokens; the runtime adds RFC 9728 metadata with live scopes_supported and a
 * per-tool scope check that answers 403 insufficient_scope before any tool
 * runs. createServer builds a fresh Server per request, as the stateless SDK
 * transport requires. Without verifier, the built-in RS256 JWKS verifier is
 * built from the oauth issuer, audience and JWKS URL.
 */
export function newHttpHandler(createServer: () => Server, oauth?: OAuthResourceServer, verifier?: TokenVerifier): HttpHandler {
  const app = express();
  app.disable("x-powered-by");
  const scopesByTool = toolScopes(createServer());

  const serve = async (req: Request, res: Response): Promise<void> => {
    if (req.method !== "POST") {
      res.status(405).set("Allow", "POST").json({
        jsonrpc: "2.0",
        error: { code: -32000, message: "Method not allowed." },
        id: null,
      });
      return;
    }
    const server = createServer();
    const transport = new StreamableHTTPServerTransport({ sessionIdGenerator: undefined });
    res.on("close", () => {
      void transport.close();
      void server.close();
    });
    await server.connect(transport);
    await transport.handleRequest(req, res, req.body);
  };
  const parseJson = express.json({
    limit: MAX_REQUEST_BODY_BYTES,
    strict: true,
    type: () => true,
    verify: (req, _res, buffer) => {
      (req as Request & { rawBody?: Buffer }).rawBody = buffer;
    },
  });

  if (oauth === undefined) {
    app.all("/mcp", crossOriginProtection, parseJson, (req, res) => void serve(req, res));
    return app as unknown as HttpHandler;
  }

  const resolved = resolveOAuth(oauth);
  let tokenVerifier = verifier;
  if (tokenVerifier === undefined) {
    if (!resolved.jwksUrl) {
      throw new Error("mcpruntime: OAuth needs a token verifier or a JWKS URL");
    }
    tokenVerifier = new JWKSTokenVerifier({ jwksUrl: resolved.jwksUrl, issuer: resolved.issuer ?? "", audience: resolved.audience ?? "" });
  }

  const metadata = (req: Request, res: Response): void => {
    res.set({
      "Access-Control-Allow-Origin": "*",
      "Access-Control-Allow-Methods": "GET, OPTIONS",
      "Access-Control-Allow-Headers": "Content-Type",
    });
    if (req.method === "OPTIONS") {
      res.status(204).end();
      return;
    }
    if (req.method !== "GET") {
      res.status(405).type("text/plain").send("Method not allowed\n");
      return;
    }
    const scopes = new Set(resolved.scopes);
    for (const required of scopesByTool.values()) {
      required.forEach((scope) => scopes.add(scope));
    }
    const document: Record<string, unknown> = { resource: resolved.resourceUrl };
    if (resolved.authorizationServers.length > 0) {
      document.authorization_servers = resolved.authorizationServers;
    }
    if (scopes.size > 0) {
      document.scopes_supported = [...scopes].sort();
    }
    res.status(200).json(document);
  };
  app.all(PROTECTED_RESOURCE_METADATA_PATH, metadata);
  if (resolved.metadataPath !== PROTECTED_RESOURCE_METADATA_PATH) {
    app.all(resolved.metadataPath, metadata);
  }
  app.all(`${PROTECTED_RESOURCE_METADATA_PATH}/*rest`, (_req, res) => {
    res.status(404).type("text/plain").send("404 page not found\n");
  });

  const scopeGate = (req: Request, res: Response, next: NextFunction): void => {
    const required = new Set(resolved.scopes);
    if (req.method === "POST") {
      const raw = (req as Request & { rawBody?: Buffer }).rawBody;
      try {
        if (raw !== undefined && raw.length > 0) {
          rejectDuplicateNames(raw.toString("utf8"));
        }
        for (const scope of requestedToolScopes(req.body, scopesByTool)) {
          required.add(scope);
        }
      } catch {
        res.status(400).type("text/plain").send("invalid MCP request\n");
        return;
      }
    }
    const granted = req.auth?.scopes ?? [];
    if ([...required].some((scope) => !granted.includes(scope))) {
      res.set({
        "WWW-Authenticate": `Bearer error="insufficient_scope", scope="${[...required].sort().join(" ")}", resource_metadata="${resolved.metadataUrl}"`,
        "Cache-Control": "no-store",
      });
      res.status(403).type("text/plain").send("insufficient scope\n");
      return;
    }
    next();
  };

  app.all(
    resolved.mcpPath,
    crossOriginProtection,
    requireBearerAuth({ verifier: tokenVerifier, resourceMetadataUrl: resolved.metadataUrl }),
    parseJson,
    scopeGate,
    (req, res) => void serve(req, res),
  );
  app.use((error: { status?: number; type?: string }, _req: Request, res: Response, _next: NextFunction) => {
    if (error.type === "entity.too.large" || error.status === 413) {
      res.status(413).type("text/plain").send("MCP request body too large\n");
      return;
    }
    res.status(error.status === undefined || error.status >= 500 ? 500 : 400).type("text/plain").send("invalid MCP request\n");
  });
  return app as unknown as HttpHandler;
}

function requestedToolScopes(body: unknown, scopesByTool: Map<string, string[]>): string[] {
  const messages = Array.isArray(body) ? body : [body];
  const scopes: string[] = [];
  for (const message of messages) {
    if (message === null || typeof message !== "object" || (message as { method?: unknown }).method !== "tools/call" || !("id" in message)) {
      continue;
    }
    const params = (message as { params?: unknown }).params;
    if (params === null || typeof params !== "object" || Array.isArray(params)) {
      throw new Error("tools/call params must be an object");
    }
    const name = (params as { name?: unknown }).name;
    if (typeof name !== "string") {
      throw new Error("tools/call name must be a string");
    }
    scopes.push(...(scopesByTool.get(name) ?? []));
  }
  return scopes;
}

/** Rejects a JSON object with a repeated "name" key, which JSON.parse would silently collapse. */
function rejectDuplicateNames(text: string): void {
  const stack: Array<Set<string> | null> = [];
  let index = 0;
  let expectKey = false;
  while (index < text.length) {
    const char = text[index];
    if (char === '"') {
      let end = index + 1;
      while (end < text.length && text[end] !== '"') {
        end += text[end] === "\\" ? 2 : 1;
      }
      const top = stack[stack.length - 1];
      if (expectKey && top) {
        const key = JSON.parse(text.slice(index, end + 1)) as string;
        if (key === "name" && top.has(key)) {
          throw new Error("ambiguous duplicate tool name");
        }
        top.add(key);
        expectKey = false;
      }
      index = end + 1;
      continue;
    }
    if (char === "{") {
      stack.push(new Set());
      expectKey = true;
    } else if (char === "[") {
      stack.push(null);
    } else if (char === "}" || char === "]") {
      stack.pop();
    } else if (char === ",") {
      expectKey = stack[stack.length - 1] !== null && stack.length > 0;
    }
    index += 1;
  }
}

/** Mirrors net/http CrossOriginProtection for state-changing requests. */
function crossOriginProtection(req: Request, res: Response, next: NextFunction): void {
  if (req.method === "GET" || req.method === "HEAD" || req.method === "OPTIONS") {
    next();
    return;
  }
  const site = req.get("sec-fetch-site");
  const origin = req.get("origin");
  const allowed = site !== undefined ? site === "same-origin" || site === "none" : origin === undefined || new URL(origin).host === req.get("host");
  if (!allowed) {
    res.status(403).type("text/plain").send("cross-origin request detected\n");
    return;
  }
  next();
}
