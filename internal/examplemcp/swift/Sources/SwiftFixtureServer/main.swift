import Foundation
import MCP
import ProtocGenMCPRuntime

// Mode selection mirrors the JVM fixtures: no argument serves the example
// tools over stdio, "resources" the resources and prompts, "http" config/v1.
let mode = CommandLine.arguments.dropFirst().first ?? ""
switch mode {
case "resources":
    try await runResourcesServer()
case "http":
    try await runHTTPServer()
default:
    try await runExampleServer()
}

func runExampleServer() async throws {
    let server = Server(name: "protoc-gen-mcp-swift-example-server", version: "v0.0.1")
    try await registerExampleAPITools(server, ExampleHandler(), namespace: "example")
    try await serve(server, transport: StdioTransport())
    await server.waitUntilCompleted()
}

func runResourcesServer() async throws {
    let server = try await newResourcesMCPServer(ResourcesMCPHandlers(resources: ResourcesHandler()))
    try await registerPromptsPrompts(server, PromptsHandler())
    try await serve(server, transport: StdioTransport())
    await server.waitUntilCompleted()
}
