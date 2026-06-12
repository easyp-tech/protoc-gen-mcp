package codegen

import (
	"fmt"
	"strings"

	"github.com/easyp-tech/protoc-gen-mcp/internal/schema"
	mcpoptionsv1 "github.com/easyp-tech/protoc-gen-mcp/mcp/options/v1"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type serviceMetadata struct {
	Namespace   string
	Description string
	Icons       []*mcpoptionsv1.Icon
}

type methodMetadata struct {
	Name        string
	Title       string
	Description string
	Examples    []string
	Hidden      bool
	Disabled    bool
	Deprecated  bool

	Annotations *mcpoptionsv1.ToolAnnotations
	Icons       []*mcpoptionsv1.Icon
	TaskSupport mcpoptionsv1.TaskSupport
}

type fieldMetadata struct {
	schema.FieldMetadata
}

func loadServiceMetadata(service *protogen.Service) (serviceMetadata, error) {
	commentMetadata := parseCommentBlock(service.Comments.Leading)
	metadata := serviceMetadata{
		Description: commentMetadata.Description,
	}

	options, err := getServiceOptions(service)
	if err != nil {
		return serviceMetadata{}, err
	}
	if options == nil {
		return metadata, nil
	}

	metadata.Namespace = strings.TrimSpace(options.GetNamespace())
	if strings.TrimSpace(options.GetDescription()) != "" {
		metadata.Description = strings.TrimSpace(options.GetDescription())
	}
	metadata.Icons = options.GetIcons()

	return metadata, nil
}

func loadMethodMetadata(method *protogen.Method) (methodMetadata, error) {
	commentMetadata := parseCommentBlock(method.Comments.Leading)
	metadata := methodMetadata{
		Name:        string(method.Desc.Name()),
		Description: commentMetadata.Description,
		Examples:    commentMetadata.Examples,
	}

	options, err := getMethodOptions(method)
	if err != nil {
		return methodMetadata{}, err
	}

	// Read standard protobuf deprecated option from method descriptor.
	// This is independent of MCP options and must be checked unconditionally.
	if stdOpts, ok := method.Desc.Options().(*descriptorpb.MethodOptions); ok && stdOpts != nil {
		metadata.Deprecated = stdOpts.GetDeprecated()
	}

	if options == nil {
		return metadata, nil
	}

	metadata.Hidden = options.GetHidden()
	metadata.Annotations = options.GetAnnotations()
	metadata.Icons = options.GetIcons()
	if exec := options.GetExecution(); exec != nil {
		metadata.TaskSupport = exec.GetTaskSupport()
	}

	if strings.TrimSpace(options.GetName()) != "" {
		metadata.Name = strings.TrimSpace(options.GetName())
	}
	metadata.Title = strings.TrimSpace(options.GetTitle())
	if strings.TrimSpace(options.GetDescription()) != "" {
		metadata.Description = strings.TrimSpace(options.GetDescription())
	}

	return metadata, nil
}

func loadFieldMetadata(field *protogen.Field) (fieldMetadata, error) {
	commentMetadata := parseCommentBlock(field.Comments.Leading)
	metadata := fieldMetadata{
		FieldMetadata: schema.FieldMetadata{
			Metadata: schema.Metadata{
				Description: commentMetadata.Description,
				Examples:    commentMetadata.Examples,
			},
		},
	}

	options, err := getFieldOptions(field)
	if err != nil {
		return fieldMetadata{}, err
	}
	if options == nil {
		return metadata, nil
	}

	if strings.TrimSpace(options.GetDescription()) != "" {
		metadata.Description = strings.TrimSpace(options.GetDescription())
	}

	// Typed examples from FieldOptions.examples
	if protoExamples := options.GetExamples(); len(protoExamples) > 0 {
		typedExamples := make([]any, 0, len(protoExamples))
		for _, ev := range protoExamples {
			if materialized, ok := materializeExampleValue(ev); ok {
				typedExamples = append(typedExamples, materialized)
			}
		}
		if len(typedExamples) > 0 {
			metadata.TypedExamples = typedExamples
		}
	}

	// Default value from FieldOptions.default_value
	if dv := options.GetDefaultValue(); dv != nil {
		if materialized, ok := materializeExampleValue(dv); ok {
			metadata.Default = materialized
			metadata.HasDefault = true
		}
	}

	// String validation constraints
	metadata.Pattern = options.GetPattern()
	metadata.Format = options.GetFormat()
	metadata.MinLength = options.MinLength
	metadata.MaxLength = options.MaxLength

	// Number validation constraints
	metadata.Minimum = options.Minimum
	metadata.Maximum = options.Maximum
	metadata.ExclusiveMinimum = options.ExclusiveMinimum
	metadata.ExclusiveMaximum = options.ExclusiveMaximum
	metadata.MultipleOf = options.MultipleOf

	// Array constraints
	metadata.MinItems = options.MinItems
	metadata.MaxItems = options.MaxItems
	metadata.UniqueItems = options.GetUniqueItems()
	
	// ReadOnly constraints
	metadata.ReadOnly = options.GetReadOnly()

	return metadata, nil
}

type commentMetadata struct {
	Description string
	Examples    []string
}

func parseCommentBlock(comments protogen.Comments) commentMetadata {
	trimmed := strings.TrimSpace(string(comments))
	if trimmed == "" {
		return commentMetadata{}
	}

	lines := strings.Split(trimmed, "\n")
	descriptionLines := make([]string, 0, len(lines))
	examples := make([]string, 0)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "example:"):
			example := strings.TrimSpace(line[len("example:"):])
			if example != "" {
				examples = append(examples, example)
			}
		case strings.HasPrefix(lower, "examples:"):
			rest := strings.TrimSpace(line[len("examples:"):])
			if rest != "" {
				for _, part := range strings.Split(rest, "|") {
					part = strings.TrimSpace(part)
					if part != "" {
						examples = append(examples, part)
					}
				}
			}
		default:
			descriptionLines = append(descriptionLines, line)
		}
	}

	return commentMetadata{
		Description: strings.Join(descriptionLines, "\n"),
		Examples:    examples,
	}
}

