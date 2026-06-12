package codegen

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func CollectJVMFileModel(file *protogen.File, model FileModel) (JVMFileModel, error) {
	switch model.Options.Language {
	case LanguageKotlin, LanguageJava:
	default:
		return JVMFileModel{}, fmt.Errorf("jvm model requires lang=kotlin or lang=java, got %q", model.Options.Language)
	}

	methods, err := collectJVMGraphMethods(file)
	if err != nil {
		return JVMFileModel{}, err
	}
	methodByFullName := make(map[string]*protogen.Method, len(methods))
	for _, method := range methods {
		methodByFullName[string(method.Desc.FullName())] = method
	}

	includeCurrentFileTypes := len(methods) == 0
	typeGraph, err := collectJVMTypeGraphFromMethods(file, methods, includeCurrentFileTypes)
	if err != nil {
		return JVMFileModel{}, err
	}

	jvmModel := JVMFileModel{
		Language:                model.Options.Language,
		ProtoPath:               model.ProtoPath,
		GeneratedFilenamePrefix: jvmGeneratedFilenamePrefixForProtoPath(model.ProtoPath),
		ProtoPackage:            string(file.Desc.Package()),
		Services:                make([]JVMServiceModel, 0, len(model.Services)),
		Prompts:                 model.Prompts,
		Resources:               model.Resources,
		Types:                   typeGraph,
	}

	for _, service := range model.Services {
		serviceModel := JVMServiceModel{
			ProtoFullName: service.ProtoFullName,
			ProtoName:     service.ProtoName,
			Namespace:     service.Namespace,
			Description:   service.Description,
			HandlerName:   jvmHandlerName(service.ProtoName),
			RegisterName:  jvmRegisterName(service.ProtoName),
			Icons:         service.Icons,
			Methods:       make([]JVMMethodModel, 0, len(service.Methods)),
		}
		for _, method := range service.Methods {
			descriptorMethod := methodByFullName[method.ProtoFullName]
			serviceModel.Methods = append(serviceModel.Methods, newJVMMethodModel(service.ProtoName, method, descriptorMethod, file.Desc.Path()))
		}
		jvmModel.Services = append(jvmModel.Services, serviceModel)
	}

	return jvmModel, nil
}

func newJVMMethodModel(serviceName string, method MethodModel, descriptorMethod *protogen.Method, currentProtoPath string) JVMMethodModel {
	methodModel := JVMMethodModel{
		ProtoFullName:    method.ProtoFullName,
		ProtoName:        method.ProtoName,
		ToolName:         method.Name,
		Title:            method.Title,
		Description:      method.Description,
		Examples:         append([]string(nil), method.Examples...),
		Deprecated:       method.Deprecated,
		MethodName:       jvmMethodName(method.ProtoName),
		SchemaConst:      jvmSchemaConst(serviceName, method.ProtoName),
		Input:            newJVMTypeRefFromShared(method.Input),
		Output:           newJVMTypeRefFromShared(method.Output),
		InputSchemaJSON:  method.InputSchemaJSON,
		OutputSchemaJSON: method.OutputSchemaJSON,
		Annotations:      method.Annotations,
		Icons:            method.Icons,
		TaskSupport:      method.TaskSupport,
	}
	if descriptorMethod != nil {
		methodModel.Input = newJVMTypeRefFromMessage(descriptorMethod.Input, currentProtoPath)
		methodModel.Output = newJVMTypeRefFromMessage(descriptorMethod.Output, currentProtoPath)
	}
	return methodModel
}

func newJVMTypeRefFromShared(ref TypeRef) JVMTypeRef {
	return JVMTypeRef{
		ProtoFullName: ref.ProtoFullName,
		ProtoName:     ref.ProtoDisplayName,
		PublicName:    jvmExportedIdentifier(ref.ProtoDisplayName),
		IsMessage:     ref.ProtoFullName != "",
	}
}

