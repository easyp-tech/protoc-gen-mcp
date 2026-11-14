import Foundation
import MCP
import SwiftProtobuf

/// One generated tool: its listing, schemas, required scopes and handler.
public struct RegisteredTool: Sendable {
    public let name: String
    public let listing: Tool
    public let requiredScopes: [String]
    let inputValidator: SchemaValidator
    let outputValidator: SchemaValidator
    let run: @Sendable (ToolContext, Data) async throws -> Value

    /// Builds a tool from generated metadata. descriptorJSON holds the listing
    /// fields except name and schemas (title, description, annotations, icons,
    /// _meta); outputType and shapes complete ProtoJSON defaults of the result.
    public init<Request: SwiftProtobuf.Message, Response: SwiftProtobuf.Message>(
        name: String,
        descriptorJSON: String,
        inputSchemaJSON: String,
        outputSchemaJSON: String,
        requiredScopes: [String] = [],
        request: Request.Type,
        outputType: String,
        shapes: ProtoJSONShapes,
        handler: @escaping @Sendable (ToolContext, Request) async throws -> Response
    ) throws {
        guard case .object(var listing) = try jsonValue(descriptorJSON) else {
            throw RuntimeError("mcpruntime: tool descriptor for \(name) must be a JSON object")
        }
        listing["name"] = .string(name)
        listing["inputSchema"] = try jsonValue(inputSchemaJSON)
        listing["outputSchema"] = try jsonValue(outputSchemaJSON)
        self.name = name
        self.listing = try decodeDescriptor(Tool.self, .object(listing))
        self.requiredScopes = requiredScopes
        self.inputValidator = try SchemaValidator(schemaJSON: inputSchemaJSON)
        self.outputValidator = try SchemaValidator(schemaJSON: outputSchemaJSON)
        self.run = { context, arguments in
            let decoded: Request
            do {
                decoded = try Request(jsonUTF8Data: arguments)
            } catch {
                throw ToolInputError(message: errorMessage(error))
            }
            let response = try await handler(context, decoded)
            return try encodeOutput { try protoJSON(response, type: outputType, shapes: shapes) }
        }
    }
}

struct ToolInputError: Error {
    let message: String
}

/// Marks errors raised while encoding a handler's result, which are server
/// bugs rather than tool failures.
struct ToolOutputError: Error {
    let message: String
}

@inline(__always)
private func encodeOutput(_ encode: () throws -> Value) throws -> Value {
    do {
        return try encode()
    } catch {
        throw ToolOutputError(message: errorMessage(error))
    }
}

/// Registers generated tools on server. The first call installs tools/list
/// and tools/call and enables the tools capability; later calls from other
/// generated files share the same registry.
public func addTools(_ server: Server, _ tools: [RegisteredTool]) async throws {
    let registry = registry(for: server)
    for tool in tools {
        try registry.add(tool: tool)
    }
    guard registry.markInstalled("tools") else { return }
    await server.protocGenMCPEnable(tools: true)
    await server.withMethodHandler(ListTools.self) { _ in
        ListTools.Result(tools: registry.tools.map(\.listing))
    }
    await server.withMethodHandler(CallTool.self) { params in
        try await dispatchToolCall(registry, params)
    }
}

/// Required scopes of every tool registered on server, by tool name.
public func toolScopes(_ server: Server) -> [String: [String]] {
    var scopes: [String: [String]] = [:]
    for tool in registry(for: server).tools {
        scopes[tool.name] = tool.requiredScopes
    }
    return scopes
}

/// Checks scopes, validates, decodes and runs one tool call, then validates its output.
func dispatchToolCall(_ registry: ServerRegistry, _ params: CallTool.Parameters) async throws -> CallTool.Result {
    guard let tool = registry.tool(named: params.name) else {
        throw MCPError.invalidParams("unknown tool '\(params.name)'")
    }
    let token = registry.accessToken
    // Defence in depth behind the HTTP scope check; stdio has no token.
    if let denied = toolGuardError(requiredScopes: tool.requiredScopes, token: token) {
        return CallTool.Result(content: [.text(text: denied, annotations: nil, _meta: nil)], isError: true)
    }
    let arguments = Value.object(params.arguments ?? [:])
    if let violation = tool.inputValidator.validate(arguments) {
        throw invalidParams(tool.name, violation)
    }
    let structured: Value
    do {
        structured = try await tool.run(ToolContext(accessToken: token), try jsonData(arguments))
    } catch let error as ToolInputError {
        throw invalidParams(tool.name, error.message)
    } catch let error as ToolOutputError {
        throw MCPError.internalError("mcpruntime: marshal output for tool '\(tool.name)': \(error.message)")
    } catch {
        return CallTool.Result(content: [.text(text: errorMessage(error), annotations: nil, _meta: nil)], isError: true)
    }
    guard case .object = structured else {
        throw MCPError.internalError("mcpruntime: marshal output for tool '\(tool.name)': generated output must be a JSON object")
    }
    if let violation = tool.outputValidator.validate(structured) {
        throw MCPError.internalError("mcpruntime: validate output for tool '\(tool.name)': \(violation)")
    }
    return CallTool.Result(
        content: [.text(text: try jsonString(structured), annotations: nil, _meta: nil)],
        structuredContent: Optional(structured)
    )
}

private func invalidParams(_ tool: String, _ reason: String) -> MCPError {
    MCPError.invalidParams("invalid arguments for tool '\(tool)': \(reason)")
}

/// Joins a namespace and a method name into an MCP tool name without dots.
public func toolName(_ namespace: String?, _ name: String, defaultNamespace: String) -> String {
    let resolvedNamespace = normalizeToolSegment(namespace ?? defaultNamespace)
    let resolvedName = normalizeToolSegment(name)
    if resolvedNamespace.isEmpty {
        return resolvedName
    }
    if resolvedName.isEmpty {
        return resolvedNamespace
    }
    return "\(resolvedNamespace)_\(resolvedName)"
}

public func normalizeToolSegment(_ segment: String?) -> String {
    guard let segment else { return "" }
    return segment.trimmingCharacters(in: .whitespacesAndNewlines)
        .replacingOccurrences(of: ".", with: "_")
        .split(separator: "_", omittingEmptySubsequences: true)
        .joined(separator: "_")
}