func getServiceOptions(service *protogen.Service) (*mcpoptionsv1.ServiceOptions, error) {
	value, err := getExtension(service.Desc.Options(), mcpoptionsv1.E_Service)
	if err != nil || value == nil {
		return nil, err
	}

	options, ok := value.(*mcpoptionsv1.ServiceOptions)
	if !ok {
		return nil, fmt.Errorf("service %s returned unexpected service options type %T", service.Desc.FullName(), value)
	}

	return options, nil
}

func getMethodOptions(method *protogen.Method) (*mcpoptionsv1.MethodOptions, error) {
	value, err := getExtension(method.Desc.Options(), mcpoptionsv1.E_Method)
	if err != nil || value == nil {
		return nil, err
	}

	options, ok := value.(*mcpoptionsv1.MethodOptions)
	if !ok {
		return nil, fmt.Errorf("method %s returned unexpected method options type %T", method.Desc.FullName(), value)
	}

	return options, nil
}

func getFieldOptions(field *protogen.Field) (*mcpoptionsv1.FieldOptions, error) {
	value, err := getExtension(field.Desc.Options(), mcpoptionsv1.E_Field)
	if err != nil || value == nil {
		return nil, err
	}

	options, ok := value.(*mcpoptionsv1.FieldOptions)
	if !ok {
		return nil, fmt.Errorf("field %s returned unexpected field options type %T", field.Desc.FullName(), value)
	}

	return options, nil
}

func getExtension(options proto.Message, extension protoreflect.ExtensionType) (any, error) {
	if options == nil {
		return nil, nil
	}
	if !proto.HasExtension(options, extension) {
		return nil, nil
	}

	return proto.GetExtension(options, extension), nil
}

type messageMetadata struct {
	Title       string
	Description string
	// TypedExamples holds materialized ExampleObject values from MessageOptions.examples.
	TypedExamples []any
}

type enumMetadata struct {
	Title       string
	Description string
}

type enumValueMetadata struct {
	Description string
	Hidden      bool
}

func loadMessageMetadata(message *protogen.Message) (messageMetadata, error) {
	commentMetadata := parseCommentBlock(message.Comments.Leading)
	metadata := messageMetadata{
		Description: commentMetadata.Description,
	}

	options, err := getMessageOptions(message)
	if err != nil {
		return messageMetadata{}, err
	}
	if options == nil {
		return metadata, nil
	}

	metadata.Title = strings.TrimSpace(options.GetTitle())
	if strings.TrimSpace(options.GetDescription()) != "" {
		metadata.Description = strings.TrimSpace(options.GetDescription())
	}

	if protoExamples := options.GetExamples(); len(protoExamples) > 0 {
		typedExamples := make([]any, 0, len(protoExamples))
		for _, obj := range protoExamples {
			typedExamples = append(typedExamples, materializeExampleObject(obj))
		}
		if len(typedExamples) > 0 {
			metadata.TypedExamples = typedExamples
		}
	}

	return metadata, nil
}

func loadEnumMetadata(enum *protogen.Enum) (enumMetadata, error) {
	metadata := enumMetadata{}

	options, err := getEnumOptions(enum)
	if err != nil {
		return enumMetadata{}, err
	}
	if options == nil {
		return metadata, nil
	}

	metadata.Title = strings.TrimSpace(options.GetTitle())
	metadata.Description = strings.TrimSpace(options.GetDescription())

	return metadata, nil
}

