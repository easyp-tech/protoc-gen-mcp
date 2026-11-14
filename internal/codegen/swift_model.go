package codegen

import (
	"encoding/json"
	"fmt"
	"sort"

	mcpoptionsv1 "github.com/easyp-tech/protoc-gen-mcp/mcp/options/v1"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// SwiftFileModel is what render_swift.go needs beyond the shared FileModel:
// Swift type names of swift-protobuf messages and the ProtoJSON default
// tables swift-protobuf cannot derive at run time.
type SwiftFileModel struct {
	ProtoPath               string
	GeneratedFilenamePrefix string
	FileIdentifier          string
	Services                []SwiftServiceModel
	Prompts                 []PromptModel
	Resources               []SwiftResourceModel
	Shapes                  []SwiftShapeMessage
	// MessageTypes lists every message of the file for google.protobuf.Any.
	MessageTypes []string
	Factory      ServerFactoryModel
}

type SwiftServiceModel struct {
	ProtoName    string
	Namespace    string
	HandlerName  string
	RegisterName string
	Methods      []SwiftMethodModel
}

type SwiftMethodModel struct {
	Method         MethodModel
	FuncName       string
	SchemaConst    string
	InputType      string
	OutputType     string
	OutputFullName string
	DescriptorJSON string
}

type SwiftResourceModel struct {
	Resource       ResourceModel
	Kind           resourceContentKind
	TypeName       string
	JSONField      string
	DescriptorJSON string
	Params         []string
}

type SwiftShapeMessage struct {
	FullName string
	Fields   []SwiftShapeField
}

// SwiftShapeField is one ProtoJSONField literal: Default is a Swift Value
// expression or empty, Nested a message full name or empty.
type SwiftShapeField struct {
	JSONName    string
	Default     string
	Nested      string
	Cardinality string
}

func CollectSwiftFileModel(file *protogen.File, model FileModel) (SwiftFileModel, error) {
	if model.Options.Language != LanguageSwift {
		return SwiftFileModel{}, fmt.Errorf("swift model requires lang=swift, got %q", model.Options.Language)
	}
	methods, err := collectTypeScriptDescriptorMethods(file)
	if err != nil {
		return SwiftFileModel{}, err
	}
	methodByFullName := make(map[string]*protogen.Method, len(methods))
	for _, method := range methods {
		methodByFullName[string(method.Desc.FullName())] = method
	}

	shapes := newSwiftShapeCollector()
	swiftModel := SwiftFileModel{
		ProtoPath:               model.ProtoPath,
		GeneratedFilenamePrefix: model.GeneratedFilenamePrefix,
		FileIdentifier:          swiftFileIdentifier(model.ProtoPath),
		Prompts:                 model.Prompts,
		Factory:                 model.Factory,
	}
	for _, service := range model.Services {
		serviceModel := SwiftServiceModel{
			ProtoName:    service.ProtoName,
			Namespace:    service.Namespace,
			HandlerName:  typescriptHandlerName(service.ProtoName),
			RegisterName: typescriptRegisterName(service.ProtoName),
		}
		for _, method := range service.Methods {
			descriptor := methodByFullName[method.ProtoFullName]
			if descriptor == nil {
				return SwiftFileModel{}, fmt.Errorf("method %s: descriptor not found for Swift render", method.ProtoFullName)
			}
			if err := validateSwiftIcons(method.Icons); err != nil {
				return SwiftFileModel{}, fmt.Errorf("method %s: %w", method.ProtoFullName, err)
			}
			descriptorJSON, err := swiftToolDescriptorJSON(method)
			if err != nil {
				return SwiftFileModel{}, err
			}
			shapes.add(descriptor.Output.Desc)
			serviceModel.Methods = append(serviceModel.Methods, SwiftMethodModel{
				Method:         method,
				FuncName:       swiftMethodName(method.ProtoName),
				SchemaConst:    jvmLowerCamelIdentifier(service.ProtoName) + jvmExportedIdentifier(method.ProtoName),
				InputType:      swiftMessageTypeName(descriptor.Input.Desc),
				OutputType:     swiftMessageTypeName(descriptor.Output.Desc),
				OutputFullName: string(descriptor.Output.Desc.FullName()),
				DescriptorJSON: descriptorJSON,
			})
		}
		swiftModel.Services = append(swiftModel.Services, serviceModel)
	}

	for _, resource := range model.Resources {
		descriptorJSON, err := resourceDescriptorJSON(resource)
		if err != nil {
			return SwiftFileModel{}, err
		}
		resourceModel := SwiftResourceModel{
			Resource:       resource,
			Kind:           resourceContent(resource),
			DescriptorJSON: descriptorJSON,
			Params:         swiftParamIdentifiers(resource.Params),
		}
		if resourceModel.Kind != resourceContentSourceFile {
			message := swiftMessage(file, resource.Output.ProtoFullName)
			if message == nil {
				return SwiftFileModel{}, fmt.Errorf("resource %s: message not found for Swift render", resource.ProtoFullName)
			}
			resourceModel.TypeName = swiftMessageTypeName(message.Desc)
			switch resourceModel.Kind {
			case resourceContentProtoJSON:
				shapes.add(message.Desc)
			case resourceContentTextField, resourceContentBlobField:
				field := message.Desc.Fields().ByName(protoreflect.Name(resource.ContentField))
				if field == nil {
					return SwiftFileModel{}, fmt.Errorf("resource %s: content field %q not found", resource.ProtoFullName, resource.ContentField)
				}
				resourceModel.JSONField = field.JSONName()
			}
		}
		swiftModel.Resources = append(swiftModel.Resources, resourceModel)
	}
	swiftModel.Shapes = shapes.messages()
	swiftModel.MessageTypes = swiftFileMessageTypes(file.Messages)
	return swiftModel, nil
}

func swiftFileMessageTypes(messages []*protogen.Message) []string {
	var names []string
	for _, message := range messages {
		if message.Desc.IsMapEntry() {
			continue
		}
		names = append(names, swiftMessageTypeName(message.Desc))
		names = append(names, swiftFileMessageTypes(message.Messages)...)
	}
	return names
}

// swiftToolDescriptorJSON holds the tools/list fields other than name and
// schemas, decoded by the runtime into the SDK Tool type.
func swiftToolDescriptorJSON(method MethodModel) (string, error) {
	descriptor := map[string]any{}
	if method.Title != "" {
		descriptor["title"] = method.Title
	}
	if method.Description != "" {
		descriptor["description"] = method.Description
	}
	if ann := method.Annotations; ann != nil {
		annotations := map[string]any{}
		if ann.ReadOnlyHint {
			annotations["readOnlyHint"] = true
		}
		if ann.DestructiveHint != nil {
			annotations["destructiveHint"] = ann.GetDestructiveHint()
		}
		if ann.IdempotentHint {
			annotations["idempotentHint"] = true
		}
		if ann.OpenWorldHint != nil {
			annotations["openWorldHint"] = ann.GetOpenWorldHint()
		}
		if ann.Title != "" {
			annotations["title"] = ann.Title
		}
		descriptor["annotations"] = annotations
	}
	if len(method.Icons) > 0 {
		icons := make([]map[string]any, 0, len(method.Icons))
		for _, icon := range method.Icons {
			icons = append(icons, iconMap(icon))
		}
		descriptor["icons"] = icons
	}
	if meta := toolMetaJSON(method.AppUI); meta != "" {
		descriptor["_meta"] = json.RawMessage(meta)
	}
	encoded, err := json.Marshal(descriptor)
	if err != nil {
		return "", fmt.Errorf("method %s: encode tool descriptor: %w", method.ProtoFullName, err)
	}
	return string(encoded), nil
}

// swiftPromptDescriptorJSON holds the prompts/list entry other than name.
func swiftPromptDescriptorJSON(prompt PromptModel) (string, error) {
	descriptor := map[string]any{}
	if prompt.Title != "" {
		descriptor["title"] = prompt.Title
	}
	if prompt.Description != "" {
		descriptor["description"] = prompt.Description
	}
	arguments := make([]map[string]any, 0, len(prompt.Arguments))
	for _, argument := range prompt.Arguments {
		arguments = append(arguments, map[string]any{
			"name":        argument.Name,
			"description": argument.Description,
			"required":    argument.Required,
		})
	}
	descriptor["arguments"] = arguments
	encoded, err := json.Marshal(descriptor)
	if err != nil {
		return "", fmt.Errorf("prompt %s: encode descriptor: %w", prompt.ProtoFullName, err)
	}
	return string(encoded), nil
}

// validateSwiftIcons rejects themes the Swift SDK Icon type cannot decode.
func validateSwiftIcons(icons []*mcpoptionsv1.Icon) error {
	for _, icon := range icons {
		switch icon.GetTheme() {
		case "", "light", "dark":
		default:
			return fmt.Errorf("unsupported Swift icon theme %q", icon.GetTheme())
		}
	}
	return nil
}

// swiftShapeCollector gathers, for every message a tool or resource prints,
// the protojson EmitDefaultValues behaviour of each field.
type swiftShapeCollector struct {
	seen  map[protoreflect.FullName]SwiftShapeMessage
	order []protoreflect.FullName
}

func newSwiftShapeCollector() *swiftShapeCollector {
	return &swiftShapeCollector{seen: map[protoreflect.FullName]SwiftShapeMessage{}}
}

func (c *swiftShapeCollector) add(message protoreflect.MessageDescriptor) {
	if message == nil || isSwiftProtobufWellKnownFile(message.ParentFile()) || message.IsMapEntry() {
		return
	}
	if _, ok := c.seen[message.FullName()]; ok {
		return
	}
	c.seen[message.FullName()] = SwiftShapeMessage{}
	c.order = append(c.order, message.FullName())
	shape := SwiftShapeMessage{FullName: string(message.FullName())}
	fields := message.Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		shapeField := SwiftShapeField{JSONName: field.JSONName(), Cardinality: "single"}
		var nested protoreflect.MessageDescriptor
		switch {
		case field.IsMap():
			shapeField.Default = ".object([:])"
			shapeField.Cardinality = "map"
			if value := field.MapValue(); value.Kind() == protoreflect.MessageKind {
				nested = value.Message()
			}
		case field.IsList():
			shapeField.Default = ".array([])"
			shapeField.Cardinality = "list"
			if field.Kind() == protoreflect.MessageKind || field.Kind() == protoreflect.GroupKind {
				nested = field.Message()
			}
		case field.Kind() == protoreflect.MessageKind || field.Kind() == protoreflect.GroupKind:
			nested = field.Message()
		case !field.HasPresence():
			shapeField.Default = swiftScalarDefault(field)
		}
		if nested != nil && !isSwiftProtobufWellKnownFile(nested.ParentFile()) {
			shapeField.Nested = string(nested.FullName())
			c.add(nested)
		}
		if shapeField.Default != "" || shapeField.Nested != "" {
			shape.Fields = append(shape.Fields, shapeField)
		}
	}
	c.seen[message.FullName()] = shape
}

func (c *swiftShapeCollector) messages() []SwiftShapeMessage {
	result := make([]SwiftShapeMessage, 0, len(c.order))
	for _, name := range c.order {
		result = append(result, c.seen[name])
	}
	sort.Slice(result, func(i, j int) bool { return result[i].FullName < result[j].FullName })
	return result
}

// swiftScalarDefault is what protojson prints for an unset field without presence.
func swiftScalarDefault(field protoreflect.FieldDescriptor) string {
	switch field.Kind() {
	case protoreflect.BoolKind:
		return ".bool(false)"
	case protoreflect.StringKind, protoreflect.BytesKind:
		return `.string("")`
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return `.string("0")`
	case protoreflect.EnumKind:
		enum := field.Enum()
		if enum.FullName() == "google.protobuf.NullValue" {
			return ".null"
		}
		value := enum.Values().ByNumber(0)
		if value == nil {
			value = enum.Values().Get(0)
		}
		return ".string(" + swiftStringLiteral(string(value.Name())) + ")"
	default:
		return ".int(0)"
	}
}
