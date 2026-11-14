import { Server } from "@modelcontextprotocol/sdk/server/index.js";

export const APP_EXTENSION_ID = "io.modelcontextprotocol/ui";
export const APP_HTML_MIME_TYPE = "text/html;profile=mcp-app";

/**
 * Mirrors mcpruntime.NewConfiguredServer: a server with the proto-declared
 * implementation info and, when Apps is enabled, the MCP Apps extension.
 * Generated registrations add the tools, prompts and resources capabilities.
 */
export function newConfiguredServer(name: string, version: string, apps: boolean): Server {
  if (name === "") {
    throw new Error("mcpruntime: configured server needs a name");
  }
  const capabilities = apps ? { extensions: { [APP_EXTENSION_ID]: { mimeTypes: [APP_HTML_MIME_TYPE] } } } : {};
  return new Server({ name, version: version || "v0.0.1" }, { capabilities });
}
