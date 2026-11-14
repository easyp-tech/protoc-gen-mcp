import Foundation
import Logging
import MCP

/// MCP Apps extension identifier, as in mcpruntime.AppExtensionID.
public let appExtensionID = "io.modelcontextprotocol/ui"

/// MCP Apps HTML MIME type, as in mcpruntime.AppHTMLMIMEType.
public let appHTMLMIMEType = "text/html;profile=mcp-app"

/// Mirrors mcpruntime.NewConfiguredServer: a server with the proto-declared
/// implementation info. Generated registrations enable the tools, prompts and
/// resources capabilities they need. Start it with ``serve(_:transport:)`` so
/// MCP Apps is advertised when apps is true.
public func newConfiguredServer(name: String, version: String, apps: Bool) throws -> Server {
    if name.isEmpty {
        throw RuntimeError("mcpruntime: configured server needs a name")
    }
    let server = Server(name: name, version: version.isEmpty ? "v0.0.1" : version)
    registry(for: server).apps = apps
    return server
}

/// Starts server on transport. When the server enables MCP Apps, the
/// transport is wrapped so initialize advertises capabilities.extensions,
/// which the SDK's typed capabilities cannot express.
public func serve(_ server: Server, transport: any Transport) async throws {
    if registry(for: server).apps {
        try await server.start(transport: AppsCapabilityTransport(wrapping: transport))
    } else {
        try await server.start(transport: transport)
    }
}

extension Server {
    func protocGenMCPEnable(tools: Bool = false, prompts: Bool = false, resources: Bool = false) {
        if tools, capabilities.tools == nil {
            capabilities.tools = .init(listChanged: false)
        }
        if prompts, capabilities.prompts == nil {
            capabilities.prompts = .init(listChanged: false)
        }
        if resources, capabilities.resources == nil {
            capabilities.resources = .init(subscribe: false, listChanged: false)
        }
    }
}

/// Forwards to another transport and adds the MCP Apps extension to the
/// capabilities of the initialize result.
public actor AppsCapabilityTransport: Transport {
    public nonisolated let logger: Logger
    private let inner: any Transport

    public init(wrapping inner: any Transport) {
        self.inner = inner
        self.logger = Logger(label: "protoc-gen-mcp.apps-capability")
    }

    public func connect() async throws {
        try await inner.connect()
    }

    public func disconnect() async {
        await inner.disconnect()
    }

    public func send(_ data: Data) async throws {
        try await inner.send(addAppsExtension(data))
    }

    public func receive() -> AsyncThrowingStream<Data, Swift.Error> {
        // receive() is synchronous in the protocol; hop to the inner actor.
        let inner = self.inner
        return AsyncThrowingStream { continuation in
            let task = Task {
                do {
                    for try await data in await inner.receive() {
                        continuation.yield(data)
                    }
                    continuation.finish()
                } catch {
                    continuation.finish(throwing: error)
                }
            }
            continuation.onTermination = { _ in task.cancel() }
        }
    }
}

/// Adds capabilities.extensions to an initialize result; other messages pass unchanged.
func addAppsExtension(_ data: Data) -> Data {
    guard case .object(var message)? = try? jsonValue(data),
        case .object(var result)? = message["result"],
        result["protocolVersion"] != nil, result["serverInfo"] != nil
    else {
        return data
    }
    var capabilities: [String: Value] = [:]
    if case .object(let existing)? = result["capabilities"] {
        capabilities = existing
    }
    var extensions: [String: Value] = [:]
    if case .object(let existing)? = capabilities["extensions"] {
        extensions = existing
    }
    extensions[appExtensionID] = .object(["mimeTypes": .array([.string(appHTMLMIMEType)])])
    capabilities["extensions"] = .object(extensions)
    result["capabilities"] = .object(capabilities)
    message["result"] = .object(result)
    return (try? jsonData(.object(message))) ?? data
}
