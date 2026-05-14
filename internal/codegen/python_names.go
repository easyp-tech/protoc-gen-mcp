package codegen

import (
	"fmt"
	"hash/crc32"
	"path"
	"strings"
	"unicode"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func pythonModulePath(file *protogen.File) string {
	return pythonModulePathForProtoPath(file.Desc.Path())
}

func pythonOutputPath(file *protogen.File) string {
	return pythonGeneratedFilenamePrefix(file) + "_mcp.py"
}

func pythonProtobufOutputPath(file *protogen.File) string {
	return pythonGeneratedFilenamePrefix(file) + "_mcp_pb.py"
}

func pythonOutputPathForHandler(file *protogen.File, handler PythonHandler, dualHandlers bool) string {
	if dualHandlers && handler == PythonHandlerProtobuf {
		return pythonProtobufOutputPath(file)
	}
	return pythonOutputPath(file)
}

func pythonPublicModulePathForProtoPath(protoPath string) string {
	return strings.ReplaceAll(pythonGeneratedFilenamePrefixForProtoPath(protoPath), "/", ".") + "_mcp"
}

func pythonPublicBasenameModuleForProtoPath(protoPath string) string {
	return path.Base(pythonGeneratedFilenamePrefixForProtoPath(protoPath)) + "_mcp"
}

func pythonPublicModuleAliasForProtoPath(protoPath string, isCurrent bool) string {
	if isCurrent {
		return pythonPublicBasenameModuleForProtoPath(protoPath)
	}

	modulePath := strings.ReplaceAll(pythonPublicModulePathForProtoPath(protoPath), ".", "_")
	return fmt.Sprintf("%s_%08x", modulePath, crc32.ChecksumIEEE([]byte(protoPath)))
}

func pythonMethodName(method string) string {
	return toSnakeCase(method)
}

func pythonSchemaConst(service, method string) string {
	return toUpperSnakeCase(service) + "_" + toUpperSnakeCase(method)
}

func pythonRegisterName(service string) string {
	return "register_" + toSnakeCase(service) + "_tools"
}

func pythonProtocolName(service string) string {
	return service + "ToolHandler"
}

func pythonBasenameModule(file *protogen.File) string {
	return pythonBasenameModuleForProtoPath(file.Desc.Path())
}

func pythonModuleAlias(file *protogen.File, isCurrent bool) string {
	return pythonModuleAliasForProtoPath(file.Desc.Path(), isCurrent)
}

func pythonModuleAliasForProtoPath(protoPath string, isCurrent bool) string {
	if isCurrent {
		return pythonBasenameModuleForProtoPath(protoPath)
	}

	modulePath := strings.ReplaceAll(pythonModulePathForProtoPath(protoPath), ".", "_")
	return fmt.Sprintf("%s_%08x", modulePath, crc32.ChecksumIEEE([]byte(protoPath)))
}

func pythonImportParts(file *protogen.File) (string, string) {
	modulePath := pythonModulePath(file)
	lastDot := strings.LastIndex(modulePath, ".")
	if lastDot == -1 {
		return "", modulePath
	}
	return modulePath[:lastDot], modulePath[lastDot+1:]
}

func pythonGeneratedFilenamePrefix(file *protogen.File) string {
	return pythonGeneratedFilenamePrefixForProtoPath(file.Desc.Path())
}

func pythonGeneratedFilenamePrefixForProtoPath(protoPath string) string {
	prefix := strings.TrimSuffix(protoPath, path.Ext(protoPath))
	parts := strings.Split(prefix, "/")
	for i, part := range parts {
		parts[i] = strings.ReplaceAll(part, "-", "_")
	}
	return strings.Join(parts, "/")
}

func pythonModulePathForProtoPath(protoPath string) string {
	return strings.ReplaceAll(pythonGeneratedFilenamePrefixForProtoPath(protoPath), "/", ".") + "_pb2"
}

func pythonBasenameModuleForProtoPath(protoPath string) string {
	return path.Base(pythonGeneratedFilenamePrefixForProtoPath(protoPath)) + "_pb2"
}

func pythonPublicTypeName(message *protogen.Message) string {
	return pythonPublicIdentifier(descriptorTypePath(message.Desc.FullName(), message.Desc.ParentFile().Package()))
}

func pythonPublicEnumName(enum *protogen.Enum) string {
	return pythonPublicIdentifier(descriptorTypePath(enum.Desc.FullName(), enum.Desc.ParentFile().Package()))
}

func pythonPublicIdentifier(parts []string) string {
	var b strings.Builder
	for _, part := range parts {
		b.WriteString(pythonExportedIdentifier(part))
	}
	return b.String()
}

func pythonOneofWrapperName(typeName, oneofName string) string {
	return typeName + pythonExportedIdentifier(oneofName) + "Variant"
}

func pythonOneofVariantWrapperName(typeName, oneofName, fieldName string) string {
	return typeName + pythonExportedIdentifier(oneofName) + pythonExportedIdentifier(fieldName) + "Variant"
}

func pythonExportedIdentifier(value string) string {
	if value == "" {
		return ""
	}

	segments := strings.FieldsFunc(value, func(r rune) bool {
		return r == '_' || r == '.' || r == '-' || r == ' ' || r == '/'
	})
	var b strings.Builder
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		runes := []rune(segment)
		for i, r := range runes {
			if i == 0 {
				b.WriteRune(unicode.ToUpper(r))
				continue
			}
			if i > 0 && unicode.IsUpper(r) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) && unicode.IsUpper(runes[i-1]) {
				b.WriteRune(r)
				continue
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}

func descriptorTypePath(fullName, pkg protoreflect.FullName) []string {
	name := string(fullName)
	packagePrefix := string(pkg)
	if packagePrefix != "" {
		prefix := packagePrefix + "."
		name = strings.TrimPrefix(name, prefix)
	}
	if name == "" {
		return nil
	}
	return strings.Split(name, ".")
}

func pythonCheckNameCollision(registry map[string]string, category, name, owner string) error {
	if existing, ok := registry[name]; ok && existing != owner {
		return fmt.Errorf("python %s collision for %q between %s and %s", category, name, existing, owner)
	}
	registry[name] = owner
	return nil
}

func pythonCheckOwnedNameCollision(scoped map[string]map[string]string, scope, category, name, owner string) error {
	registry := scoped[scope]
	if registry == nil {
		registry = map[string]string{}
		scoped[scope] = registry
	}
	return pythonCheckNameCollision(registry, category, name, owner)
}

func toSnakeCase(value string) string {
	if value == "" {
		return ""
	}

	runes := []rune(value)
	var b strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 && shouldInsertUnderscore(runes, i) {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		if r == '.' || r == '-' || r == ' ' {
			b.WriteByte('_')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func toUpperSnakeCase(value string) string {
	return strings.ToUpper(toSnakeCase(value))
}

func shouldInsertUnderscore(runes []rune, i int) bool {
	prev := runes[i-1]
	if prev == '_' || prev == '.' || prev == '-' || prev == ' ' {
		return false
	}
	if unicode.IsLower(prev) || unicode.IsDigit(prev) {
		return true
	}
	if i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
		return true
	}
	return false
}
