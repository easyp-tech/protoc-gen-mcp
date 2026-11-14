import Foundation
import MCP

/// Everything protoc-gen-mcp registered on one SDK server. Several generated
/// files share it, so their tools, prompts and resources are served together.
public final class ServerRegistry: @unchecked Sendable {
    private let lock = NSLock()
    private var _tools: [RegisteredTool] = []
    private var _prompts: [RegisteredPrompt] = []
    private var _statics: [RegisteredStaticResource] = []
    private var _templates: [RegisteredResourceTemplate] = []
    private var _apps = false
    private var _accessToken: AccessToken?
    private var installed: Set<String> = []

    init() {}

    func withLock<T>(_ body: () throws -> T) rethrows -> T {
        lock.lock()
        defer { lock.unlock() }
        return try body()
    }

    /// Whether the server advertises the MCP Apps extension.
    public var apps: Bool {
        get { withLock { _apps } }
        set { withLock { _apps = newValue } }
    }

    /// The token of the HTTP request this server was built for; nil on stdio.
    public var accessToken: AccessToken? {
        get { withLock { _accessToken } }
        set { withLock { _accessToken = newValue } }
    }

    public var tools: [RegisteredTool] { withLock { _tools } }
    public var prompts: [RegisteredPrompt] { withLock { _prompts } }
    public var staticResources: [RegisteredStaticResource] { withLock { _statics } }
    public var resourceTemplates: [RegisteredResourceTemplate] { withLock { _templates } }

    func tool(named name: String) -> RegisteredTool? {
        withLock { _tools.first { $0.name == name } }
    }

    func add(tool: RegisteredTool) throws {
        try withLock {
            if _tools.contains(where: { $0.name == tool.name }) {
                throw RuntimeError("duplicate tool registration: \(tool.name)")
            }
            _tools.append(tool)
        }
    }

    func add(prompt: RegisteredPrompt) throws {
        try withLock {
            if _prompts.contains(where: { $0.prompt.name == prompt.prompt.name }) {
                throw RuntimeError("duplicate prompt registration: \(prompt.prompt.name)")
            }
            _prompts.append(prompt)
        }
    }

    func add(staticResource: RegisteredStaticResource) throws {
        try withLock {
            if _statics.contains(where: { $0.resource.uri == staticResource.resource.uri }) {
                throw RuntimeError("duplicate resource registration: \(staticResource.resource.uri)")
            }
            _statics.append(staticResource)
        }
    }

    func add(template: RegisteredResourceTemplate) throws {
        try withLock {
            if _templates.contains(where: { $0.template.uriTemplate == template.template.uriTemplate }) {
                throw RuntimeError("duplicate resource template registration: \(template.template.uriTemplate)")
            }
            _templates.append(template)
        }
    }

    /// Returns true the first time a group of handlers is installed.
    func markInstalled(_ group: String) -> Bool {
        withLock { installed.insert(group).inserted }
    }
}

private final class WeakServer: @unchecked Sendable {
    weak var server: Server?
    let registry: ServerRegistry

    init(server: Server, registry: ServerRegistry) {
        self.server = server
        self.registry = registry
    }
}

private let registriesLock = NSLock()
nonisolated(unsafe) private var registries: [ObjectIdentifier: WeakServer] = [:]

/// The registry of server, created on first use.
public func registry(for server: Server) -> ServerRegistry {
    registriesLock.lock()
    defer { registriesLock.unlock() }
    registries = registries.filter { $0.value.server != nil }
    let key = ObjectIdentifier(server)
    if let existing = registries[key], existing.server === server {
        return existing.registry
    }
    let created = ServerRegistry()
    registries[key] = WeakServer(server: server, registry: created)
    return created
}

/// An error raised by the runtime itself rather than by a handler.
public struct RuntimeError: Error, CustomStringConvertible, Sendable {
    public let description: String

    public init(_ description: String) {
        self.description = description
    }
}
