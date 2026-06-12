package codegen

import (
	"fmt"
	"sort"

	"google.golang.org/protobuf/compiler/protogen"
)

func CollectTypeScriptFileModel(file *protogen.File, model FileModel) (TypeScriptFileModel, error) {
	if model.Options.Language != LanguageTypeScript {
		return TypeScriptFileModel{}, fmt.Errorf("typescript model requires lang=typescript, got %q", model.Options.Language)
	}

	methods, err := collectTypeScriptDescriptorMethods(file)
	if err != nil {
		return TypeScriptFileModel{}, err
	}
	methodByFullName := make(map[string]*protogen.Method, len(methods))
	for _, method := range methods {
		methodByFullName[string(method.Desc.FullName())] = method
	}

	currentFile := newTypeScriptTypeOwner(file.Desc.Path(), file.Desc.Path())
	registryRefs := map[string]TypeScriptRegistryRef{
		currentFile.ProtoPath: currentFile.RegistryRef,
	}
	imports := map[string]*typeScriptImportAccumulator{}

	tsModel := TypeScriptFileModel{
		Language:                model.Options.Language,
		ProtoPath:               model.ProtoPath,
		GeneratedFilenamePrefix: model.GeneratedFilenamePrefix,
		ProtoPackage:            string(file.Desc.Package()),
		CurrentFile:             currentFile,
		Services:                make([]TypeScriptServiceModel, 0, len(model.Services)),
		Prompts:                 model.Prompts,
		Resources:               model.Resources,
	}

	for _, service := range model.Services {
		serviceModel := TypeScriptServiceModel{
			ProtoFullName: service.ProtoFullName,
			ProtoName:     service.ProtoName,
			Namespace:     service.Namespace,
			Description:   service.Description,
			HandlerName:   typescriptHandlerName(service.ProtoName),
			RegisterName:  typescriptRegisterName(service.ProtoName),
			Icons:         service.Icons,
			Methods:       make([]TypeScriptMethodModel, 0, len(service.Methods)),
		}
		for _, method := range service.Methods {
			descriptorMethod := methodByFullName[method.ProtoFullName]
			methodModel := newTypeScriptMethodModel(service.ProtoName, method, descriptorMethod, file.Desc.Path())
			trackTypeScriptTypeRef(methodModel.Input, imports, registryRefs)
			trackTypeScriptTypeRef(methodModel.Output, imports, registryRefs)
			serviceModel.Methods = append(serviceModel.Methods, methodModel)
		}
		tsModel.Services = append(tsModel.Services, serviceModel)
	}

	tsModel.Imports = flattenTypeScriptImports(imports)
	if err := validateTypeScriptImportCollisions(tsModel.Imports); err != nil {
		return TypeScriptFileModel{}, err
	}
	tsModel.RegistryRefs = flattenTypeScriptRegistryRefs(registryRefs)
	if err := validateTypeScriptModelRefCollisions(tsModel); err != nil {
		return TypeScriptFileModel{}, err
	}
	return tsModel, nil
}

func newTypeScriptMethodModel(serviceName string, method MethodModel, descriptorMethod *protogen.Method, currentProtoPath string) TypeScriptMethodModel {
	methodModel := TypeScriptMethodModel{
		ProtoFullName:    method.ProtoFullName,
		ProtoName:        method.ProtoName,
		ToolName:         method.Name,
		Title:            method.Title,
		Description:      method.Description,
		Examples:         append([]string(nil), method.Examples...),
		Deprecated:       method.Deprecated,
		MethodName:       typescriptMethodName(method.ProtoName),
		SchemaConst:      typescriptSchemaConst(serviceName, method.ProtoName),
		Input:            newTypeScriptTypeRefFromShared(method.Input),
		Output:           newTypeScriptTypeRefFromShared(method.Output),
		InputSchemaJSON:  method.InputSchemaJSON,
		OutputSchemaJSON: method.OutputSchemaJSON,
		Annotations:      method.Annotations,
		Icons:            method.Icons,
		TaskSupport:      method.TaskSupport,
	}
	if descriptorMethod != nil {
		methodModel.Input = newTypeScriptTypeRefFromMessage(descriptorMethod.Input, currentProtoPath)
		methodModel.Output = newTypeScriptTypeRefFromMessage(descriptorMethod.Output, currentProtoPath)
	}
	return methodModel
}

func newTypeScriptTypeRefFromShared(ref TypeRef) TypeScriptTypeRef {
	typeName := typescriptExportedIdentifier(ref.ProtoDisplayName)
	return TypeScriptTypeRef{
		ProtoFullName: ref.ProtoFullName,
		ProtoName:     ref.ProtoDisplayName,
		TypeName:      typeName,
		SchemaName:    typescriptSchemaName(typeName),
	}
}

