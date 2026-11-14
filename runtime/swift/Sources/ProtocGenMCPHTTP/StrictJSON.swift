import Foundation

/// A strict JSON reader for MCP request bodies: exactly one value, no
/// trailing data, and no repeated "name" key in an object, which a lenient
/// parser would silently collapse.
enum StrictJSON {
    indirect enum Node {
        case object([(String, Node)])
        case array([Node])
        case string(String)
        case other

        subscript(key: String) -> Node? {
            guard case .object(let pairs) = self else { return nil }
            return pairs.last { $0.0 == key }?.1
        }

        func has(_ key: String) -> Bool {
            guard case .object(let pairs) = self else { return false }
            return pairs.contains { $0.0 == key }
        }
    }

    struct Failure: Error {
        let reason: String
    }

    static func parse(_ data: Data) throws -> Node {
        var parser = Parser(bytes: Array(data))
        parser.skipWhitespace()
        let node = try parser.value()
        parser.skipWhitespace()
        guard parser.index == parser.bytes.count else {
            throw Failure(reason: "unexpected data after the JSON value")
        }
        return node
    }

    private struct Parser {
        let bytes: [UInt8]
        var index = 0

        mutating func skipWhitespace() {
            while index < bytes.count, [0x20, 0x09, 0x0A, 0x0D].contains(bytes[index]) {
                index += 1
            }
        }

        mutating func peek() throws -> UInt8 {
            guard index < bytes.count else { throw Failure(reason: "unexpected end of JSON") }
            return bytes[index]
        }

        mutating func next() throws -> UInt8 {
            let byte = try peek()
            index += 1
            return byte
        }

        mutating func expect(_ byte: UInt8) throws {
            guard try next() == byte else { throw Failure(reason: "expected \(Character(UnicodeScalar(byte)))") }
        }

        mutating func value() throws -> Node {
            switch try peek() {
            case UInt8(ascii: "{"): return try object()
            case UInt8(ascii: "["): return try array()
            case UInt8(ascii: "\""): return .string(try string())
            case UInt8(ascii: "t"): try literal("true"); return .other
            case UInt8(ascii: "f"): try literal("false"); return .other
            case UInt8(ascii: "n"): try literal("null"); return .other
            default: try number(); return .other
            }
        }

        mutating func object() throws -> Node {
            index += 1
            var pairs: [(String, Node)] = []
            skipWhitespace()
            if try peek() == UInt8(ascii: "}") {
                index += 1
                return .object(pairs)
            }
            while true {
                skipWhitespace()
                guard try peek() == UInt8(ascii: "\"") else { throw Failure(reason: "expected an object key") }
                let key = try string()
                if key == "name", pairs.contains(where: { $0.0 == "name" }) {
                    throw Failure(reason: "ambiguous duplicate tool name")
                }
                skipWhitespace()
                try expect(UInt8(ascii: ":"))
                skipWhitespace()
                pairs.append((key, try value()))
                skipWhitespace()
                switch try next() {
                case UInt8(ascii: "}"): return .object(pairs)
                case UInt8(ascii: ","): continue
                default: throw Failure(reason: "expected , or } in object")
                }
            }
        }

        mutating func array() throws -> Node {
            index += 1
            var items: [Node] = []
            skipWhitespace()
            if try peek() == UInt8(ascii: "]") {
                index += 1
                return .array(items)
            }
            while true {
                skipWhitespace()
                items.append(try value())
                skipWhitespace()
                switch try next() {
                case UInt8(ascii: "]"): return .array(items)
                case UInt8(ascii: ","): continue
                default: throw Failure(reason: "expected , or ] in array")
                }
            }
        }

        mutating func string() throws -> String {
            try expect(UInt8(ascii: "\""))
            var scalars = String.UnicodeScalarView()
            var raw: [UInt8] = []
            func flush() throws {
                guard !raw.isEmpty else { return }
                guard let text = String(bytes: raw, encoding: .utf8) else { throw Failure(reason: "invalid UTF-8 in string") }
                scalars.append(contentsOf: text.unicodeScalars)
                raw.removeAll()
            }
            while true {
                let byte = try next()
                if byte == UInt8(ascii: "\"") {
                    try flush()
                    return String(scalars)
                }
                if byte < 0x20 {
                    throw Failure(reason: "control character in string")
                }
                if byte != UInt8(ascii: "\\") {
                    raw.append(byte)
                    continue
                }
                try flush()
                let escape = try next()
                switch escape {
                case UInt8(ascii: "\""), UInt8(ascii: "\\"), UInt8(ascii: "/"):
                    scalars.append(UnicodeScalar(escape))
                case UInt8(ascii: "b"): scalars.append("\u{08}")
                case UInt8(ascii: "f"): scalars.append("\u{0C}")
                case UInt8(ascii: "n"): scalars.append("\n")
                case UInt8(ascii: "r"): scalars.append("\r")
                case UInt8(ascii: "t"): scalars.append("\t")
                case UInt8(ascii: "u"):
                    guard index + 4 <= bytes.count,
                        let code = UInt32(String(decoding: bytes[index..<index + 4], as: UTF8.self), radix: 16)
                    else {
                        throw Failure(reason: "invalid unicode escape")
                    }
                    index += 4
                    scalars.append(UnicodeScalar(code) ?? "\u{FFFD}")
                default:
                    throw Failure(reason: "invalid escape")
                }
            }
        }

        mutating func literal(_ word: String) throws {
            let expected = Array(word.utf8)
            guard index + expected.count <= bytes.count, Array(bytes[index..<index + expected.count]) == expected else {
                throw Failure(reason: "invalid literal")
            }
            index += expected.count
        }

        mutating func number() throws {
            let start = index
            while index < bytes.count, "+-0123456789.eE".utf8.contains(bytes[index]) {
                index += 1
            }
            guard index > start, Double(String(decoding: bytes[start..<index], as: UTF8.self)) != nil else {
                throw Failure(reason: "unexpected character in JSON")
            }
        }
    }
}
