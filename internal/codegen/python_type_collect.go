package codegen

import (
	"fmt"
	"sort"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func CollectPythonTypeGraph(file *protogen.File, runtime PythonRuntime) (PythonTypeGraph, error) {
	methods, err := collectPythonGraphMethods(file)
	if err != nil {
		return PythonTypeGraph{}, err
	}
	return collectPythonTypeGraphFromMethods(file, runtime, methods, false)
}

func collectPythonTypeGraphFromMethods(file *protogen.File, runtime PythonRuntime, methods []*protogen.Method, includeCurrentFileTypes bool) (PythonTypeGraph, error) {
	collector := pythonTypeCollector{
		rootProtoPath: file.Desc.Path(),
		graph: PythonTypeGraph{
			Runtime:     runtime,
			CurrentFile: newPythonTypeOwner(file.Desc.Path(), file.Desc.Path()),
		},
		ownedPublicNames: map[string]map[string]string{},
		importAliases:    map[string]string{},
		seenMessages:     map[protoreflect.FullName]bool{},
		seenEnums:        map[protoreflect.FullName]bool{},
	}

	for _, method := range methods {
		if err := collector.collectMessage(method.Input); err != nil {
			return PythonTypeGraph{}, err
		}
		if err := collector.collectMessage(method.Output); err != nil {
			return PythonTypeGraph{}, err
		}
	}
	if includeCurrentFileTypes {
		if err := collector.collectCurrentFileTypes(file); err != nil {
			return PythonTypeGraph{}, err
		}
	}

	return collector.graph, nil
}

func collectPythonTypeGraphFromRefs(file *protogen.File, runtime PythonRuntime, refs map[string]struct{}) (PythonTypeGraph, error) {
	collector := pythonTypeCollector{
		rootProtoPath: file.Desc.Path(),
		graph: PythonTypeGraph{
			Runtime:     runtime,
			CurrentFile: newPythonTypeOwner(file.Desc.Path(), file.Desc.Path()),
		},
		ownedPublicNames: map[string]map[string]string{},
		importAliases:    map[string]string{},
		seenMessages:     map[protoreflect.FullName]bool{},
		seenEnums:        map[protoreflect.FullName]bool{},
	}

	messageIndex := map[string]*protogen.Message{}
	enumIndex := map[string]*protogen.Enum{}
	indexPythonFileTypes(file.Messages, file.Enums, messageIndex, enumIndex)

	orderedRefs := make([]string, 0, len(refs))
	for ref := range refs {
		orderedRefs = append(orderedRefs, ref)
	}
	sort.Strings(orderedRefs)

	for _, ref := range orderedRefs {
		switch {
		case messageIndex[ref] != nil:
			if err := collector.collectMessage(messageIndex[ref]); err != nil {
				return PythonTypeGraph{}, err
			}
		case enumIndex[ref] != nil:
			if err := collector.collectEnum(enumIndex[ref]); err != nil {
				return PythonTypeGraph{}, err
			}
		default:
			return PythonTypeGraph{}, fmt.Errorf("python type %q not found in file %q", ref, file.Desc.Path())
		}
	}

	return collector.graph, nil
}

func augmentPythonModelWithCurrentTypeRefs(file *protogen.File, model *FileModel, refs map[string]struct{}) error {
	if model == nil || len(refs) == 0 {
		return nil
	}

	graph := model.PythonTypes
	if graph == nil {
		collected, err := collectPythonTypeGraphFromRefs(file, model.Options.PythonRuntime, refs)
		if err != nil {
			return err
		}
		model.PythonTypes = &collected
		return nil
	}

	extra, err := collectPythonTypeGraphFromRefs(file, model.Options.PythonRuntime, refs)
	if err != nil {
		return err
	}
	return mergePythonTypeGraphs(graph, extra)
}

func mergePythonTypeGraphs(base *PythonTypeGraph, extra PythonTypeGraph) error {
	if base == nil {
		return nil
	}

	importsByProtoPath := make(map[string]struct{}, len(base.Imports))
	for _, item := range base.Imports {
		importsByProtoPath[item.ProtoPath] = struct{}{}
	}
	for _, item := range extra.Imports {
		if _, exists := importsByProtoPath[item.ProtoPath]; exists {
			continue
		}
		base.Imports = append(base.Imports, item)
		importsByProtoPath[item.ProtoPath] = struct{}{}
	}

	typesByFullName := make(map[string]struct{}, len(base.Types))
	currentPublicNames := make(map[string]string)
	for _, typ := range base.Types {
		typesByFullName[typ.ProtoFullName] = struct{}{}
		if typ.Owner.IsCurrentFile {
			currentPublicNames[typ.PublicName] = typ.ProtoFullName
		}
	}
	for _, typ := range extra.Types {
		if existing, ok := currentPublicNames[typ.PublicName]; ok && typ.Owner.IsCurrentFile && existing != typ.ProtoFullName {
			return fmt.Errorf("python public type name collision for %q between %s and %s", typ.PublicName, existing, typ.ProtoFullName)
		}
		if _, exists := typesByFullName[typ.ProtoFullName]; exists {
			continue
		}
		base.Types = append(base.Types, typ)
		typesByFullName[typ.ProtoFullName] = struct{}{}
		if typ.Owner.IsCurrentFile {
			currentPublicNames[typ.PublicName] = typ.ProtoFullName
		}
	}

	return nil
}

func collectPythonGraphMethods(file *protogen.File) ([]*protogen.Method, error) {
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

func indexPythonFileTypes(messages []*protogen.Message, enums []*protogen.Enum, messageIndex map[string]*protogen.Message, enumIndex map[string]*protogen.Enum) {
	for _, enum := range enums {
		enumIndex[string(enum.Desc.FullName())] = enum
	}
	for _, message := range messages {
		messageIndex[string(message.Desc.FullName())] = message
		indexPythonFileTypes(message.Messages, message.Enums, messageIndex, enumIndex)
	}
}

type pythonTypeCollector struct {
	rootProtoPath    string
	graph            PythonTypeGraph
	ownedPublicNames map[string]map[string]string
	importAliases    map[string]string
	seenMessages     map[protoreflect.FullName]bool
	seenEnums        map[protoreflect.FullName]bool
}

func (c *pythonTypeCollector) collectCurrentFileTypes(file *protogen.File) error {
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

func (c *pythonTypeCollector) collectMessageTree(message *protogen.Message) error {
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

func (c *pythonTypeCollector) collectMessage(message *protogen.Message) error {
	if message == nil || message.Desc.IsMapEntry() {
		return nil
	}

	if wkt, ok, err := classifyPythonWellKnownType(message.Desc.FullName()); ok {
		if err != nil {
			return err
		}
		if wkt != PythonWellKnownTypeNone {
			return nil
		}
	}

	if c.seenMessages[message.Desc.FullName()] {
		return nil
	}
	c.seenMessages[message.Desc.FullName()] = true

	owner, err := c.ownerForProtoPath(message.Desc.ParentFile().Path())
	if err != nil {
		return err
	}

	publicName := pythonPublicTypeName(message)
	ownerKey := string(message.Desc.FullName())
	if err := c.checkOwnedPublicName(owner, "public type name", publicName, ownerKey); err != nil {
		return err
	}

	model := PythonType{
		Kind:          PythonTypeKindMessage,
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
		wrapperName := pythonOneofWrapperName(publicName, string(oneof.Desc.Name()))
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
		oneofModel := PythonOneof{
			ProtoName:   string(oneof.Desc.Name()),
			WrapperName: wrapperName,
			Variants:    make([]PythonOneofVariant, 0, len(oneof.Fields)),
		}
		for _, field := range oneof.Fields {
			ref, err := c.typeRefForField(field)
			if err != nil {
				return err
			}
			variantWrapper := pythonOneofVariantWrapperName(publicName, string(oneof.Desc.Name()), string(field.Desc.Name()))
			if err := c.checkOwnedPublicName(owner, "oneof wrapper name", variantWrapper, ownerKey+"."+string(oneof.Desc.Name())+"."+string(field.Desc.Name())); err != nil {
				return err
			}
			oneofModel.Variants = append(oneofModel.Variants, PythonOneofVariant{
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

func (c *pythonTypeCollector) collectEnum(enum *protogen.Enum) error {
	if enum == nil {
		return nil
	}
	if c.seenEnums[enum.Desc.FullName()] {
		return nil
	}
	c.seenEnums[enum.Desc.FullName()] = true

	owner, err := c.ownerForProtoPath(enum.Desc.ParentFile().Path())
	if err != nil {
		return err
	}

	publicName := pythonPublicEnumName(enum)
	if err := c.checkOwnedPublicName(owner, "public type name", publicName, string(enum.Desc.FullName())); err != nil {
		return err
	}

	model := PythonType{
		Kind:          PythonTypeKindEnum,
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
		model.EnumValues = append(model.EnumValues, PythonEnumValue{
			ProtoName: string(value.Desc.Name()),
			Number:    int32(value.Desc.Number()),
			Hidden:    metadata.Hidden,
		})
	}

	c.graph.Types = append(c.graph.Types, model)
	return nil
}

func (c *pythonTypeCollector) collectField(field *protogen.Field, oneofWrappers map[protoreflect.Name]string, parentPublicName string) (PythonField, error) {
	typeRef, err := c.typeRefForField(field)
	if err != nil {
		return PythonField{}, err
	}

	model := PythonField{
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
		model.VariantWrapperName = pythonOneofVariantWrapperName(parentPublicName, string(field.Oneof.Desc.Name()), string(field.Desc.Name()))
	}

	if field.Desc.IsMap() && field.Message != nil && len(field.Message.Fields) == 2 {
		model.MapKeyScalar = pythonScalarForKind(field.Message.Fields[0].Desc.Kind())
		valueRef, err := c.typeRefForField(field.Message.Fields[1])
		if err != nil {
			return PythonField{}, err
		}
		model.MapValue = &valueRef
	}

	return model, nil
}

func (c *pythonTypeCollector) typeRefForField(field *protogen.Field) (PythonTypeRef, error) {
	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		return PythonTypeRef{Scalar: PythonScalarBool}, nil
	case protoreflect.StringKind:
		return PythonTypeRef{Scalar: PythonScalarString}, nil
	case protoreflect.BytesKind:
		return PythonTypeRef{Scalar: PythonScalarBytes}, nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return PythonTypeRef{Scalar: PythonScalarInt32}, nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return PythonTypeRef{Scalar: PythonScalarUInt32}, nil
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return PythonTypeRef{Scalar: PythonScalarInt64}, nil
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return PythonTypeRef{Scalar: PythonScalarUInt64}, nil
	case protoreflect.FloatKind:
		return PythonTypeRef{Scalar: PythonScalarFloat}, nil
	case protoreflect.DoubleKind:
		return PythonTypeRef{Scalar: PythonScalarDouble}, nil
	case protoreflect.EnumKind:
		if err := c.collectEnum(field.Enum); err != nil {
			return PythonTypeRef{}, err
		}
		owner, err := c.ownerForProtoPath(field.Enum.Desc.ParentFile().Path())
		if err != nil {
			return PythonTypeRef{}, err
		}
		return PythonTypeRef{
			ProtoFullName: string(field.Enum.Desc.FullName()),
			ProtoName:     string(field.Enum.Desc.Name()),
			PublicName:    pythonPublicEnumName(field.Enum),
			Owner:         owner,
			IsEnum:        true,
		}, nil
	case protoreflect.MessageKind:
		if field.Message == nil {
			return PythonTypeRef{}, nil
		}
		wkt, ok, err := classifyPythonWellKnownType(field.Message.Desc.FullName())
		if err != nil {
			return PythonTypeRef{}, err
		}
		owner, ownerErr := c.ownerForProtoPath(field.Message.Desc.ParentFile().Path())
		if ownerErr != nil {
			return PythonTypeRef{}, ownerErr
		}
		if ok {
			return PythonTypeRef{
				ProtoFullName: string(field.Message.Desc.FullName()),
				ProtoName:     string(field.Message.Desc.Name()),
				Owner:         owner,
				WellKnownType: wkt,
				IsMessage:     true,
			}, nil
		}
		if err := c.collectMessage(field.Message); err != nil {
			return PythonTypeRef{}, err
		}
		return PythonTypeRef{
			ProtoFullName: string(field.Message.Desc.FullName()),
			ProtoName:     string(field.Message.Desc.Name()),
			PublicName:    pythonPublicTypeName(field.Message),
			Owner:         owner,
			IsMessage:     true,
		}, nil
	default:
		return PythonTypeRef{}, nil
	}
}

func (c *pythonTypeCollector) ownerForProtoPath(protoPath string) (PythonTypeOwner, error) {
	owner := newPythonTypeOwner(protoPath, c.rootProtoPath)
	if owner.IsCurrentFile {
		return owner, nil
	}

	if err := pythonCheckNameCollision(c.importAliases, "import alias", owner.PublicModule.ModuleAlias, owner.ProtoPath+":public"); err != nil {
		return PythonTypeOwner{}, err
	}
	if err := pythonCheckNameCollision(c.importAliases, "import alias", owner.ProtobufModule.ModuleAlias, owner.ProtoPath+":protobuf"); err != nil {
		return PythonTypeOwner{}, err
	}

	for _, existing := range c.graph.Imports {
		if existing.ProtoPath == owner.ProtoPath {
			return owner, nil
		}
	}
	c.graph.Imports = append(c.graph.Imports, PythonImport{
		ProtoPath:      owner.ProtoPath,
		PublicModule:   owner.PublicModule,
		ProtobufModule: owner.ProtobufModule,
	})
	return owner, nil
}

func (c *pythonTypeCollector) checkOwnedPublicName(owner PythonTypeOwner, category, name, ownerKey string) error {
	return pythonCheckOwnedNameCollision(c.ownedPublicNames, owner.PublicModule.ModulePath, category, name, ownerKey)
}

func newPythonTypeOwner(protoPath, currentProtoPath string) PythonTypeOwner {
	isCurrent := protoPath == currentProtoPath
	return PythonTypeOwner{
		ProtoPath:     protoPath,
		IsCurrentFile: isCurrent,
		PublicModule: PythonModuleRef{
			ModulePath:     pythonPublicModulePathForProtoPath(protoPath),
			ModuleAlias:    pythonPublicModuleAliasForProtoPath(protoPath, isCurrent),
			ModuleBasename: pythonPublicBasenameModuleForProtoPath(protoPath),
		},
		ProtobufModule: PythonModuleRef{
			ModulePath:     pythonModulePathForProtoPath(protoPath),
			ModuleAlias:    pythonModuleAliasForProtoPath(protoPath, isCurrent),
			ModuleBasename: pythonBasenameModuleForProtoPath(protoPath),
		},
	}
}

func classifyPythonWellKnownType(fullName protoreflect.FullName) (PythonWellKnownType, bool, error) {
	switch fullName {
	case "google.protobuf.Any":
		return PythonWellKnownTypeAny, true, nil
	case "google.protobuf.Empty":
		return PythonWellKnownTypeEmpty, true, nil
	case "google.protobuf.Timestamp":
		return PythonWellKnownTypeTimestamp, true, nil
	case "google.protobuf.Duration":
		return PythonWellKnownTypeDuration, true, nil
	case "google.protobuf.FieldMask":
		return PythonWellKnownTypeFieldMask, true, nil
	case "google.protobuf.Struct":
		return PythonWellKnownTypeStruct, true, nil
	case "google.protobuf.Value":
		return PythonWellKnownTypeValue, true, nil
	case "google.protobuf.ListValue":
		return PythonWellKnownTypeListValue, true, nil
	case "google.protobuf.BoolValue":
		return PythonWellKnownTypeBoolValue, true, nil
	case "google.protobuf.StringValue":
		return PythonWellKnownTypeStringValue, true, nil
	case "google.protobuf.BytesValue":
		return PythonWellKnownTypeBytesValue, true, nil
	case "google.protobuf.Int32Value":
		return PythonWellKnownTypeInt32Value, true, nil
	case "google.protobuf.UInt32Value":
		return PythonWellKnownTypeUInt32Value, true, nil
	case "google.protobuf.Int64Value":
		return PythonWellKnownTypeInt64Value, true, nil
	case "google.protobuf.UInt64Value":
		return PythonWellKnownTypeUInt64Value, true, nil
	case "google.protobuf.FloatValue":
		return PythonWellKnownTypeFloatValue, true, nil
	case "google.protobuf.DoubleValue":
		return PythonWellKnownTypeDoubleValue, true, nil
	default:
		if isGoogleWellKnownType(fullName) {
			return PythonWellKnownTypeNone, true, fmt.Errorf("well-known type %q is not supported", fullName)
		}
		return PythonWellKnownTypeNone, false, nil
	}
}

func isGoogleWellKnownType(fullName protoreflect.FullName) bool {
	return strings.HasPrefix(string(fullName), "google.protobuf.")
}

func pythonScalarForKind(kind protoreflect.Kind) PythonScalar {
	switch kind {
	case protoreflect.BoolKind:
		return PythonScalarBool
	case protoreflect.StringKind:
		return PythonScalarString
	case protoreflect.BytesKind:
		return PythonScalarBytes
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return PythonScalarInt32
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return PythonScalarUInt32
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return PythonScalarInt64
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return PythonScalarUInt64
	case protoreflect.FloatKind:
		return PythonScalarFloat
	case protoreflect.DoubleKind:
		return PythonScalarDouble
	default:
		return PythonScalarUnknown
	}
}
