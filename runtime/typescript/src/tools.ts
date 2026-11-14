import { createRegistry, fromJson, toJson } from "@bufbuild/protobuf";
import type { DescFile, DescMessage, JsonValue, MessageShape, Registry } from "@bufbuild/protobuf";
import type { Server } from "@modelcontextprotocol/sdk/server/index.js";
import type { RequestHandlerExtra } from "@modelcontextprotocol/sdk/shared/protocol.js";
import { CallToolRequestSchema, ErrorCode, ListToolsRequestSchema, McpError } from "@modelcontextprotocol/sdk/types.js";
import type {
  CallToolRequest,
  CallToolResult,
  ServerNotification,
  ServerRequest,
  Tool,
} from "@modelcontextprotocol/sdk/types.js";
import { Ajv2020 } from "ajv/dist/2020.js";
import type { ValidateFunction } from "ajv/dist/2020.js";

import { errorMessage } from "./util.js";

/** Context passed to generated tool handlers. */
export type ToolRequestContext = RequestHandlerExtra<ServerRequest, ServerNotification>;

export type ToolAnnotationsMetadata = {
  readOnlyHint?: boolean;
  destructiveHint?: boolean;
  idempotentHint?: boolean;
  openWorldHint?: boolean;
  title?: string;
};

export type ToolIconMetadata = {
  src: string;
  mimeType?: string;
  sizes?: string[];
  theme?: string;
};

export type ToolExecutionMetadata = {
  taskSupport: "optional" | "required";
};

/** One generated tool: MCP metadata, schemas, Protobuf-ES descriptors and handler. */
export type RegisteredTool = {
  name: string;
  title?: string;
  description?: string;
  handler: (ctx: ToolRequestContext, request: never) => unknown | Promise<unknown>;
  inputSchemaJson: string;
  outputSchemaJson: string;
  inputSchema: DescMessage;
  outputSchema: DescMessage;
  registry: Registry;
  validateInput: ValidateFunction;
  validateOutput: ValidateFunction;
  annotations?: ToolAnnotationsMetadata;
  icons: ToolIconMetadata[];
  execution?: ToolExecutionMetadata;
  meta?: Record<string, unknown>;
  requiredScopes?: string[];
};

type ServerToolRegistry = {
  tools: RegisteredTool[];
};

const toolRegistrySymbol: unique symbol = Symbol.for("protoc-gen-mcp.typescript.toolRegistry") as never;
const jsonSchemaValidator = new Ajv2020({ strict: false, allErrors: true });

/**
 * Registers generated tools on server. The first call installs tools/list and
 * tools/call; later calls from other generated files share the same registry.
 */
export function addTools(server: Server, tools: RegisteredTool[]): void {
  const registry = toolRegistry(server);
  for (const tool of tools) {
    if (registry.tools.some((existing) => existing.name === tool.name)) {
      throw new Error(`duplicate tool registration: ${tool.name}`);
    }
    registry.tools.push(tool);
  }
}

function toolRegistry(server: Server): ServerToolRegistry {
  const holder = server as Server & { [toolRegistrySymbol]?: ServerToolRegistry };
  const existing = holder[toolRegistrySymbol];
  if (existing !== undefined) {
    return existing;
  }
  const registry: ServerToolRegistry = { tools: [] };
  holder[toolRegistrySymbol] = registry;
  server.registerCapabilities({ tools: {} });
  server.assertCanSetRequestHandler("tools/list");
  server.assertCanSetRequestHandler("tools/call");
  server.setRequestHandler(ListToolsRequestSchema, async (): Promise<{ tools: Tool[] }> => ({
    tools: registry.tools.map(buildListTool),
  }));
  server.setRequestHandler(CallToolRequestSchema, async (request, extra): Promise<CallToolResult> =>
    dispatchToolCall(registry.tools, request, extra as ToolRequestContext),
  );
  return registry;
}

/** Returns the required scopes of every tool registered on server, by tool name. */
export function toolScopes(server: Server): Map<string, string[]> {
  const scopes = new Map<string, string[]>();
  const registry = (server as Server & { [toolRegistrySymbol]?: ServerToolRegistry })[toolRegistrySymbol];
  for (const tool of registry?.tools ?? []) {
    scopes.set(tool.name, [...(tool.requiredScopes ?? [])]);
  }
  return scopes;
}

/** Builds the tools/list entry of a registered tool. */
export function buildListTool(tool: RegisteredTool): Tool {
  const listed = {
    name: tool.name,
    inputSchema: loadSchema(tool.inputSchemaJson),
    outputSchema: loadSchema(tool.outputSchemaJson),
  } as Tool;
  if (tool.title !== undefined) {
    listed.title = tool.title;
  }
  if (tool.description !== undefined) {
    listed.description = tool.description;
  }
  if (tool.annotations !== undefined) {
    listed.annotations = tool.annotations;
  }
  if (tool.icons.length > 0) {
    listed.icons = tool.icons as Tool["icons"];
  }
  if (tool.execution !== undefined) {
    listed.execution = tool.execution;
  }
  if (tool.meta !== undefined) {
    listed._meta = tool.meta;
  }
  return listed;
}