func newTypeScriptTypeRefFromMessage(message *protogen.Message, currentProtoPath string) TypeScriptTypeRef {
	if message == nil {
		return TypeScriptTypeRef{}
	}
	owner := newTypeScriptTypeOwner(message.Desc.ParentFile().Path(), currentProtoPath)
	typeName := typescriptPublicTypeName(message)
	return TypeScriptTypeRef{
		ProtoFullName: string(message.Desc.FullName()),
		ProtoName:     string(message.Desc.Name()),
		TypeName:      typeName,
		SchemaName:    typescriptSchemaName(typeName),
		Owner:         owner,
		RegistryRef:   owner.RegistryRef,
	}
}

func collectTypeScriptDescriptorMethods(file *protogen.File) ([]*protogen.Method, error) {
	var methods []*protogen.Method
	for _, service := range file.Services {
		for _, method := range service.Methods {
			_, include, err := selectToolMethod(method)
			if err != nil {
				return nil, err
			}
			if !include {
				continue
			}
			methods = append(methods, method)
		}
	}
	return methods, nil
}

func newTypeScriptTypeOwner(protoPath, currentProtoPath string) TypeScriptTypeOwner {
	registryRef := typescriptRegistryRefForProtoPath(protoPath)
	return TypeScriptTypeOwner{
		ProtoPath:               protoPath,
		IsCurrentFile:           protoPath == currentProtoPath,
		GeneratedFilenamePrefix: typescriptGeneratedFilenamePrefixForProtoPath(protoPath),
		ModuleSpecifier:         typescriptProtobufModuleSpecifier(currentProtoPath, protoPath),
		RegistryRef:             registryRef,
	}
}

type typeScriptImportAccumulator struct {
	protoPath               string
	generatedFilenamePrefix string
	moduleSpecifier         string
	typeNames               map[string]struct{}
	schemaNames             map[string]struct{}
	registryRefs            map[string]struct{}
}

func trackTypeScriptTypeRef(ref TypeScriptTypeRef, imports map[string]*typeScriptImportAccumulator, registryRefs map[string]TypeScriptRegistryRef) {
	if ref.Owner.ProtoPath == "" {
		return
	}
	registryRefs[ref.Owner.ProtoPath] = ref.RegistryRef
	if ref.Owner.IsCurrentFile {
		return
	}

	acc := imports[ref.Owner.ProtoPath]
	if acc == nil {
		acc = &typeScriptImportAccumulator{
			protoPath:               ref.Owner.ProtoPath,
			generatedFilenamePrefix: ref.Owner.GeneratedFilenamePrefix,
			moduleSpecifier:         ref.Owner.ModuleSpecifier,
			typeNames:               map[string]struct{}{},
			schemaNames:             map[string]struct{}{},
			registryRefs:            map[string]struct{}{},
		}
		imports[ref.Owner.ProtoPath] = acc
	}
	if ref.TypeName != "" {
		acc.typeNames[ref.TypeName] = struct{}{}
	}
	if ref.SchemaName != "" {
		acc.schemaNames[ref.SchemaName] = struct{}{}
	}
	if ref.RegistryRef.RefName != "" {
		acc.registryRefs[ref.RegistryRef.RefName] = struct{}{}
	}
}

func flattenTypeScriptImports(imports map[string]*typeScriptImportAccumulator) []TypeScriptImport {
	protoPaths := make([]string, 0, len(imports))
	for protoPath := range imports {
		protoPaths = append(protoPaths, protoPath)
	}
	sort.Strings(protoPaths)

	out := make([]TypeScriptImport, 0, len(protoPaths))
	for _, protoPath := range protoPaths {
		acc := imports[protoPath]
		out = append(out, TypeScriptImport{
			ProtoPath:               acc.protoPath,
			GeneratedFilenamePrefix: acc.generatedFilenamePrefix,
			ModuleSpecifier:         acc.moduleSpecifier,
			TypeNames:               sortedStringSet(acc.typeNames),
			SchemaNames:             sortedStringSet(acc.schemaNames),
			RegistryRefs:            sortedStringSet(acc.registryRefs),
		})
	}
	return out
}

func flattenTypeScriptRegistryRefs(registryRefs map[string]TypeScriptRegistryRef) []TypeScriptRegistryRef {
	protoPaths := make([]string, 0, len(registryRefs))
	for protoPath := range registryRefs {
		protoPaths = append(protoPaths, protoPath)
	}
	sort.Strings(protoPaths)

	out := make([]TypeScriptRegistryRef, 0, len(protoPaths))
	for _, protoPath := range protoPaths {
		out = append(out, registryRefs[protoPath])
	}
	return out
}

func sortedStringSet(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
