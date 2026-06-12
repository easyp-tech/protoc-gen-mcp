package codegen

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// collectPrompts scans all messages in the file for (mcp.options.v1.prompt).
// Returns fail-fast error if any argument field has unsupported type.
func collectPrompts(file *protogen.File) ([]PromptModel, error) {
	var prompts []PromptModel

	for _, message := range file.Messages {
		opts, err := getPromptOptions(message)
		if err != nil {
			return nil, err
		}
		if opts == nil {
			continue
		}

		name := strings.TrimSpace(opts.GetName())
		if name == "" {
			name = toSnakeCase(string(message.Desc.Name()))
		}

		description := strings.TrimSpace(opts.GetDescription())
		if description == "" {
			commentMeta := parseCommentBlock(message.Comments.Leading)
			description = commentMeta.Description
		}

		prompt := PromptModel{
			ProtoFullName: string(message.Desc.FullName()),
			ProtoName:     string(message.Desc.Name()),
			Name:          name,
			Title:         strings.TrimSpace(opts.GetTitle()),
			Description:   description,
			Icons:         opts.GetIcons(),
			Input:         newTypeRef(message),
		}

		for _, field := range message.Fields {
			arg, err := collectPromptArgument(message, field)
			if err != nil {
				return nil, err
			}
			prompt.Arguments = append(prompt.Arguments, arg)
		}

		prompts = append(prompts, prompt)
	}

	return prompts, nil
}

func collectPromptArgument(message *protogen.Message, field *protogen.Field) (PromptArgumentModel, error) {
	msgName := string(message.Desc.Name())
	fieldName := string(field.Desc.Name())

	// Reject oneof fields (but not synthetic optional oneofs).
	if field.Oneof != nil && !field.Desc.HasOptionalKeyword() {
		return PromptArgumentModel{}, fmt.Errorf(
			"prompt %q field %q is inside oneof; prompt arguments must be top-level",
			msgName, fieldName,
		)
	}

	// Reject map fields.
	if field.Desc.IsMap() {
		return PromptArgumentModel{}, fmt.Errorf(
			"prompt %q field %q is a map; prompt arguments must be scalar or enum",
			msgName, fieldName,
		)
	}

	// Reject repeated fields.
	if field.Desc.IsList() {
		return PromptArgumentModel{}, fmt.Errorf(
			"prompt %q field %q is repeated; prompt arguments must be singular",
			msgName, fieldName,
		)
	}

	// Reject message-typed fields (nested messages).
	if field.Desc.Kind() == protoreflect.MessageKind || field.Desc.Kind() == protoreflect.GroupKind {
		return PromptArgumentModel{}, fmt.Errorf(
			"prompt %q field %q has unsupported type message; prompt arguments must be scalar or enum",
			msgName, fieldName,
		)
	}

	// Determine description from field options or leading comment.
	desc := ""
	fieldOpts, err := getFieldOptions(field)
	if err != nil {
		return PromptArgumentModel{}, err
	}
	if fieldOpts != nil && strings.TrimSpace(fieldOpts.GetDescription()) != "" {
		desc = strings.TrimSpace(fieldOpts.GetDescription())
	} else {
		commentMeta := parseCommentBlock(field.Comments.Leading)
		desc = commentMeta.Description
	}

	// Required: singular without optional keyword.
	required := !field.Desc.HasOptionalKeyword()

	return PromptArgumentModel{
		ProtoName:   fieldName,
		Name:        field.Desc.JSONName(),
		Description: desc,
		Required:    required,
	}, nil
}
