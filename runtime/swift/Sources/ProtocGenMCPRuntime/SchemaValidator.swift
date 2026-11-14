import Foundation
import MCP

/// Validates instances against the JSON Schema 2020-12 subset that
/// protoc-gen-mcp generates: type, enum, const, properties, required,
/// additionalProperties, propertyNames, items, array/string/number bounds,
/// pattern, multipleOf, allOf/anyOf/oneOf/not and local $ref into $defs.
/// Like the validators of the other runtimes, format is an annotation only.
public struct SchemaValidator: Sendable {
    public let root: Value

    public init(schemaJSON: String) throws {
        let value = try JSONDecoder().decode(Value.self, from: Data(schemaJSON.utf8))
        guard case .object = value else {
            throw RuntimeError("mcpruntime: generated schema JSON must be an object")
        }
        root = value
    }

    /// Returns a description of the first violation, or nil when instance is valid.
    public func validate(_ instance: Value) -> String? {
        validate(instance, against: root, at: "")
    }

    private func validate(_ instance: Value, against schema: Value, at path: String) -> String? {
        switch schema {
        case .bool(let allowed):
            return allowed ? nil : "\(display(path)): no value is allowed here"
        case .object(let keywords):
            return validate(instance, keywords: keywords, at: path)
        default:
            return nil
        }
    }

    private func validate(_ instance: Value, keywords: [String: Value], at path: String) -> String? {
        if case .string(let ref)? = keywords["$ref"] {
            guard let target = resolve(ref) else {
                return "\(display(path)): unresolvable $ref \(ref)"
            }
            if let error = validate(instance, against: target, at: path) {
                return error
            }
        }
        if let types = keywords["type"], !matchesType(instance, types) {
            return "\(display(path)): expected type \(typeNames(types)), got \(typeName(instance))"
        }
        if case .array(let options)? = keywords["enum"], !options.contains(where: { jsonEqual($0, instance) }) {
            return "\(display(path)): value is not one of the allowed values"
        }
        if let expected = keywords["const"], !jsonEqual(expected, instance) {
            return "\(display(path)): value must be \(expected)"
        }
        if let error = validateObject(instance, keywords: keywords, at: path) { return error }
        if let error = validateArray(instance, keywords: keywords, at: path) { return error }
        if let error = validateString(instance, keywords: keywords, at: path) { return error }
        if let error = validateNumber(instance, keywords: keywords, at: path) { return error }
        if case .array(let schemas)? = keywords["allOf"] {
            for schema in schemas {
                if let error = validate(instance, against: schema, at: path) { return error }
            }
        }
        if case .array(let schemas)? = keywords["anyOf"],
            !schemas.contains(where: { validate(instance, against: $0, at: path) == nil })
        {
            return "\(display(path)): value does not match any allowed schema"
        }
        if case .array(let schemas)? = keywords["oneOf"] {
            let matches = schemas.filter { validate(instance, against: $0, at: path) == nil }.count
            if matches != 1 {
                return "\(display(path)): value must match exactly one schema, matched \(matches)"
            }
        }
        if let negated = keywords["not"], validate(instance, against: negated, at: path) == nil {
            return "\(display(path)): value matches a disallowed schema"
        }
        return nil
    }

    private func validateObject(_ instance: Value, keywords: [String: Value], at path: String) -> String? {
        guard case .object(let object) = instance else { return nil }
        if case .array(let required)? = keywords["required"] {
            for case .string(let name) in required where object[name] == nil {
                return "\(display(path)): missing required property \(name)"
            }
        }
        var properties: [String: Value] = [:]
        if case .object(let declared)? = keywords["properties"] {
            properties = declared
        }
        for (name, value) in object.sorted(by: { $0.key < $1.key }) {
            let childPath = path + "/" + name
            if let names = keywords["propertyNames"], let error = validate(.string(name), against: names, at: childPath) {
                return error
            }
            if let schema = properties[name] {
                if let error = validate(value, against: schema, at: childPath) { return error }
            } else if let additional = keywords["additionalProperties"] {
                if let error = validate(value, against: additional, at: childPath) {
                    if case .bool(false) = additional {
                        return "\(display(path)): unexpected property \(name)"
                    }
                    return error
                }
            }
        }
        return nil
    }

    private func validateArray(_ instance: Value, keywords: [String: Value], at path: String) -> String? {
        guard case .array(let items) = instance else { return nil }
        if let minimum = integer(keywords["minItems"]), items.count < minimum {
            return "\(display(path)): expected at least \(minimum) items"
        }
        if let maximum = integer(keywords["maxItems"]), items.count > maximum {
            return "\(display(path)): expected at most \(maximum) items"
        }
        if case .bool(true)? = keywords["uniqueItems"] {
            for (index, item) in items.enumerated() where items[..<index].contains(where: { jsonEqual($0, item) }) {
                return "\(display(path)): items must be unique"
            }
        }
        if let schema = keywords["items"] {
            for (index, item) in items.enumerated() {
                if let error = validate(item, against: schema, at: "\(path)/\(index)") { return error }
            }
        }
        return nil
    }