function loadSchema(rawSchemaJson: string): Record<string, unknown> {
  const parsed: unknown = JSON.parse(rawSchemaJson);
  if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error("mcpruntime: generated schema JSON must be an object");
  }
  return parsed as Record<string, unknown>;
}

/** Compiles a generated JSON Schema with the shared Ajv 2020-12 validator. */
export function compileSchema(rawSchemaJson: string): ValidateFunction {
  return jsonSchemaValidator.compile(loadSchema(rawSchemaJson));
}

function assertValid(toolName: string, phase: "input" | "output", validate: ValidateFunction, value: unknown): void {
  if (validate(value)) {
    return;
  }
  const message = jsonSchemaValidator.errorsText(validate.errors);
  if (phase === "input") {
    invalidParams(toolName, message);
  }
  throw new Error(`mcpruntime: validate output for tool '${toolName}': ${message}`);
}

function invalidParams(toolName: string, error: unknown): never {
  throw new McpError(ErrorCode.InvalidParams, `invalid arguments for tool '${toolName}': ${errorMessage(error)}`);
}

function toolErrorResult(error: unknown): CallToolResult {
  return {
    content: [{ type: "text", text: errorMessage(error) }],
    isError: true,
  };
}

/** Validates, decodes and runs one tool call, then validates its output. */
export async function dispatchToolCall(
  tools: RegisteredTool[],
  request: CallToolRequest,
  extra: ToolRequestContext,
): Promise<CallToolResult> {
  const toolName = request.params.name;
  const tool = tools.find((candidate) => candidate.name === toolName);
  if (tool === undefined) {
    throw new McpError(ErrorCode.InvalidParams, `unknown tool '${toolName}'`);
  }
  if (tool.requiredScopes !== undefined && tool.requiredScopes.length > 0) {
    // Defence in depth behind the HTTP scope check; stdio has no token.
    const granted = extra.authInfo?.scopes;
    if (granted === undefined) {
      return toolErrorResult("permission denied: an authenticated token is required");
    }
    const missing = tool.requiredScopes.find((scope) => !granted.includes(scope));
    if (missing !== undefined) {
      return toolErrorResult(`permission denied: missing required scope "${missing}"`);
    }
  }
  const args = (request.params.arguments ?? {}) as JsonValue;
  assertValid(tool.name, "input", tool.validateInput, args);

  let parsedRequest: MessageShape<typeof tool.inputSchema>;
  try {
    parsedRequest = fromJson(tool.inputSchema, args, { registry: tool.registry });
  } catch (error) {
    invalidParams(tool.name, error);
  }

  let response: unknown;
  try {
    response = await tool.handler(extra, parsedRequest as never);
  } catch (error) {
    return toolErrorResult(error);
  }

  let payload: JsonValue;
  try {
    payload = toJson(tool.outputSchema, response as MessageShape<typeof tool.outputSchema>, {
      alwaysEmitImplicit: true,
      registry: tool.registry,
    }) as JsonValue;
  } catch (error) {
    throw new Error(`mcpruntime: marshal output for tool '${tool.name}': ${errorMessage(error)}`);
  }
  if (payload === null || typeof payload !== "object" || Array.isArray(payload)) {
    throw new Error(`mcpruntime: marshal output for tool '${tool.name}': generated output must be a JSON object`);
  }
  const structuredContent = payload as Record<string, unknown>;
  assertValid(tool.name, "output", tool.validateOutput, structuredContent);
  return {
    content: [{ type: "text", text: JSON.stringify(structuredContent) }],
    structuredContent,
  };
}

/** Builds a Protobuf-ES registry from generated file descriptors and their imports. */
export function buildProtoRegistry(...fileRegistries: DescFile[]): Registry {
  const collected: DescFile[] = [];
  const seen = new Set<string>();
  const visit = (file: DescFile): void => {
    const key = file.proto.name || file.name;
    if (seen.has(key)) {
      return;
    }
    seen.add(key);
    collected.push(file);
    for (const dependency of file.dependencies) {
      visit(dependency);
    }
  };
  for (const file of fileRegistries) {
    visit(file);
  }
  return createRegistry(...collected);
}

/** Joins a namespace and a method name into an MCP tool name without dots. */
export function resolveToolName(
  namespace: string | null | undefined,
  defaultName: string,
  defaultNamespace: string,
): string {
  const resolvedNamespace = normalizeToolSegment(namespace === null || namespace === undefined ? defaultNamespace : namespace);
  const resolvedName = normalizeToolSegment(defaultName);
  if (resolvedNamespace === "") {
    return resolvedName;
  }
  if (resolvedName === "") {
    return resolvedNamespace;
  }
  return `${resolvedNamespace}_${resolvedName}`;
}

export function normalizeToolSegment(segment: string | null | undefined): string {
  if (segment === null || segment === undefined) {
    return "";
  }
  return segment.trim().replace(/[.]+/g, "_").replace(/_+/g, "_").replace(/^_+|_+$/g, "");
}
