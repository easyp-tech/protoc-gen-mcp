import Foundation
import MCP
import SwiftProtobuf

/// Field facts that swift-protobuf does not expose at run time, generated per
/// message so ProtoJSON output matches protojson with EmitDefaultValues: every
/// field without presence is printed even when it holds its default.
public struct ProtoJSONField: Sendable {
    /// How a message-typed field holds its messages.
    public enum Cardinality: Sendable {
        case single
        case list
        case map
    }

    public let jsonName: String
    /// Printed when the field is absent; nil for fields with presence.
    public let defaultValue: Value?
    /// Message type whose fields are completed inside this field's value.
    public let nestedType: String?
    public let cardinality: Cardinality

    public init(_ jsonName: String, default defaultValue: Value? = nil, nested nestedType: String? = nil, _ cardinality: Cardinality = .single) {
        self.jsonName = jsonName
        self.defaultValue = defaultValue
        self.nestedType = nestedType
        self.cardinality = cardinality
    }
}

/// Generated field tables keyed by fully qualified message name.
public struct ProtoJSONShapes: Sendable {
    public let messages: [String: [ProtoJSONField]]

    public init(_ messages: [String: [ProtoJSONField]]) {
        self.messages = messages
    }

    /// Adds the default of every absent field without presence, recursively.
    func complete(_ value: Value, type: String) -> Value {
        guard case .object(var object) = value, let fields = messages[type] else {
            return value
        }
        for field in fields {
            if let present = object[field.jsonName] {
                guard let nested = field.nestedType else { continue }
                switch (field.cardinality, present) {
                case (.list, .array(let items)):
                    object[field.jsonName] = .array(items.map { complete($0, type: nested) })
                case (.map, .object(let entries)):
                    object[field.jsonName] = .object(entries.mapValues { complete($0, type: nested) })
                case (.single, _):
                    object[field.jsonName] = complete(present, type: nested)
                default:
                    break
                }
            } else if let defaultValue = field.defaultValue {
                object[field.jsonName] = defaultValue
            }
        }
        return .object(object)
    }
}

/// Encodes message as ProtoJSON with defaults completed for type.
public func protoJSON(_ message: some SwiftProtobuf.Message, type: String, shapes: ProtoJSONShapes) throws -> Value {
    let data = try message.jsonUTF8Data()
    return shapes.complete(try jsonValue(data), type: type)
}

/// Makes generated messages resolvable inside google.protobuf.Any ProtoJSON,
/// like the type registry the other runtimes build from file descriptors.
/// Well-known types are registered by SwiftProtobuf itself.
public func registerAnyMessageTypes(_ types: [any SwiftProtobuf.Message.Type]) {
    for type in types {
        _ = Google_Protobuf_Any.register(messageType: type)
    }
}