func newJVMTypeRefFromMessage(message *protogen.Message, currentProtoPath string) JVMTypeRef {
	if message == nil {
		return JVMTypeRef{}
	}
	owner := newJVMTypeOwner(
		message.Desc.ParentFile().Path(),
		string(message.Desc.ParentFile().Package()),
		currentProtoPath,
	)
	return JVMTypeRef{
		ProtoFullName: string(message.Desc.FullName()),
		ProtoName:     string(message.Desc.Name()),
		PublicName:    jvmPublicTypeName(message),
		Owner:         owner,
		IsMessage:     true,
	}
}

func collectJVMGraphMethods(file *protogen.File) ([]*protogen.Method, error) {
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

func collectJVMTypeGraphFromMethods(file *protogen.File, methods []*protogen.Method, includeCurrentFileTypes bool) (JVMTypeGraph, error) {
	collector := jvmTypeCollector{
		rootProtoPath: file.Desc.Path(),
		graph: JVMTypeGraph{
			CurrentFile: newJVMTypeOwner(file.Desc.Path(), string(file.Desc.Package()), file.Desc.Path()),
		},
		ownedPublicNames: map[string]map[string]string{},
		seenMessages:     map[protoreflect.FullName]bool{},
		seenEnums:        map[protoreflect.FullName]bool{},
	}

	for _, method := range methods {
		if err := collector.collectMessage(method.Input); err != nil {
			return JVMTypeGraph{}, err
		}
		if err := collector.collectMessage(method.Output); err != nil {
			return JVMTypeGraph{}, err
		}
	}
	if includeCurrentFileTypes {
		if err := collector.collectCurrentFileTypes(file); err != nil {
			return JVMTypeGraph{}, err
		}
	}

	return collector.graph, nil
}

type jvmTypeCollector struct {
	rootProtoPath    string
	graph            JVMTypeGraph
	ownedPublicNames map[string]map[string]string
	seenMessages     map[protoreflect.FullName]bool
	seenEnums        map[protoreflect.FullName]bool
}

func (c *jvmTypeCollector) collectCurrentFileTypes(file *protogen.File) error {
	for _, message := range file.Messages {
		if err := c.collectMessageTree(message); err != nil {
			return err
		}
	}
	for _, enum := range file.Enums {
		if err := c.collectEnum(enum); err != nil {
			return err
		}
	}
	return nil
}

func (c *jvmTypeCollector) collectMessageTree(message *protogen.Message) error {
	if err := c.collectMessage(message); err != nil {
		return err
	}
	for _, nested := range message.Messages {
		if err := c.collectMessageTree(nested); err != nil {
			return err
		}
	}
	for _, enum := range message.Enums {
		if err := c.collectEnum(enum); err != nil {
			return err
		}
	}
	return nil
}

func (c *jvmTypeCollector) collectMessage(message *protogen.Message) error {
	if message == nil || message.Desc.IsMapEntry() {
		return nil
	}

	if wkt, ok, err := classifyJVMWellKnownType(message.Desc.FullName()); ok {
		if err != nil {
			return err
		}
		if wkt != JVMWellKnownTypeNone {
			return nil
		}
	}

	if c.seenMessages[message.Desc.FullName()] {
		return nil
	}
	c.seenMessages[message.Desc.FullName()] = true

	owner, err := c.ownerForFile(message.Desc.ParentFile())
	if err != nil {
		return err
	}

	publicName := jvmPublicTypeName(message)
	ownerKey := string(message.Desc.FullName())
	if err := c.checkOwnedPublicName(owner, "public type name", publicName, ownerKey); err != nil {
		return err
	}

	model := JVMType{
		Kind:          JVMTypeKindMessage,
		ProtoFullName: string(message.Desc.FullName()),
		ProtoName:     string(message.Desc.Name()),
		PublicName:    publicName,
		Owner:         owner,
		NestingPath:   descriptorTypePath(message.Desc.FullName(), message.Desc.ParentFile().Package()),
	}

	oneofWrappers := make(map[protoreflect.Name]string, len(message.Oneofs))
	for _, oneof := range message.Oneofs {
		if oneof.Desc.IsSynthetic() {
			continue
		}
		wrapperName := jvmOneofWrapperName(publicName, string(oneof.Desc.Name()))
		if err := c.checkOwnedPublicName(owner, "oneof wrapper name", wrapperName, ownerKey+"."+string(oneof.Desc.Name())); err != nil {
			return err
		}
		oneofWrappers[oneof.Desc.Name()] = wrapperName
	}

	for _, field := range message.Fields {
		fieldModel, err := c.collectField(field, oneofWrappers, publicName)
		if err != nil {
			return err
		}
		model.Fields = append(model.Fields, fieldModel)
	}

	for _, oneof := range message.Oneofs {
		if oneof.Desc.IsSynthetic() {
			continue
		}
		wrapperName := oneofWrappers[oneof.Desc.Name()]
		oneofModel := JVMOneof{
			ProtoName:   string(oneof.Desc.Name()),
			WrapperName: wrapperName,
			Variants:    make([]JVMOneofVariant, 0, len(oneof.Fields)),
		}
		for _, field := range oneof.Fields {
			ref, err := c.typeRefForField(field)
			if err != nil {
				return err
			}
			variantWrapper := jvmOneofVariantWrapperName(publicName, string(oneof.Desc.Name()), string(field.Desc.Name()))
			if err := c.checkOwnedPublicName(owner, "oneof wrapper name", variantWrapper, ownerKey+"."+string(oneof.Desc.Name())+"."+string(field.Desc.Name())); err != nil {
				return err
			}
			oneofModel.Variants = append(oneofModel.Variants, JVMOneofVariant{
				ProtoName:   string(field.Desc.Name()),
				FieldNumber: int(field.Desc.Number()),
				WrapperName: variantWrapper,
				Type:        ref,
				HasPresence: field.Desc.HasPresence(),
			})
		}
		model.Oneofs = append(model.Oneofs, oneofModel)
	}

	c.graph.Types = append(c.graph.Types, model)
	return nil
}

func (c *jvmTypeCollector) collectEnum(enum *protogen.Enum) error {
	if enum == nil {
		return nil
	}
	if c.seenEnums[enum.Desc.FullName()] {
		return nil
	}
	c.seenEnums[enum.Desc.FullName()] = true

	owner, err := c.ownerForFile(enum.Desc.ParentFile())
	if err != nil {
		return err
	}

	publicName := jvmPublicEnumName(enum)
	if err := c.checkOwnedPublicName(owner, "public type name", publicName, string(enum.Desc.FullName())); err != nil {
		return err
	}

	model := JVMType{
		Kind:          JVMTypeKindEnum,
		ProtoFullName: string(enum.Desc.FullName()),
		ProtoName:     string(enum.Desc.Name()),
		PublicName:    publicName,
		Owner:         owner,
		NestingPath:   descriptorTypePath(enum.Desc.FullName(), enum.Desc.ParentFile().Package()),
	}
	for _, value := range enum.Values {
		metadata, err := loadEnumValueMetadata(value)
		if err != nil {
			return err
		}
		model.EnumValues = append(model.EnumValues, JVMEnumValue{
			ProtoName: string(value.Desc.Name()),
			Number:    int32(value.Desc.Number()),
			Hidden:    metadata.Hidden,
		})
	}

	c.graph.Types = append(c.graph.Types, model)
	return nil
}

func (c *jvmTypeCollector) collectField(field *protogen.Field, oneofWrappers map[protoreflect.Name]string, parentPublicName string) (JVMField, error) {
	typeRef, err := c.typeRefForField(field)
	if err != nil {
		return JVMField{}, err
	}

	model := JVMField{
		ProtoName:   string(field.Desc.Name()),
		JSONName:    field.Desc.JSONName(),
		Number:      int(field.Desc.Number()),
		Type:        typeRef,
		IsRepeated:  field.Desc.IsList() && !field.Desc.IsMap(),
		IsMap:       field.Desc.IsMap(),
		HasPresence: field.Desc.HasPresence(),
		IsSchemaRequired: !field.Desc.IsList() &&
			!field.Desc.IsMap() &&
			(field.Oneof == nil || field.Oneof.Desc.IsSynthetic()) &&
			!field.Desc.HasOptionalKeyword(),
	}

	if field.Oneof != nil && !field.Oneof.Desc.IsSynthetic() {
		model.OneofProtoName = string(field.Oneof.Desc.Name())
		model.OneofWrapperName = oneofWrappers[field.Oneof.Desc.Name()]
		model.VariantWrapperName = jvmOneofVariantWrapperName(parentPublicName, string(field.Oneof.Desc.Name()), string(field.Desc.Name()))
	}

	if field.Desc.IsMap() && field.Message != nil && len(field.Message.Fields) == 2 {
		model.MapKeyScalar = jvmScalarForKind(field.Message.Fields[0].Desc.Kind())
		valueRef, err := c.typeRefForField(field.Message.Fields[1])
		if err != nil {
			return JVMField{}, err
		}
		model.MapValue = &valueRef
	}

	return model, nil
}

func (c *jvmTypeCollector) typeRefForField(field *protogen.Field) (JVMTypeRef, error) {
	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		return JVMTypeRef{Scalar: JVMScalarBool}, nil
	case protoreflect.StringKind:
		return JVMTypeRef{Scalar: JVMScalarString}, nil
	case protoreflect.BytesKind:
		return JVMTypeRef{Scalar: JVMScalarBytes}, nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return JVMTypeRef{Scalar: JVMScalarInt32}, nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return JVMTypeRef{Scalar: JVMScalarUInt32}, nil
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return JVMTypeRef{Scalar: JVMScalarInt64}, nil
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return JVMTypeRef{Scalar: JVMScalarUInt64}, nil
	case protoreflect.FloatKind:
		return JVMTypeRef{Scalar: JVMScalarFloat}, nil
	case protoreflect.DoubleKind:
		return JVMTypeRef{Scalar: JVMScalarDouble}, nil
	case protoreflect.EnumKind:
		if err := c.collectEnum(field.Enum); err != nil {
			return JVMTypeRef{}, err
		}
		owner, err := c.ownerForFile(field.Enum.Desc.ParentFile())
		if err != nil {
			return JVMTypeRef{}, err
		}
		return JVMTypeRef{
			ProtoFullName: string(field.Enum.Desc.FullName()),
			ProtoName:     string(field.Enum.Desc.Name()),
			PublicName:    jvmPublicEnumName(field.Enum),
			Owner:         owner,
			IsEnum:        true,
		}, nil
	case protoreflect.MessageKind:
		if field.Message == nil {
			return JVMTypeRef{}, nil
		}
		wkt, ok, err := classifyJVMWellKnownType(field.Message.Desc.FullName())
		if err != nil {
			return JVMTypeRef{}, err
		}
		owner, ownerErr := c.ownerForFile(field.Message.Desc.ParentFile())
		if ownerErr != nil {
			return JVMTypeRef{}, ownerErr
		}
		if ok {
			return JVMTypeRef{
				ProtoFullName: string(field.Message.Desc.FullName()),
				ProtoName:     string(field.Message.Desc.Name()),
				Owner:         owner,
				WellKnownType: wkt,
				IsMessage:     true,
			}, nil
		}
		if err := c.collectMessage(field.Message); err != nil {
			return JVMTypeRef{}, err
		}
		return JVMTypeRef{
			ProtoFullName: string(field.Message.Desc.FullName()),
			ProtoName:     string(field.Message.Desc.Name()),
			PublicName:    jvmPublicTypeName(field.Message),
			Owner:         owner,
			IsMessage:     true,
		}, nil
	default:
		return JVMTypeRef{}, nil
	}
}