    private func validateString(_ instance: Value, keywords: [String: Value], at path: String) -> String? {
        guard let text = stringValue(instance) else { return nil }
        let length = text.unicodeScalars.count
        if let minimum = integer(keywords["minLength"]), length < minimum {
            return "\(display(path)): string shorter than \(minimum)"
        }
        if let maximum = integer(keywords["maxLength"]), length > maximum {
            return "\(display(path)): string longer than \(maximum)"
        }
        if case .string(let pattern)? = keywords["pattern"] {
            guard let regex = try? NSRegularExpression(pattern: pattern) else {
                return "\(display(path)): invalid pattern \(pattern)"
            }
            let range = NSRange(text.startIndex..<text.endIndex, in: text)
            if regex.firstMatch(in: text, range: range) == nil {
                return "\(display(path)): string does not match pattern \(pattern)"
            }
        }
        return nil
    }

    private func validateNumber(_ instance: Value, keywords: [String: Value], at path: String) -> String? {
        guard let number = numberValue(instance) else { return nil }
        if let minimum = numberValue(keywords["minimum"]), number < minimum {
            return "\(display(path)): must be >= \(minimum)"
        }
        if let maximum = numberValue(keywords["maximum"]), number > maximum {
            return "\(display(path)): must be <= \(maximum)"
        }
        if let minimum = numberValue(keywords["exclusiveMinimum"]), number <= minimum {
            return "\(display(path)): must be > \(minimum)"
        }
        if let maximum = numberValue(keywords["exclusiveMaximum"]), number >= maximum {
            return "\(display(path)): must be < \(maximum)"
        }
        if let divisor = numberValue(keywords["multipleOf"]), divisor > 0 {
            let quotient = number / divisor
            if abs(quotient - quotient.rounded()) > 1e-9 {
                return "\(display(path)): must be a multiple of \(divisor)"
            }
        }
        return nil
    }

    private func resolve(_ ref: String) -> Value? {
        if ref == "#" {
            return root
        }
        guard ref.hasPrefix("#/") else { return nil }
        var current = root
        for rawSegment in ref.dropFirst(2).split(separator: "/", omittingEmptySubsequences: false) {
            let segment = rawSegment.replacingOccurrences(of: "~1", with: "/").replacingOccurrences(of: "~0", with: "~")
            guard case .object(let object) = current, let next = object[segment] else { return nil }
            current = next
        }
        return current
    }

    private func matchesType(_ instance: Value, _ types: Value) -> Bool {
        switch types {
        case .string(let name):
            return matches(instance, typeName: name)
        case .array(let names):
            return names.contains { if case .string(let name) = $0 { return matches(instance, typeName: name) } else { return false } }
        default:
            return true
        }
    }

    private func matches(_ instance: Value, typeName name: String) -> Bool {
        switch (name, instance) {
        case ("null", .null), ("boolean", .bool), ("object", .object), ("array", .array):
            return true
        case ("string", .string), ("string", .data):
            return true
        case ("number", .int), ("number", .double), ("integer", .int):
            return true
        case ("integer", .double(let value)):
            return value.isFinite && value == value.rounded()
        default:
            return false
        }
    }

    private func typeName(_ instance: Value) -> String {
        switch instance {
        case .null: return "null"
        case .bool: return "boolean"
        case .int: return "integer"
        case .double: return "number"
        case .string, .data: return "string"
        case .array: return "array"
        case .object: return "object"
        }
    }

    private func typeNames(_ types: Value) -> String {
        if case .array(let names) = types {
            return names.map { "\($0)" }.joined(separator: " or ")
        }
        return "\(types)"
    }

    private func display(_ path: String) -> String {
        path.isEmpty ? "/" : path
    }
}

func stringValue(_ value: Value?) -> String? {
    switch value {
    case .string(let text)?:
        return text
    case .data(let mimeType, let data)?:
        return data.dataURLEncoded(mimeType: mimeType)
    default:
        return nil
    }
}

func numberValue(_ value: Value?) -> Double? {
    switch value {
    case .int(let number)?:
        return Double(number)
    case .double(let number)?:
        return number
    default:
        return nil
    }
}

private func integer(_ value: Value?) -> Int? {
    numberValue(value).map { Int($0) }
}

/// JSON equality: numbers compare by value, data URLs as their strings.
func jsonEqual(_ lhs: Value, _ rhs: Value) -> Bool {
    if let left = numberValue(lhs), let right = numberValue(rhs) {
        return left == right
    }
    if let left = stringValue(lhs), let right = stringValue(rhs) {
        return left == right
    }
    switch (lhs, rhs) {
    case (.null, .null):
        return true
    case (.bool(let left), .bool(let right)):
        return left == right
    case (.array(let left), .array(let right)):
        return left.count == right.count && zip(left, right).allSatisfy { jsonEqual($0, $1) }
    case (.object(let left), .object(let right)):
        return left.count == right.count && left.allSatisfy { key, value in right[key].map { jsonEqual(value, $0) } ?? false }
    default:
        return false
    }
}
