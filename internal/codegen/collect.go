package codegen

import (
	"fmt"

	"github.com/easyp-tech/protoc-gen-mcp/internal/schema"
	"github.com/google/jsonschema-go/jsonschema"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// CollectFileModel normalizes a protobuf file into the shared semantic IR used
// by language-specific renderers.
func CollectFileModel(file *protogen.File, opts Options) (FileModel, error) {
	if file.Desc.Syntax() != protoreflect.Proto3 {
		return FileModel{}, fmt.Errorf("only proto3 files are supported in MVP: %s", file.Desc.Path())
	}

	model := FileModel{
		ProtoPath:               file.Desc.Path(),
		GeneratedFilenamePrefix: file.GeneratedFilenamePrefix,
		Options:                 opts,
		Services:                make([]ServiceModel, 0, len(file.Services)),
	}
	var pythonGraphMethods []*protogen.Method

	for _, service := range file.Services {
		serviceMetadata, err := loadServiceMetadata(service)
		if err != nil {
			return FileModel{}, err
		}

		serviceModel := ServiceModel{
			ProtoFullName: string(service.Desc.FullName()),
			ProtoName:     string(service.Desc.Name()),
			Namespace:     serviceMetadata.Namespace,
			Description:   serviceMetadata.Description,
			Icons:         serviceMetadata.Icons,
			Methods:       make([]MethodModel, 0, len(service.Methods)),
		}

		for _, method := range service.Methods {
			methodMetadata, include, err := selectToolMethod(method)
			if err != nil {
				return FileModel{}, err
			}
			if !include {
				continue
			}

			inputSchema, err := generateSchema(method.Input)
			if err != nil {
				return FileModel{}, fmt.Errorf("input schema for %s: %w", method.Desc.FullName(), err)
			}
			if len(methodMetadata.Examples) > 0 {
				inputSchema.Examples = make([]any, 0, len(methodMetadata.Examples))
				for _, example := range methodMetadata.Examples {
					inputSchema.Examples = append(inputSchema.Examples, example)
				}
			}

			if methodMetadata.Deprecated {
				inputSchema.Deprecated = true
			}

			outputSchema, err := generateSchema(method.Output)
			if err != nil {
				return FileModel{}, fmt.Errorf("output schema for %s: %w", method.Desc.FullName(), err)
			}

			inputSchemaJSON, err := schema.MarshalJSON(inputSchema)
			if err != nil {
				return FileModel{}, fmt.Errorf("marshal input schema for %s: %w", method.Desc.FullName(), err)
			}
			outputSchemaJSON, err := schema.MarshalJSON(outputSchema)
			if err != nil {
				return FileModel{}, fmt.Errorf("marshal output schema for %s: %w", method.Desc.FullName(), err)
			}

			methodModel := MethodModel{
				ProtoFullName:    string(method.Desc.FullName()),
				ProtoName:        string(method.Desc.Name()),
				Name:             methodMetadata.Name,
				Title:            methodMetadata.Title,
				Description:      methodMetadata.Description,
				Examples:         methodMetadata.Examples,
				Deprecated:       methodMetadata.Deprecated,
				Input:            newTypeRef(method.Input),
				Output:           newTypeRef(method.Output),
				InputSchemaJSON:  inputSchemaJSON,
				OutputSchemaJSON: outputSchemaJSON,
				Annotations:      methodMetadata.Annotations,
				Icons:            methodMetadata.Icons,
			}
			if len(methodModel.Icons) == 0 {
				methodModel.Icons = serviceModel.Icons
			}

			serviceModel.Methods = append(serviceModel.Methods, methodModel)
			pythonGraphMethods = append(pythonGraphMethods, method)
		}

		if len(serviceModel.Methods) > 0 {
			model.Services = append(model.Services, serviceModel)
		}
	}

	if opts.Language == LanguagePython {
		includeCurrentFileTypes := len(pythonGraphMethods) == 0
		pythonTypes, err := collectPythonTypeGraphFromMethods(file, opts.PythonRuntime, pythonGraphMethods, includeCurrentFileTypes)
		if err != nil {
			return FileModel{}, err
		}
		model.PythonTypes = &pythonTypes
	}

	return model, nil
}

func selectToolMethod(method *protogen.Method) (methodMetadata, bool, error) {
	if method.Desc.IsStreamingClient() || method.Desc.IsStreamingServer() {
		return methodMetadata{}, false, fmt.Errorf("streaming RPC is not supported: %s", method.Desc.FullName())
	}

	metadata, err := loadMethodMetadata(method)
	if err != nil {
		return methodMetadata{}, false, err
	}
	if metadata.Hidden || metadata.Disabled {
		return metadata, false, nil
	}

	return metadata, true, nil
}

func generateSchema(message *protogen.Message) (*jsonschema.Schema, error) {
	var fieldErr error

	generatedSchema, err := schema.GenerateMessageSchema(message, schema.Options{
		MessageMetadata: func(current *protogen.Message) schema.Metadata {
			metadata, err := loadMessageMetadata(current)
			if err != nil && fieldErr == nil {
				fieldErr = err
			}
			commentMetadata := parseCommentBlock(current.Comments.Leading)
			return schema.Metadata{
				Title:         metadata.Title,
				Description:   metadata.Description,
				Examples:      commentMetadata.Examples,
				TypedExamples: metadata.TypedExamples,
			}
		},
		FieldMetadata: func(field *protogen.Field) schema.FieldMetadata {
			metadata, err := loadFieldMetadata(field)
			if err != nil && fieldErr == nil {
				fieldErr = err
			}
			return metadata.FieldMetadata
		},
		EnumMetadata: func(enum *protogen.Enum) schema.EnumMetadata {
			metadata, err := loadEnumMetadata(enum)
			if err != nil && fieldErr == nil {
				fieldErr = err
			}
			return schema.EnumMetadata{
				Title:       metadata.Title,
				Description: metadata.Description,
			}
		},
		EnumValueMetadata: func(enumValue *protogen.EnumValue) schema.EnumValueMetadata {
			metadata, err := loadEnumValueMetadata(enumValue)
			if err != nil && fieldErr == nil {
				fieldErr = err
			}
			return schema.EnumValueMetadata{
				Description: metadata.Description,
				Hidden:      metadata.Hidden,
			}
		},
	})
	if err != nil {
		return nil, err
	}
	if fieldErr != nil {
		return nil, fieldErr
	}

	return generatedSchema, nil
}

func newTypeRef(message *protogen.Message) TypeRef {
	return TypeRef{
		ProtoFullName:    string(message.Desc.FullName()),
		ProtoDisplayName: string(message.Desc.Name()),
	}
}