func (c *jvmTypeCollector) ownerForFile(file protoreflect.FileDescriptor) (JVMTypeOwner, error) {
	owner := newJVMTypeOwner(file.Path(), string(file.Package()), c.rootProtoPath)
	if owner.IsCurrentFile {
		return owner, nil
	}

	for _, existing := range c.graph.Imports {
		if existing.ProtoPath == owner.ProtoPath {
			return owner, nil
		}
	}
	c.graph.Imports = append(c.graph.Imports, JVMImport{
		ProtoPath:               owner.ProtoPath,
		GeneratedFilenamePrefix: owner.GeneratedFilenamePrefix,
		ProtoPackage:            owner.ProtoPackage,
	})
	return owner, nil
}

func (c *jvmTypeCollector) checkOwnedPublicName(owner JVMTypeOwner, category, name, ownerKey string) error {
	return jvmCheckOwnedNameCollision(c.ownedPublicNames, owner.GeneratedFilenamePrefix, category, name, ownerKey)
}

func newJVMTypeOwner(protoPath, protoPackage, currentProtoPath string) JVMTypeOwner {
	return JVMTypeOwner{
		ProtoPath:               protoPath,
		IsCurrentFile:           protoPath == currentProtoPath,
		GeneratedFilenamePrefix: jvmGeneratedFilenamePrefixForProtoPath(protoPath),
		ProtoPackage:            protoPackage,
	}
}

