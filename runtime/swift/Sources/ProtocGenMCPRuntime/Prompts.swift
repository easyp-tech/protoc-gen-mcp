import Foundation
import MCP

/// One generated prompt: its listing and handler.
public struct RegisteredPrompt: Sendable {
    public let prompt: Prompt
    let handler: @Sendable ([String: String]) async throws -> GetPrompt.Result

    /// descriptorJSON holds the prompt listing except its name.
    public init(
        name: String,
        descriptorJSON: String,
        handler: @escaping @Sendable ([String: String]) async throws -> GetPrompt.Result
    ) throws {
        guard case .object(var listing) = try jsonValue(descriptorJSON) else {
            throw RuntimeError("mcpruntime: prompt descriptor for \(name) must be a JSON object")
        }
        listing["name"] = .string(name)
        self.prompt = try decodeDescriptor(Prompt.self, .object(listing))
        self.handler = handler
    }
}

/// Registers generated prompts on server, installing prompts/list and prompts/get once.
public func addPrompts(_ server: Server, _ prompts: [RegisteredPrompt]) async throws {
    let registry = registry(for: server)
    for prompt in prompts {
        try registry.add(prompt: prompt)
    }
    guard registry.markInstalled("prompts") else { return }
    await server.protocGenMCPEnable(prompts: true)
    await server.withMethodHandler(ListPrompts.self) { _ in
        ListPrompts.Result(prompts: registry.prompts.map(\.prompt))
    }
    await server.withMethodHandler(GetPrompt.self) { params in
        guard let entry = registry.prompts.first(where: { $0.prompt.name == params.name }) else {
            throw MCPError.invalidParams("unknown prompt '\(params.name)'")
        }
        let arguments = params.arguments ?? [:]
        for argument in entry.prompt.arguments ?? [] where argument.required == true && arguments[argument.name] == nil {
            throw MCPError.invalidParams("missing required prompt argument: \(argument.name)")
        }
        return try await entry.handler(arguments)
    }
}

/// Joins an optional namespace and a prompt name without dots.
public func promptName(_ namespace: String?, _ name: String) -> String {
    let resolvedNamespace = normalizeToolSegment(namespace ?? "")
    let resolvedName = normalizeToolSegment(name)
    if resolvedNamespace.isEmpty {
        return resolvedName
    }
    if resolvedName.isEmpty {
        return resolvedNamespace
    }
    return "\(resolvedNamespace)_\(resolvedName)"
}