func loadEnumValueMetadata(enumValue *protogen.EnumValue) (enumValueMetadata, error) {
	metadata := enumValueMetadata{}

	options, err := getEnumValueOptions(enumValue)
	if err != nil {
		return enumValueMetadata{}, err
	}
	if options == nil {
		return metadata, nil
	}

	metadata.Description = strings.TrimSpace(options.GetDescription())
	metadata.Hidden = options.GetHidden()

	return metadata, nil
}

func getMessageOptions(message *protogen.Message) (*mcpoptionsv1.MessageOptions, error) {
	value, err := getExtension(message.Desc.Options(), mcpoptionsv1.E_Message)
	if err != nil || value == nil {
		return nil, err
	}

	options, ok := value.(*mcpoptionsv1.MessageOptions)
	if !ok {
		return nil, fmt.Errorf("message %s returned unexpected message options type %T", message.Desc.FullName(), value)
	}

	return options, nil
}

func getEnumOptions(enum *protogen.Enum) (*mcpoptionsv1.EnumOptions, error) {
	value, err := getExtension(enum.Desc.Options(), mcpoptionsv1.E_Enum)
	if err != nil || value == nil {
		return nil, err
	}

	options, ok := value.(*mcpoptionsv1.EnumOptions)
	if !ok {
		return nil, fmt.Errorf("enum %s returned unexpected enum options type %T", enum.Desc.FullName(), value)
	}

	return options, nil
}

func getEnumValueOptions(enumValue *protogen.EnumValue) (*mcpoptionsv1.EnumValueOptions, error) {
	value, err := getExtension(enumValue.Desc.Options(), mcpoptionsv1.E_EnumValue)
	if err != nil || value == nil {
		return nil, err
	}

	options, ok := value.(*mcpoptionsv1.EnumValueOptions)
	if !ok {
		return nil, fmt.Errorf("enum value %s returned unexpected enum value options type %T", enumValue.Desc.FullName(), value)
	}

	return options, nil
}

func getPromptOptions(message *protogen.Message) (*mcpoptionsv1.PromptOptions, error) {
	value, err := getExtension(message.Desc.Options(), mcpoptionsv1.E_Prompt)
	if err != nil || value == nil {
		return nil, err
	}

	options, ok := value.(*mcpoptionsv1.PromptOptions)
	if !ok {
		return nil, fmt.Errorf("message %s returned unexpected prompt options type %T", message.Desc.FullName(), value)
	}

	return options, nil
}

func getResourceOptions(message *protogen.Message) (*mcpoptionsv1.ResourceOptions, error) {
	value, err := getExtension(message.Desc.Options(), mcpoptionsv1.E_Resource)
	if err != nil || value == nil {
		return nil, err
	}

	options, ok := value.(*mcpoptionsv1.ResourceOptions)
	if !ok {
		return nil, fmt.Errorf("message %s returned unexpected resource options type %T", message.Desc.FullName(), value)
	}

	return options, nil
}

// materializeExampleValue converts a proto ExampleValue to a Go any value
// suitable for JSON Schema examples/defaults.
// Returns (nil, false) if the value is nil or has no kind set.
func materializeExampleValue(v *mcpoptionsv1.ExampleValue) (any, bool) {
	if v == nil {
		return nil, false
	}
	switch k := v.GetKind().(type) {
	case *mcpoptionsv1.ExampleValue_StringValue:
		return k.StringValue, true
	case *mcpoptionsv1.ExampleValue_NumberValue:
		return k.NumberValue, true
	case *mcpoptionsv1.ExampleValue_BoolValue:
		return k.BoolValue, true
	case *mcpoptionsv1.ExampleValue_NullValue:
		if k.NullValue {
			return nil, true
		}
		return nil, false
	case *mcpoptionsv1.ExampleValue_ObjectValue:
		return materializeExampleObject(k.ObjectValue), true
	case *mcpoptionsv1.ExampleValue_ArrayValue:
		return materializeExampleArray(k.ArrayValue), true
	default:
		return nil, false
	}
}

// materializeExampleObject converts a proto ExampleObject to a map[string]any,
// recursively processing each property value.
func materializeExampleObject(obj *mcpoptionsv1.ExampleObject) map[string]any {
	if obj == nil || len(obj.GetProperties()) == 0 {
		return map[string]any{}
	}
	result := make(map[string]any, len(obj.GetProperties()))
	for key, val := range obj.GetProperties() {
		materialized, ok := materializeExampleValue(val)
		if ok {
			result[key] = materialized
		}
	}
	return result
}

// materializeExampleArray converts a proto ExampleArray to a []any,
// recursively processing each item.
func materializeExampleArray(arr *mcpoptionsv1.ExampleArray) []any {
	if arr == nil || len(arr.GetItems()) == 0 {
		return []any{}
	}
	result := make([]any, 0, len(arr.GetItems()))
	for _, item := range arr.GetItems() {
		materialized, ok := materializeExampleValue(item)
		if ok {
			result = append(result, materialized)
		}
	}
	return result
}
