import type { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { ErrorCode, GetPromptRequestSchema, ListPromptsRequestSchema, McpError } from "@modelcontextprotocol/sdk/types.js";
import type { GetPromptResult, Prompt, PromptArgument } from "@modelcontextprotocol/sdk/types.js";

import { normalizeToolSegment } from "./tools.js";

export type RegisteredPrompt = {
  prompt: Prompt & { arguments: PromptArgument[] };
  handler: (args: Record<string, string>) => Promise<GetPromptResult> | GetPromptResult;
};

export type ServerPromptRegistry = Map<string, RegisteredPrompt>;

const promptRegistrySymbol: unique symbol = Symbol.for("protoc-gen-mcp.typescript.promptRegistry") as never;

export function installPromptHandlers(server: Server): ServerPromptRegistry {
  const holder = server as Server & { [promptRegistrySymbol]?: ServerPromptRegistry };
  const existing = holder[promptRegistrySymbol];
  if (existing !== undefined) {
    return existing;
  }
  const registry: ServerPromptRegistry = new Map();
  holder[promptRegistrySymbol] = registry;
  server.registerCapabilities({ prompts: {} });
  server.setRequestHandler(ListPromptsRequestSchema, async () => ({
    prompts: [...registry.values()].map((entry) => entry.prompt),
  }));
  server.setRequestHandler(GetPromptRequestSchema, async (request) => {
    const requestedName = request.params.name;
    const entry = registry.get(requestedName);
    if (entry === undefined) {
      throw new McpError(ErrorCode.InvalidParams, `unknown prompt '${requestedName}'`);
    }
    const args: Record<string, string> = { ...(request.params.arguments ?? {}) };
    for (const arg of entry.prompt.arguments) {
      if (arg.required && !(arg.name in args)) {
        throw new McpError(ErrorCode.InvalidParams, `missing required prompt argument: ${arg.name}`);
      }
    }
    return await entry.handler(args);
  });
  return registry;
}

export function addRegisteredPrompt(registry: ServerPromptRegistry, entry: RegisteredPrompt): void {
  if (registry.has(entry.prompt.name)) {
    throw new Error(`duplicate prompt registration: ${entry.prompt.name}`);
  }
  registry.set(entry.prompt.name, entry);
}

export function promptName(namespace: string | undefined, promptName: string): string {
  const normalizedNamespace = normalizeToolSegment(namespace ?? "");
  const normalizedPromptName = normalizeToolSegment(promptName);
  if (normalizedNamespace === "") {
    return normalizedPromptName;
  }
  if (normalizedPromptName === "") {
    return normalizedNamespace;
  }
  return `${normalizedNamespace}_${normalizedPromptName}`;
}
