package codegen

import (
	"fmt"
	"path"
	"strings"
	"unicode"

	"google.golang.org/protobuf/compiler/protogen"
)

func jvmGeneratedFilenamePrefixForProtoPath(protoPath string) string {
	prefix := strings.TrimSuffix(protoPath, path.Ext(protoPath))
	parts := strings.Split(prefix, "/")
	for i, part := range parts {
		parts[i] = strings.ReplaceAll(part, "-", "_")
	}
	return strings.Join(parts, "/")
}

func jvmFileBaseName(protoPath string) string {
	base := strings.TrimSuffix(protoPath, ".proto")
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}
	return base
}

func jvmPublicTypeName(message *protogen.Message) string {
	return jvmPublicIdentifier(descriptorTypePath(message.Desc.FullName(), message.Desc.ParentFile().Package()))
}

func jvmPublicEnumName(enum *protogen.Enum) string {
	return jvmPublicIdentifier(descriptorTypePath(enum.Desc.FullName(), enum.Desc.ParentFile().Package()))
}

func jvmPublicIdentifier(parts []string) string {
	var b strings.Builder
	for _, part := range parts {
		b.WriteString(jvmExportedIdentifier(part))
	}
	return b.String()
}

func jvmExportedIdentifier(value string) string {
	return pythonExportedIdentifier(value)
}

func jvmLowerCamelIdentifier(value string) string {
	exported := jvmExportedIdentifier(value)
	if exported == "" {
		return ""
	}

	runes := []rune(exported)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func jvmMethodName(method string) string {
	return jvmLowerCamelIdentifier(method)
}

func jvmHandlerName(service string) string {
	return jvmExportedIdentifier(service) + "ToolHandler"
}

func jvmRegisterName(service string) string {
	return "register" + jvmExportedIdentifier(service) + "Tools"
}

func jvmSchemaConst(service, method string) string {
	return toUpperSnakeCase(service) + "_" + toUpperSnakeCase(method)
}

func jvmOneofWrapperName(typeName, oneofName string) string {
	return typeName + jvmExportedIdentifier(oneofName) + "Variant"
}

func jvmOneofVariantWrapperName(typeName, oneofName, fieldName string) string {
	return typeName + jvmExportedIdentifier(oneofName) + jvmExportedIdentifier(fieldName) + "Variant"
}

func jvmCheckNameCollision(registry map[string]string, category, name, owner string) error {
	if existing, ok := registry[name]; ok && existing != owner {
		return fmt.Errorf("jvm %s collision for %q between %s and %s", category, name, existing, owner)
	}
	registry[name] = owner
	return nil
}

func jvmCheckOwnedNameCollision(scoped map[string]map[string]string, scope, category, name, owner string) error {
	registry := scoped[scope]
	if registry == nil {
		registry = map[string]string{}
		scoped[scope] = registry
	}
	return jvmCheckNameCollision(registry, category, name, owner)
}
