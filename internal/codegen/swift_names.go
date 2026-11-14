package codegen

import (
	"fmt"
	"strings"
	"unicode"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Swift names follow swift-protobuf's SwiftProtobufPluginLibrary NamingUtils,
// so generated sidecars refer to the types protoc-gen-swift emits.

var swiftKeywords = setOf(
	// Declarations.
	"associatedtype", "class", "deinit", "enum", "extension", "fileprivate",
	"func", "import", "init", "inout", "internal", "let", "open", "operator",
	"private", "protocol", "public", "static", "struct", "subscript",
	"typealias", "var",
	// Statements.
	"break", "case", "continue", "default", "defer", "do", "else",
	"fallthrough", "for", "guard", "if", "in", "repeat", "return", "switch",
	"where", "while",
	// Expressions and types.
	"as", "Any", "catch", "false", "is", "nil", "rethrows", "super", "self",
	"Self", "throw", "throws", "true", "try",
)

// swiftReservedTypeNames is reservedTypeNames from swift-protobuf NamingUtils.
var swiftReservedTypeNames = func() map[string]struct{} {
	names := setOf(
		"SwiftProtobuf", "Extensions", "protoMessageName", "decodeMessage",
		"traverse", "isInitialized", "unknownFields", "debugDescription",
		"description", "dynamicType", "hashValue", "Type", "Protocol", "Swift",
		"Equatable", "Hashable", "Sendable",
		// Common types.
		"Bool", "Data", "Double", "Float", "Int", "Int32", "Int64", "String",
		"UInt", "UInt32", "UInt64",
		// Special variables.
		"__COLUMN__", "__FILE__", "__FUNCTION__", "__LINE__",
	)
	for keyword := range swiftKeywords {
		names[keyword] = struct{}{}
	}
	return names
}()

// swiftReservedParamNames are parameter names a generated Swift method cannot
// take without backticks; the suffix rule keeps them readable instead.
var swiftReservedParamNames = func() map[string]struct{} {
	names := setOf("uri", "params")
	for keyword := range swiftKeywords {
		names[keyword] = struct{}{}
	}
	return names
}()

func swiftParamIdentifiers(params []ResourceParamModel) []string {
	camel := make([]ResourceParamModel, len(params))
	for i, param := range params {
		camel[i] = ResourceParamModel{Name: swiftLowerCamel(param.Name)}
	}
	return safeParamIdentifiers(camel, func(name string) bool {
		_, ok := swiftReservedParamNames[name]
		return ok || isUnderscoreOnly(name)
	})
}

// swiftAbbreviations are upper-cased inside names, as swift-protobuf does
// for property names (user_id becomes userID).
var swiftAbbreviations = setOf("url", "http", "https", "id")

// swiftLowerCamel turns user_id into userID for Swift parameter labels.
func swiftLowerCamel(value string) string {
	var out strings.Builder
	for _, segment := range strings.Split(value, "_") {
		if segment == "" {
			continue
		}
		if out.Len() == 0 {
			runes := []rune(segment)
			runes[0] = unicode.ToLower(runes[0])
			out.WriteString(string(runes))
			continue
		}
		if _, ok := swiftAbbreviations[strings.ToLower(segment)]; ok {
			out.WriteString(strings.ToUpper(segment))
			continue
		}
		runes := []rune(segment)
		runes[0] = unicode.ToUpper(runes[0])
		out.WriteString(string(runes))
	}
	if out.Len() == 0 {
		return value
	}
	return out.String()
}

// swiftIdentifier escapes Swift keywords with backticks.
func swiftIdentifier(name string) string {
	if _, ok := swiftKeywords[name]; ok {
		return "`" + name + "`"
	}
	return name
}

// swiftMethodName is the lowerCamel handler method for an RPC or primitive.
func swiftMethodName(protoName string) string {
	return swiftIdentifier(jvmLowerCamelIdentifier(protoName))
}

// swiftTypePrefix ports NamingUtils.typePrefix: swift_prefix wins, otherwise
// "package.some_name" becomes "Package_SomeName_".
func swiftTypePrefix(file protoreflect.FileDescriptor) string {
	if options, ok := file.Options().(*descriptorpb.FileOptions); ok && options != nil && options.SwiftPrefix != nil {
		return options.GetSwiftPrefix()
	}
	pkg := string(file.Package())
	if pkg == "" {
		return ""
	}
	var prefix strings.Builder
	makeUpper := true
	for _, r := range pkg {
		switch {
		case r == '_':
			makeUpper = true
		case r == '.':
			makeUpper = true
			prefix.WriteByte('_')
		default:
			if prefix.Len() == 0 && unicode.IsDigit(r) {
				prefix.WriteByte('_')
			}
			if makeUpper {
				prefix.WriteRune(unicode.ToUpper(r))
				makeUpper = false
			} else {
				prefix.WriteRune(r)
			}
		}
	}
	prefix.WriteByte('_')
	return prefix.String()
}

// swiftSanitizeTypeName ports sanitizeTypeName with the "Message" disambiguator.
func swiftSanitizeTypeName(name string) string {
	const disambiguator = "Message"
	if _, reserved := swiftReservedTypeNames[name]; reserved || isUnderscoreOnly(name) {
		return name + disambiguator
	}
	if strings.HasSuffix(name, disambiguator) && len(name) > len(disambiguator) {
		return swiftSanitizeTypeName(strings.TrimSuffix(name, disambiguator)) + disambiguator
	}
	return name
}

// swiftMessageTypeName is the Swift type protoc-gen-swift generates for message.
// Well-known types live in the SwiftProtobuf module.
func swiftMessageTypeName(message protoreflect.MessageDescriptor) string {
	var name string
	if parent, ok := message.Parent().(protoreflect.MessageDescriptor); ok {
		name = swiftMessageTypeName(parent) + "." + swiftSanitizeTypeName(string(message.Name()))
	} else {
		name = swiftSanitizeTypeName(swiftTypePrefix(message.ParentFile()) + string(message.Name()))
	}
	if isSwiftProtobufWellKnownFile(message.ParentFile()) && message.Parent() == message.ParentFile() {
		return "SwiftProtobuf." + name
	}
	return name
}

func isSwiftProtobufWellKnownFile(file protoreflect.FileDescriptor) bool {
	return strings.HasPrefix(file.Path(), "google/protobuf/") && file.Package() == "google.protobuf"
}

// swiftStringLiteral renders value as a Swift string literal.
func swiftStringLiteral(value string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\\':
			out.WriteString(`\\`)
		case '"':
			out.WriteString(`\"`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&out, `\u{%x}`, r)
			} else {
				out.WriteRune(r)
			}
		}
	}
	out.WriteByte('"')
	return out.String()
}

func swiftStringArrayLiteral(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, swiftStringLiteral(value))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// swiftFileIdentifier is the CamelCase proto basename used in generated names.
func swiftFileIdentifier(protoPath string) string {
	return typescriptExportedIdentifier(typescriptFileBaseName(protoPath))
}

func swiftOutputPath(generatedFilenamePrefix string) string {
	return generatedFilenamePrefix + ".mcp.swift"
}

func swiftMessage(file *protogen.File, fullName string) *protogen.Message {
	return findProtogenMessage(file.Messages, fullName)
}