func classifyJVMWellKnownType(fullName protoreflect.FullName) (JVMWellKnownType, bool, error) {
	switch fullName {
	case "google.protobuf.Any":
		return JVMWellKnownTypeAny, true, nil
	case "google.protobuf.Empty":
		return JVMWellKnownTypeEmpty, true, nil
	case "google.protobuf.Timestamp":
		return JVMWellKnownTypeTimestamp, true, nil
	case "google.protobuf.Duration":
		return JVMWellKnownTypeDuration, true, nil
	case "google.protobuf.FieldMask":
		return JVMWellKnownTypeFieldMask, true, nil
	case "google.protobuf.Struct":
		return JVMWellKnownTypeStruct, true, nil
	case "google.protobuf.Value":
		return JVMWellKnownTypeValue, true, nil
	case "google.protobuf.ListValue":
		return JVMWellKnownTypeListValue, true, nil
	case "google.protobuf.BoolValue":
		return JVMWellKnownTypeBoolValue, true, nil
	case "google.protobuf.StringValue":
		return JVMWellKnownTypeStringValue, true, nil
	case "google.protobuf.BytesValue":
		return JVMWellKnownTypeBytesValue, true, nil
	case "google.protobuf.Int32Value":
		return JVMWellKnownTypeInt32Value, true, nil
	case "google.protobuf.UInt32Value":
		return JVMWellKnownTypeUInt32Value, true, nil
	case "google.protobuf.Int64Value":
		return JVMWellKnownTypeInt64Value, true, nil
	case "google.protobuf.UInt64Value":
		return JVMWellKnownTypeUInt64Value, true, nil
	case "google.protobuf.FloatValue":
		return JVMWellKnownTypeFloatValue, true, nil
	case "google.protobuf.DoubleValue":
		return JVMWellKnownTypeDoubleValue, true, nil
	default:
		if isJVMGoogleWellKnownType(fullName) {
			return JVMWellKnownTypeNone, true, fmt.Errorf("well-known type %q is not supported", fullName)
		}
		return JVMWellKnownTypeNone, false, nil
	}
}

func isJVMGoogleWellKnownType(fullName protoreflect.FullName) bool {
	return strings.HasPrefix(string(fullName), "google.protobuf.")
}

func jvmScalarForKind(kind protoreflect.Kind) JVMScalar {
	switch kind {
	case protoreflect.BoolKind:
		return JVMScalarBool
	case protoreflect.StringKind:
		return JVMScalarString
	case protoreflect.BytesKind:
		return JVMScalarBytes
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return JVMScalarInt32
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return JVMScalarUInt32
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return JVMScalarInt64
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return JVMScalarUInt64
	case protoreflect.FloatKind:
		return JVMScalarFloat
	case protoreflect.DoubleKind:
		return JVMScalarDouble
	default:
		return JVMScalarUnknown
	}
}
