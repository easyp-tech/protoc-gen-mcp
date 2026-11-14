import Foundation
import MCP
import SwiftProtobuf

/// A static resource: its listing and how to read it.
public struct RegisteredStaticResource: Sendable {
    public let resource: Resource
    let read: @Sendable (String) async throws -> Resource.Content

    public init(resource: Resource, read: @escaping @Sendable (String) async throws -> Resource.Content) {
        self.resource = resource
        self.read = read
    }
}

/// A resource template: its listing, matcher, lister and reader.
public struct RegisteredResourceTemplate: Sendable {
    public let template: Resource.Template
    let matcher: URITemplate
    let list: @Sendable () async throws -> [Resource]
    let read: @Sendable (String, [String: String]) async throws -> Resource.Content

    public init(
        template: Resource.Template,
        list: @escaping @Sendable () async throws -> [Resource],
        read: @escaping @Sendable (String, [String: String]) async throws -> Resource.Content
    ) throws {
        self.template = template
        self.matcher = try URITemplate(template.uriTemplate)
        self.list = list
        self.read = read
    }
}

/// Registers generated resources on server, installing the resources
/// handlers once. Unknown URIs answer -32602 like the Go runtime.
public func addResources(
    _ server: Server,
    statics: [RegisteredStaticResource],
    templates: [RegisteredResourceTemplate]
) async throws {
    let registry = registry(for: server)
    for entry in statics {
        try registry.add(staticResource: entry)
    }
    for entry in templates {
        try registry.add(template: entry)
    }
    guard registry.markInstalled("resources") else { return }
    await server.protocGenMCPEnable(resources: true)
    await server.withMethodHandler(ListResources.self) { _ in
        var resources = registry.staticResources.map(\.resource)
        for template in registry.resourceTemplates {
            resources.append(contentsOf: try await template.list())
        }
        return ListResources.Result(resources: resources)
    }
    await server.withMethodHandler(ListResourceTemplates.self) { _ in
        ListResourceTemplates.Result(templates: registry.resourceTemplates.map(\.template))
    }
    await server.withMethodHandler(ReadResource.self) { params in
        let uri = params.uri
        if let entry = registry.staticResources.first(where: { $0.resource.uri == uri }) {
            return ReadResource.Result(contents: [try await entry.read(uri)])
        }
        for template in registry.resourceTemplates {
            if let values = try template.matcher.match(uri) {
                return ReadResource.Result(contents: [try await template.read(uri, values)])
            }
        }
        throw MCPError.invalidParams("Resource not found: \(uri)")
    }
}

/// Builds a resource or template listing from its generated descriptor,
/// naming it after the optional namespace.
public func resourceListing<T: Decodable>(_ type: T.Type, _ descriptorJSON: String, namespace: String?, name: String) throws -> T {
    guard case .object(var listing) = try jsonValue(descriptorJSON) else {
        throw RuntimeError("mcpruntime: resource descriptor for \(name) must be a JSON object")
    }
    if let namespace, !namespace.isEmpty {
        listing["name"] = .string("\(namespace)_\(name)")
    } else {
        listing["name"] = .string(name)
    }
    return try decodeDescriptor(T.self, .object(listing))
}

/// A {name} URI template matched like the other runtimes: each variable is
/// one path segment, percent-decoded and non-empty.
public struct URITemplate: Sendable {
    let names: [String]
    let literals: [String]

    public init(_ template: String) throws {
        var names: [String] = []
        var literals: [String] = []
        var rest = Substring(template)
        while let open = rest.firstIndex(of: "{") {
            guard let close = rest[open...].firstIndex(of: "}") else {
                throw RuntimeError("mcpruntime: unterminated variable in uri template \(template)")
            }
            literals.append(String(rest[..<open]))
            names.append(String(rest[rest.index(after: open)..<close]))
            rest = rest[rest.index(after: close)...]
        }
        literals.append(String(rest))
        self.names = names
        self.literals = literals
    }

    /// The decoded variables when uri matches, nil when it does not.
    public func match(_ uri: String) throws -> [String: String]? {
        guard uri.hasPrefix(literals[0]) else { return nil }
        var rest = Substring(uri.dropFirst(literals[0].count))
        var values: [String: String] = [:]
        for (index, name) in names.enumerated() {
            let next = literals[index + 1]
            let raw: Substring
            if next.isEmpty {
                guard index == names.count - 1 else { return nil }
                raw = rest
                rest = ""
            } else {
                guard let range = rest.range(of: next) else { return nil }
                raw = rest[..<range.lowerBound]
                rest = rest[range.lowerBound...]
            }
            if raw.contains("/") {
                return nil
            }
            guard let decoded = String(raw).removingPercentEncoding else {
                throw MCPError.invalidParams("mcpruntime: parameter '\(name)' in uri '\(uri)': invalid percent-encoding")
            }
            if decoded.isEmpty {
                throw MCPError.invalidParams("mcpruntime: parameter '\(name)' has empty value in uri '\(uri)'")
            }
            values[name] = decoded
            if !next.isEmpty {
                rest = rest.dropFirst(next.count)
            }
        }
        return rest.isEmpty ? values : nil
    }
}

/// How a resource message becomes MCP content.
public enum ResourceEncoding: Sendable {
    /// The whole message as ProtoJSON text.
    case protojson(type: String, shapes: ProtoJSONShapes)
    /// One string field, by JSON name, as text.
    case textField(String)
    /// One bytes field, by JSON name, as a blob.
    case blobField(String)
}

/// Converts a handler's message into resource contents.
public func resourceContents(
    _ message: some SwiftProtobuf.Message,
    uri: String,
    mimeType: String,
    metaJSON: String?,
    encoding: ResourceEncoding
) throws -> Resource.Content {
    let meta = try metaJSON.map { try decodeDescriptor(Metadata.self, $0) }
    switch encoding {
    case .protojson(let type, let shapes):
        let text = try jsonString(try protoJSON(message, type: type, shapes: shapes))
        return .text(text, uri: uri, mimeType: mimeType, _meta: meta)
    case .textField(let field):
        guard case .object(let object) = try jsonValue(try message.jsonUTF8Data()) else {
            throw RuntimeError("mcpruntime: resource '\(uri)' did not encode as an object")
        }
        return .text(stringValue(object[field]) ?? "", uri: uri, mimeType: mimeType, _meta: meta)
    case .blobField(let field):
        guard case .object(let object) = try jsonValue(try message.jsonUTF8Data()) else {
            throw RuntimeError("mcpruntime: resource '\(uri)' did not encode as an object")
        }
        let blob = Data(base64Encoded: stringValue(object[field]) ?? "") ?? Data()
        return .binary(blob, uri: uri, mimeType: mimeType, _meta: meta)
    }
}

/// A raw resource body: text or bytes returned by a handler as is.
public enum RawResourceBody: Sendable {
    case text(String)
    case bytes(Data)
}

public func resourceContents(_ body: RawResourceBody, uri: String, mimeType: String, metaJSON: String?) throws -> Resource.Content {
    let meta = try metaJSON.map { try decodeDescriptor(Metadata.self, $0) }
    switch body {
    case .text(let text):
        return .text(text, uri: uri, mimeType: mimeType, _meta: meta)
    case .bytes(let data):
        return .binary(data, uri: uri, mimeType: mimeType, _meta: meta)
    }
}
