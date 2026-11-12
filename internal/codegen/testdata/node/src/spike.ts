import { fromJson, toJson, type JsonValue } from "@bufbuild/protobuf";
import { StructSchema } from "@bufbuild/protobuf/wkt";
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
  type CallToolResult,
  type Tool,
} from "@modelcontextprotocol/sdk/types.js";
import { Ajv2020 } from "ajv/dist/2020.js";

const inputSchema = {
  type: "object",
  properties: {
    label: { type: "string" },
  },
  required: ["label"],
  additionalProperties: false,
} satisfies Tool["inputSchema"];

const outputSchema = {
  type: "object",
  properties: {
    label: { type: "string" },
  },
  required: ["label"],
  additionalProperties: false,
} satisfies NonNullable<Tool["outputSchema"]>;

const ajv = new Ajv2020({ strict: false });
const validateInput = ajv.compile(inputSchema);
const validateOutput = ajv.compile(outputSchema);

const server = new Server(
  { name: "ts-spike", version: "0.0.0" },
  { capabilities: { tools: {} } },
);

server.setRequestHandler(ListToolsRequestSchema, async (): Promise<{ tools: Tool[] }> => ({
  tools: [
    {
      name: "spike_echo",
      description: "Raw schema echo spike",
      inputSchema,
      outputSchema,
    },
  ],
}));

server.setRequestHandler(CallToolRequestSchema, async (request): Promise<CallToolResult> => {
  const args = request.params.arguments ?? {};
  if (!validateInput(args)) {
    return {
      content: [{ type: "text", text: "invalid input" }],
      isError: true,
    };
  }

  const message = fromJson(StructSchema, args as JsonValue);
  const payload = toJson(StructSchema, message, {
    alwaysEmitImplicit: true,
  }) as Record<string, unknown>;

  if (!validateOutput(payload)) {
    throw new Error("output validation failed");
  }

  return {
    content: [{ type: "text", text: JSON.stringify(payload) }],
    structuredContent: payload,
  };
});

void server;
