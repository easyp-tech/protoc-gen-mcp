import Foundation
import MCP
import ProtocGenMCPHTTP
import ProtocGenMCPRuntime

// Local development tokens for --demo-auth. Never accept static tokens in
// production: without --demo-auth the server verifies RS256 JWTs against the
// JWKS declared in notebook.proto.
let demoScopes = [
    "demo-read": ["notebook:read"],
    "demo-write": ["notebook:read", "notebook:write"],
]

var http = false
var demoAuth = false
var addr = "127.0.0.1:8080"
var arguments = CommandLine.arguments.dropFirst().makeIterator()
while let argument = arguments.next() {
    switch argument {
    case "--http": http = true
    case "--demo-auth": demoAuth = true
    case "--addr": addr = arguments.next() ?? addr
    default: throw RuntimeError("unknown argument: \(argument)")
    }
}

let notebook = Notebook()
let handlers = NotebookMCPHandlers(notebookAPI: notebook, prompts: notebook, resources: notebook)

if http {
    let separator = addr.lastIndex(of: ":") ?? addr.endIndex
    let host = String(addr[..<separator])
    let port = Int(addr[addr.index(after: separator)...]) ?? 8080
    if demoAuth, !["127.0.0.1", "localhost", "::1"].contains(host) {
        throw RuntimeError("--demo-auth only listens on loopback addresses")
    }
    let handler = try newNotebookMCPHTTPHandler(handlers, verifier: demoAuth ? DemoVerifier(scopes: demoScopes) : nil)
    try await serveHTTP(handler, host: host, port: port)
} else {
    let server = try await newNotebookMCPServer(handlers)
    try await serve(server, transport: StdioTransport())
    await server.waitUntilCompleted()
}

struct DemoVerifier: TokenVerifier {
    let scopes: [String: [String]]

    func verify(_ token: String) async throws -> AccessToken? {
        guard let granted = scopes[token] else { return nil }
        return AccessToken(token: token, clientID: "demo", scopes: granted, expiresAt: Date().addingTimeInterval(3600))
    }
}
