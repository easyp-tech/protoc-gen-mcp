import type { DescMessage, MessageShape, Registry } from "@bufbuild/protobuf";
import { toJson } from "@bufbuild/protobuf";
import { base64Encode } from "@bufbuild/protobuf/wire";
import type { Server } from "@modelcontextprotocol/sdk/server/index.js";
import {
  ErrorCode,
  ListResourceTemplatesRequestSchema,
  ListResourcesRequestSchema,
  McpError,
  ReadResourceRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";
import type { ReadResourceResult, Resource, ResourceTemplate } from "@modelcontextprotocol/sdk/types.js";

import { errorMessage } from "./util.js";

export type ResourceEncoding =
  | { kind: "protojson"; schema: DescMessage; registry: Registry }
  | { kind: "text_field" | "blob_field"; schema: DescMessage; field: string }
  | { kind: "raw" };

export type ResourceContentOptions = {
  mimeType: string;
  meta: Record<string, unknown> | undefined;
  encoding: ResourceEncoding;
};

export type ResourceContents = ReadResourceResult["contents"][number];

export type RegisteredStaticResource = {
  resource: Resource;
  read: (uri: string) => Promise<ResourceContents>;
};

export type RegisteredResourceTemplate = {
  template: ResourceTemplate;
  pattern: RegExp;
  names: string[];
  list: () => Promise<Resource[]>;
  read: (uri: string, params: Record<string, string>) => Promise<ResourceContents>;
};

export type ServerResourceRegistry = {
  statics: Map<string, RegisteredStaticResource>;
  templates: RegisteredResourceTemplate[];
};

const resourceRegistrySymbol: unique symbol = Symbol.for("protoc-gen-mcp.typescript.resourceRegistry") as never;

export function installResourceHandlers(server: Server): ServerResourceRegistry {
  const holder = server as Server & { [resourceRegistrySymbol]?: ServerResourceRegistry };
  const existing = holder[resourceRegistrySymbol];
  if (existing !== undefined) {
    return existing;
  }
  const registry: ServerResourceRegistry = { statics: new Map(), templates: [] };
  holder[resourceRegistrySymbol] = registry;
  server.registerCapabilities({ resources: {} });
  server.setRequestHandler(ListResourcesRequestSchema, async () => {
    const resources: Resource[] = [...registry.statics.values()].map((entry) => entry.resource);
    for (const entry of registry.templates) {
      resources.push(...(await entry.list()));
    }
    return { resources };
  });
  server.setRequestHandler(ListResourceTemplatesRequestSchema, async () => ({
    resourceTemplates: registry.templates.map((entry) => entry.template),
  }));
  server.setRequestHandler(ReadResourceRequestSchema, async (request): Promise<ReadResourceResult> => {
    const uri = request.params.uri;
    const entry = registry.statics.get(uri);
    if (entry !== undefined) {
      return { contents: [await entry.read(uri)] };
    }
    for (const template of registry.templates) {
      const params = matchUriTemplate(template, uri);
      if (params !== undefined) {
        return { contents: [await template.read(uri, params)] };
      }
    }
    throw new McpError(ErrorCode.InvalidParams, "Resource not found", { uri });
  });
  return registry;
}

export function addStaticResource(registry: ServerResourceRegistry, entry: RegisteredStaticResource): void {
  if (registry.statics.has(entry.resource.uri)) {
    throw new Error(`duplicate resource registration: ${entry.resource.uri}`);
  }
  registry.statics.set(entry.resource.uri, entry);
}

export function addResourceTemplate(registry: ServerResourceRegistry, entry: RegisteredResourceTemplate): void {
  if (registry.templates.some((existing) => existing.template.uriTemplate === entry.template.uriTemplate)) {
    throw new Error(`duplicate resource template registration: ${entry.template.uriTemplate}`);
  }
  registry.templates.push(entry);
}

export function resourceListing(descriptorJson: string, namespace: string | undefined, name: string): Record<string, unknown> {
  const descriptor = JSON.parse(descriptorJson) as Record<string, unknown>;
  descriptor.name = namespace ? `${namespace}_${name}` : name;
  return descriptor;
}

export function compileUriTemplate(uriTemplate: string): RegExp {
  const parts = uriTemplate.split(/\{[a-zA-Z_][a-zA-Z0-9_]*\}/);
  const escaped = parts.map((part) => part.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"));
  return new RegExp(`^${escaped.join("([^/]+)")}$`);
}

export function matchUriTemplate(entry: RegisteredResourceTemplate, uri: string): Record<string, string> | undefined {
  const match = entry.pattern.exec(uri);
  if (match === null) {
    return undefined;
  }
  const params: Record<string, string> = {};
  entry.names.forEach((name, index) => {
    const raw = match[index + 1] ?? "";
    let value: string;
    try {
      value = decodeURIComponent(raw);
    } catch (error) {
      throw new McpError(ErrorCode.InvalidParams, `mcpruntime: parameter '${name}' in uri '${uri}': ${errorMessage(error)}`, { uri });
    }
    if (value === "") {
      throw new McpError(ErrorCode.InvalidParams, `mcpruntime: parameter '${name}' has empty value in uri '${uri}'`, { uri });
    }
    params[name] = value;
  });
  return params;
}

export function resourceContents(value: unknown, uri: string, options: ResourceContentOptions): ResourceContents {
  if (value === null || value === undefined) {
    throw new Error(`mcpruntime: resource '${uri}' handler returned no value`);
  }
  const base: { uri: string; mimeType: string; _meta?: Record<string, unknown> } = { uri, mimeType: options.mimeType };
  if (options.meta !== undefined) {
    base._meta = options.meta;
  }
  const encoding = options.encoding;
  switch (encoding.kind) {
    case "raw":
      if (typeof value === "string") {
        return { ...base, text: value };
      }
      if (value instanceof Uint8Array) {
        return { ...base, blob: base64Encode(value) };
      }
      throw new Error(`mcpruntime: resource '${uri}' handler must return a string or Uint8Array`);
    case "protojson":
      return {
        ...base,
        text: JSON.stringify(toJson(encoding.schema, value as MessageShape<typeof encoding.schema>, {
          alwaysEmitImplicit: true,
          registry: encoding.registry,
        })),
      };
    default: {
      const field = encoding.schema.fields.find((candidate) => candidate.name === encoding.field);
      if (field === undefined) {
        throw new Error(`mcpruntime: resource content field '${encoding.field}' not found`);
      }
      const raw = (value as Record<string, unknown>)[field.localName];
      if (encoding.kind === "text_field") {
        return { ...base, text: typeof raw === "string" ? raw : "" };
      }
      return { ...base, blob: base64Encode(raw instanceof Uint8Array ? raw : new Uint8Array()) };
    }
  }
}
