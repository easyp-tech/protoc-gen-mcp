import Foundation
import MCP

/// Decodes a JSON document into an SDK value.
public func jsonValue(_ json: String) throws -> Value {
    try jsonValue(Data(json.utf8))
}

public func jsonValue(_ data: Data) throws -> Value {
    try JSONDecoder().decode(PlainJSON.self, from: data).value
}

/// Decodes like Value but keeps strings as strings: the SDK turns strings that
/// look like data URLs into .data, which would not round-trip byte for byte.
private struct PlainJSON: Decodable {
    let value: Value

    init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        if container.decodeNil() {
            value = .null
        } else if let bool = try? container.decode(Bool.self) {
            value = .bool(bool)
        } else if let int = try? container.decode(Int.self) {
            value = .int(int)
        } else if let double = try? container.decode(Double.self) {
            value = .double(double)
        } else if let string = try? container.decode(String.self) {
            value = .string(string)
        } else if let array = try? container.decode([PlainJSON].self) {
            value = .array(array.map(\.value))
        } else {
            value = .object(try container.decode([String: PlainJSON].self).mapValues(\.value))
        }
    }
}

/// Compact JSON without escaped slashes, as the other runtimes print it.
public func jsonData(_ value: Value) throws -> Data {
    let encoder = JSONEncoder()
    encoder.outputFormatting = [.withoutEscapingSlashes]
    return try encoder.encode(value)
}

public func jsonString(_ value: Value) throws -> String {
    String(decoding: try jsonData(value), as: UTF8.self)
}

/// Decodes a generated JSON descriptor into an SDK type such as Tool or Resource.
public func decodeDescriptor<T: Decodable>(_ type: T.Type, _ json: String) throws -> T {
    try JSONDecoder().decode(T.self, from: Data(json.utf8))
}

func decodeDescriptor<T: Decodable>(_ type: T.Type, _ value: Value) throws -> T {
    try JSONDecoder().decode(T.self, from: try jsonData(value))
}

/// The message a handler error should carry to the client.
func errorMessage(_ error: Error) -> String {
    if let localized = error as? LocalizedError, let description = localized.errorDescription {
        return description
    }
    return String(describing: error)
}
