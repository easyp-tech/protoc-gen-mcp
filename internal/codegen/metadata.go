package codegen

import (
	"fmt"
	"strings"

	mcpoptionsv1 "github.com/easyp-tech/protoc-gen-mcp/api/mcp/options/v1"
	"github.com/easyp-tech/protoc-gen-mcp/internal/schema"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type serviceMetadata struct {
	Namespace   string
	Description string
}

type methodMetadata struct {
	Name        string
	Title       string
	Description string
	Examples    []string
	Hidden      bool
	Disabled    bool
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
	if options == nil {
		return metadata, nil
	}

	if strings.TrimSpace(options.GetName()) != "" {
		metadata.Name = strings.TrimSpace(options.GetName())
	}
	metadata.Title = strings.TrimSpace(options.GetTitle())
	if strings.TrimSpace(options.GetDescription()) != "" {
		metadata.Description = strings.TrimSpace(options.GetDescription())
	}
	if len(options.GetExamples()) > 0 {
		metadata.Examples = cloneStrings(options.GetExamples())
	}
	metadata.Hidden = options.GetHidden()
	metadata.Disabled = options.GetDisabled()

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

	if options.Required != nil && options.Optional != nil {
		return fieldMetadata{}, fmt.Errorf(
			"field %s cannot set both mcp.options.v1.field.required and optional",
			field.Desc.FullName(),
		)
	}
	if oneof := field.Desc.ContainingOneof(); oneof != nil && !oneof.IsSynthetic() &&
		(options.Required != nil || options.Optional != nil) {
		return fieldMetadata{}, fmt.Errorf(
			"field %s cannot override required/optional while participating in oneof %s",
			field.Desc.FullName(),
			oneof.FullName(),
		)
	}
	if options.Required != nil {
		required := options.GetRequired()
		metadata.RequiredOverride = &required
	}
	if options.Optional != nil {
		optional := options.GetOptional()
		metadata.OptionalOverride = &optional
	}
	if strings.TrimSpace(options.GetDescription()) != "" {
		metadata.Description = strings.TrimSpace(options.GetDescription())
	}
	if len(options.GetExamples()) > 0 {
		metadata.Examples = cloneStrings(options.GetExamples())
	}

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

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	cloned := make([]string, len(values))
	copy(cloned, values)

	return cloned
}
