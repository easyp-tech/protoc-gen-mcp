package codegen

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
)

func typescriptPublicTypeName(message *protogen.Message) string {
	return typescriptPublicIdentifier(descriptorTypePath(message.Desc.FullName(), message.Desc.ParentFile().Package()))
}

func typescriptPublicIdentifier(parts []string) string {
	var b strings.Builder
	for _, part := range parts {
		b.WriteString(typescriptExportedIdentifier(part))
	}
	return b.String()
}

func typescriptExportedIdentifier(value string) string {
	return pythonExportedIdentifier(value)
}

func typescriptMethodName(method string) string {
	return jvmLowerCamelIdentifier(method)
}

func typescriptHandlerName(service string) string {
	return typescriptExportedIdentifier(service) + "ToolHandler"
}

func typescriptRegisterName(service string) string {
	return "register" + typescriptExportedIdentifier(service) + "Tools"
}

func typescriptSchemaName(typeName string) string {
	if typeName == "" {
		return ""
	}
	return typeName + "Schema"
}

func typescriptRegistryRefForProtoPath(protoPath string) TypeScriptRegistryRef {
	return TypeScriptRegistryRef{
		ProtoPath: protoPath,
		RefName:   typescriptFileRegistryRefName(protoPath),
	}
}

func typescriptFileRegistryRefName(protoPath string) string {
	prefix := typescriptGeneratedFilenamePrefixForProtoPath(protoPath)
	return "file_" + strings.ReplaceAll(prefix, "/", "_")
}

func typescriptProtobufModuleSpecifier(currentProtoPath, targetProtoPath string) string {
	currentDir := path.Dir(typescriptOutputPathForProtoPath(currentProtoPath))
	targetPath := typescriptGeneratedFilenamePrefixForProtoPath(targetProtoPath) + "_pb.js"

	rel, err := filepath.Rel(currentDir, targetPath)
	if err != nil {
		rel = targetPath
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		rel = path.Base(targetPath)
	}
	if strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "./") {
		return rel
	}
	return "./" + rel
}

func typescriptCheckNameCollision(registry map[string]string, category, name, owner string) error {
	if existing, ok := registry[name]; ok && existing != owner {
		return fmt.Errorf("typescript %s collision for %q between %s and %s", category, name, existing, owner)
	}
	registry[name] = owner
	return nil
}

func typescriptCheckOwnedNameCollision(scoped map[string]map[string]string, scope, category, name, owner string) error {
	registry := scoped[scope]
	if registry == nil {
		registry = map[string]string{}
		scoped[scope] = registry
	}
	return typescriptCheckNameCollision(registry, category, name, owner)
}

func validateTypeScriptImportCollisions(imports []TypeScriptImport) error {
	typeNames := map[string]string{}
	schemaNames := map[string]string{}
	registryRefs := map[string]string{}

	for _, imp := range imports {
		for _, name := range imp.TypeNames {
			if err := typescriptCheckNameCollision(typeNames, "import type name", name, imp.ProtoPath); err != nil {
				return err
			}
		}
		for _, name := range imp.SchemaNames {
			if err := typescriptCheckNameCollision(schemaNames, "import value name", name, imp.ProtoPath); err != nil {
				return err
			}
		}
		for _, name := range imp.RegistryRefs {
			if err := typescriptCheckNameCollision(registryRefs, "registry ref", name, imp.ProtoPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateTypeScriptModelRefCollisions(model TypeScriptFileModel) error {
	typeNames := map[string]string{}
	schemaNames := map[string]string{}
	registryRefs := map[string]string{}

	for _, service := range model.Services {
		for _, method := range service.Methods {
			for _, ref := range []TypeScriptTypeRef{method.Input, method.Output} {
				if ref.Owner.ProtoPath == "" {
					continue
				}
				if err := typescriptCheckNameCollision(typeNames, "protobuf type name", ref.TypeName, ref.Owner.ProtoPath); err != nil {
					return err
				}
				if err := typescriptCheckNameCollision(schemaNames, "protobuf schema name", ref.SchemaName, ref.Owner.ProtoPath); err != nil {
					return err
				}
				if err := typescriptCheckNameCollision(registryRefs, "protobuf registry ref", ref.RegistryRef.RefName, ref.Owner.ProtoPath); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
