package schema

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	signedMapKeyPattern   = `^-?(0|[1-9][0-9]*)$`
	unsignedMapKeyPattern = `^(0|[1-9][0-9]*)$`
	durationPattern       = `^-?[0-9]+(?:\.[0-9]{1,9})?s$`
)

// Metadata describes human-facing schema metadata.
type Metadata struct {
	Title         string
	Description   string
	Examples      []string
	TypedExamples []any
}

// FieldMetadata extends schema metadata with validation constraints.
type FieldMetadata struct {
	Metadata

	// Typed examples from FieldOptions.examples (materialized).
	// Takes priority over Metadata.Examples (string-based from comments).
	TypedExamples []any

	// Default value from FieldOptions.default_value (materialized).
	Default    any
	HasDefault bool

	// String validation constraints
	Pattern   string
	Format    string
	MinLength uint32
	MaxLength uint32

	// Number validation constraints
	Minimum          float64
	Maximum          float64
	ExclusiveMinimum float64
	ExclusiveMaximum float64
	MultipleOf       float64

	// Array constraints
	MinItems    uint32
	MaxItems    uint32
	UniqueItems bool
}

// EnumMetadata describes human-facing enum schema metadata.
type EnumMetadata struct {
	Title       string
	Description string
}

// EnumValueMetadata describes per-value enum metadata.
type EnumValueMetadata struct {
	Description string
	Hidden      bool
}

// Options configures schema generation from protobuf messages.
type Options struct {
	MessageMetadata   func(*protogen.Message) Metadata
	FieldMetadata     func(*protogen.Field) FieldMetadata
	EnumMetadata      func(*protogen.Enum) EnumMetadata
	EnumValueMetadata func(*protogen.EnumValue) EnumValueMetadata
}

type schemaBuilder struct {
	options  Options
	root     protoreflect.FullName
	building map[protoreflect.FullName]bool
	built    map[protoreflect.FullName]*jsonschema.Schema
	defs     map[string]*jsonschema.Schema
}

// GenerateMessageSchema converts a protobuf message into a JSON Schema object.
func GenerateMessageSchema(message *protogen.Message, options Options) (*jsonschema.Schema, error) {
	if message == nil {
		return nil, fmt.Errorf("schema: message is nil")
	}

	builder := &schemaBuilder{
		options:  options,
		root:     message.Desc.FullName(),
		building: make(map[protoreflect.FullName]bool),
		built:    make(map[protoreflect.FullName]*jsonschema.Schema),
		defs:     make(map[string]*jsonschema.Schema),
	}

	schema, err := builder.generateMessageSchema(message)
	if err != nil {
		return nil, err
	}

	if len(builder.defs) > 0 {
		schema.Defs = builder.cloneDefinitions()
	}

	return schema, nil
}

// MarshalJSON marshals a schema into compact JSON for embedding in generated code.
func MarshalJSON(schema *jsonschema.Schema) (string, error) {
	if schema == nil {
		return "", fmt.Errorf("schema: schema is nil")
	}

	rawJSON, err := json.Marshal(schema)
	if err != nil {
		return "", err
	}

	return string(rawJSON), nil
}

func (builder *schemaBuilder) generateMessageSchema(message *protogen.Message) (*jsonschema.Schema, error) {
	if message == nil {
		return nil, fmt.Errorf("schema: message is nil")
	}

	fullName := message.Desc.FullName()
	if fullName == builder.root && builder.building[fullName] {
		return &jsonschema.Schema{Ref: "#"}, nil
	}
	if builder.building[fullName] {
		builder.ensureDefinition(message)
		return &jsonschema.Schema{Ref: builder.definitionRef(fullName)}, nil
	}

	if cached := builder.built[fullName]; cached != nil {
		if fullName != builder.root {
			if _, exists := builder.defs[builder.definitionName(fullName)]; exists {
				return &jsonschema.Schema{Ref: builder.definitionRef(fullName)}, nil
			}
		}

		return cached.CloneSchemas(), nil
	}

	builder.building[fullName] = true
	defer delete(builder.building, fullName)

	messageMetadata := lookupMessageMetadata(builder.options, message)
	schema := &jsonschema.Schema{
		Type:                 "object",
		Description:          messageMetadata.Description,
		Properties:           make(map[string]*jsonschema.Schema),
		AdditionalProperties: disallowAdditionalProperties(),
	}

	// Apply title if present.
	if messageMetadata.Title != "" {
		schema.Title = messageMetadata.Title
	}

	// TypedExamples take priority over string-based examples.
	if len(messageMetadata.TypedExamples) > 0 {
		schema.Examples = messageMetadata.TypedExamples
	} else {
		schema.Examples = materializeExamples(messageMetadata.Examples)
	}

	for _, field := range message.Fields {
		if shouldSkipField(field) {
			continue
		}

		fieldSchema, err := builder.generateFieldSchema(field)
		if err != nil {
			return nil, err
		}

		propertyName := field.Desc.JSONName()
		required := isRequiredField(field, lookupFieldMetadata(builder.options, field))
		if !required {
			fieldSchema = nullableSchema(fieldSchema)
		}

		schema.Properties[propertyName] = fieldSchema
		if required {
			schema.Required = append(schema.Required, propertyName)
		}
	}

	if len(schema.Properties) == 0 {
		schema.Properties = nil
	}

	if len(schema.Required) == 0 {
		schema.Required = nil
	}
	if len(schema.Examples) == 0 {
		if example, ok := builder.autoMessageExample(message, make(map[protoreflect.FullName]int)); ok {
			schema.Examples = []any{example}
		}
	}

	oneofConstraints, err := generateOneofConstraints(message)
	if err != nil {
		return nil, err
	}
	if len(oneofConstraints) > 0 {
		schema.AllOf = append(schema.AllOf, oneofConstraints...)
	}

	builder.built[fullName] = schema.CloneSchemas()
	builder.fillDefinition(message, schema)

	return schema, nil
}

func (builder *schemaBuilder) generateFieldSchema(field *protogen.Field) (*jsonschema.Schema, error) {
	if field.Desc.IsMap() {
		return builder.generateMapFieldSchema(field)
	}

	fieldMetadata := lookupFieldMetadata(builder.options, field)
	fieldSchema, err := builder.generateSingularSchema(field, fieldMetadata)
	if err != nil {
		return nil, err
	}

	fieldSchema.Description = mergeDescriptions(fieldMetadata.Description, fieldSchema.Description)

	// Apply default value if present.
	if fieldMetadata.HasDefault {
		fieldSchema.Default = marshalDefault(fieldMetadata.Default)
	}

	if field.Desc.IsList() {
		itemExamples := builder.autoSingularExamples(field, make(map[protoreflect.FullName]int))
		if len(itemExamples) > 0 {
			fieldSchema.Examples = itemExamples
		}

		// TypedExamples take priority over string-based examples from comments.
		var arrayExamples []any
		if len(fieldMetadata.TypedExamples) > 0 {
			arrayExamples = fieldMetadata.TypedExamples
		} else {
			arrayExamples = materializeExamples(fieldMetadata.Examples)
		}
		if len(arrayExamples) == 0 {
			arrayExamples = builder.autoFieldExamples(field)
		}

		arraySchema := &jsonschema.Schema{
			Type:        "array",
			Description: fieldSchema.Description,
			Examples:    arrayExamples,
			Items:       fieldSchema,
		}

		// Apply array constraints.
		applyArrayConstraints(arraySchema, fieldMetadata)

		return arraySchema, nil
	}

	// TypedExamples take priority over string-based examples from comments.
	if len(fieldMetadata.TypedExamples) > 0 {
		fieldSchema.Examples = fieldMetadata.TypedExamples
	} else if explicitExamples := materializeExamples(fieldMetadata.Examples); len(explicitExamples) > 0 {
		fieldSchema.Examples = explicitExamples
	} else if autoExamples := builder.autoFieldExamples(field); len(autoExamples) > 0 {
		fieldSchema.Examples = autoExamples
	}

	return fieldSchema, nil
}

func (builder *schemaBuilder) generateMapFieldSchema(field *protogen.Field) (*jsonschema.Schema, error) {
	if field.Message == nil || len(field.Message.Fields) != 2 {
		return nil, fmt.Errorf("schema: map field %s has invalid synthetic entry", field.Desc.FullName())
	}

	fieldMetadata := lookupFieldMetadata(builder.options, field)
	keySchema, err := generateMapKeySchema(field.Message.Fields[0])
	if err != nil {
		return nil, err
	}

	valueSchema, err := builder.generateSingularSchema(field.Message.Fields[1], FieldMetadata{})
	if err != nil {
		return nil, err
	}

	examples := materializeExamples(fieldMetadata.Examples)
	if len(examples) == 0 {
		if autoExample, ok := builder.autoMapExample(field, make(map[protoreflect.FullName]int)); ok {
			examples = []any{autoExample}
		}
	}

	return &jsonschema.Schema{
		Type:                 "object",
		Description:          fieldMetadata.Description,
		Examples:             examples,
		AdditionalProperties: valueSchema,
		PropertyNames:        keySchema,
	}, nil
}

func generateMapKeySchema(field *protogen.Field) (*jsonschema.Schema, error) {
	switch field.Desc.Kind() {
	case protoreflect.StringKind:
		return &jsonschema.Schema{Type: "string"}, nil
	case protoreflect.BoolKind:
		return &jsonschema.Schema{
			Type: "string",
			Enum: []any{"true", "false"},
		}, nil
	case protoreflect.Int32Kind,
		protoreflect.Sint32Kind,
		protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind,
		protoreflect.Sint64Kind,
		protoreflect.Sfixed64Kind:
		return &jsonschema.Schema{
			Type:    "string",
			Pattern: signedMapKeyPattern,
		}, nil
	case protoreflect.Uint32Kind,
		protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind,
		protoreflect.Fixed64Kind:
		return &jsonschema.Schema{
			Type:    "string",
			Pattern: unsignedMapKeyPattern,
		}, nil
	default:
		return nil, fmt.Errorf("schema: unsupported map key kind %q for %s", field.Desc.Kind(), field.Desc.FullName())
	}
}

func generateOneofConstraints(message *protogen.Message) ([]*jsonschema.Schema, error) {
	if message == nil {
		return nil, nil
	}

	grouped := make(map[protoreflect.Name][]*protogen.Field)
	order := make([]protoreflect.Name, 0)
	for _, field := range message.Fields {
		if !isRealOneofField(field) {
			continue
		}

		name := field.Desc.ContainingOneof().Name()
		if _, exists := grouped[name]; !exists {
			order = append(order, name)
		}
		grouped[name] = append(grouped[name], field)
	}

	constraints := make([]*jsonschema.Schema, 0, len(order))
	for _, name := range order {
		fields := grouped[name]
		if len(fields) == 0 {
			continue
		}

		selectedBranches := make([]*jsonschema.Schema, 0, len(fields))
		for _, field := range fields {
			selectedBranches = append(selectedBranches, selectedOneofFieldSchema(field.Desc.JSONName()))
		}

		branches := make([]*jsonschema.Schema, 0, len(fields)+1)
		branches = append(branches, &jsonschema.Schema{
			Not: &jsonschema.Schema{
				AnyOf: cloneSchemaSlice(selectedBranches),
			},
		})

		for _, field := range fields {
			fieldName := field.Desc.JSONName()
			otherSelections := make([]*jsonschema.Schema, 0, len(fields)-1)
			for _, other := range fields {
				if other == field {
					continue
				}
				otherSelections = append(otherSelections, selectedOneofFieldSchema(other.Desc.JSONName()))
			}

			branch := &jsonschema.Schema{
				AllOf: []*jsonschema.Schema{
					selectedOneofFieldSchema(fieldName),
				},
			}
			if len(otherSelections) > 0 {
				branch.AllOf = append(branch.AllOf, &jsonschema.Schema{
					Not: &jsonschema.Schema{
						AnyOf: otherSelections,
					},
				})
			}
			branches = append(branches, branch)
		}

		constraints = append(constraints, &jsonschema.Schema{
			OneOf: branches,
		})
	}

	return constraints, nil
}

func selectedOneofFieldSchema(fieldName string) *jsonschema.Schema {
	return &jsonschema.Schema{
		Required: []string{fieldName},
		Not: &jsonschema.Schema{
			Required: []string{fieldName},
			Properties: map[string]*jsonschema.Schema{
				fieldName: {Type: "null"},
			},
		},
	}
}

func cloneSchemaSlice(values []*jsonschema.Schema) []*jsonschema.Schema {
	if len(values) == 0 {
		return nil
	}

	cloned := make([]*jsonschema.Schema, 0, len(values))
	for _, value := range values {
		if value == nil {
			cloned = append(cloned, nil)
			continue
		}
		cloned = append(cloned, value.CloneSchemas())
	}

	return cloned
}

func (builder *schemaBuilder) generateSingularSchema(field *protogen.Field, metadata FieldMetadata) (*jsonschema.Schema, error) {
	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		return &jsonschema.Schema{Type: "boolean"}, nil
	case protoreflect.StringKind:
		s := &jsonschema.Schema{Type: "string"}
		applyStringConstraints(s, metadata)
		return s, nil
	case protoreflect.BytesKind:
		return &jsonschema.Schema{
			Type:            "string",
			ContentEncoding: "base64",
		}, nil
	case protoreflect.Int32Kind,
		protoreflect.Sint32Kind,
		protoreflect.Sfixed32Kind,
		protoreflect.Uint32Kind,
		protoreflect.Fixed32Kind:
		s := &jsonschema.Schema{Type: "integer"}
		applyNumberConstraints(s, metadata)
		return s, nil
	case protoreflect.Int64Kind,
		protoreflect.Sint64Kind,
		protoreflect.Sfixed64Kind,
		protoreflect.Uint64Kind,
		protoreflect.Fixed64Kind:
		// Int64/Uint64 are encoded as strings in ProtoJSON — number constraints don't apply.
		return &jsonschema.Schema{Type: "string"}, nil
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		// Float/Double use AnyOf schema for ProtoJSON — constraints not applied.
		return protoJSONFloatSchema(false), nil
	case protoreflect.EnumKind:
		enumMeta := lookupEnumMetadata(builder.options, field.Enum)
		enumSchema := &jsonschema.Schema{
			Type: "string",
			Enum: make([]any, 0, len(field.Enum.Values)),
		}
		if enumMeta.Title != "" {
			enumSchema.Title = enumMeta.Title
		}

		var valueDescriptions []string
		for _, enumValue := range field.Enum.Values {
			valueMeta := lookupEnumValueMetadata(builder.options, enumValue)
			if valueMeta.Hidden {
				continue
			}
			enumSchema.Enum = append(enumSchema.Enum, string(enumValue.Desc.Name()))
			if valueMeta.Description != "" {
				valueDescriptions = append(valueDescriptions, string(enumValue.Desc.Name())+": "+valueMeta.Description)
			}
		}

		// Build description: enum-level description first, then per-value descriptions.
		description := enumMeta.Description
		if len(valueDescriptions) > 0 {
			valueDesc := strings.Join(valueDescriptions, "\n")
			if description != "" {
				description += "\n\n" + valueDesc
			} else {
				description = valueDesc
			}
		}
		if description != "" {
			enumSchema.Description = description
		}

		return enumSchema, nil
	case protoreflect.MessageKind:
		fullName := field.Desc.Message().FullName()
		if schema, ok := wellKnownTypeSchema(fullName); ok {
			return schema, nil
		}
		if strings.HasPrefix(string(fullName), "google.protobuf.") {
			return nil, fmt.Errorf("schema: well-known type %q is not supported in MVP", fullName)
		}
		return builder.generateMessageSchema(field.Message)
	default:
		return nil, fmt.Errorf("schema: unsupported field kind %q for %s", field.Desc.Kind(), field.Desc.FullName())
	}
}

func (builder *schemaBuilder) definitionName(fullName protoreflect.FullName) string {
	return string(fullName)
}

func (builder *schemaBuilder) definitionRef(fullName protoreflect.FullName) string {
	return "#/$defs/" + builder.definitionName(fullName)
}

func (builder *schemaBuilder) ensureDefinition(message *protogen.Message) {
	if message == nil {
		return
	}
	if message.Desc.FullName() == builder.root {
		return
	}

	definitionName := builder.definitionName(message.Desc.FullName())
	if _, exists := builder.defs[definitionName]; exists {
		return
	}

	builder.defs[definitionName] = &jsonschema.Schema{}
}

func (builder *schemaBuilder) fillDefinition(message *protogen.Message, schema *jsonschema.Schema) {
	if message == nil || schema == nil {
		return
	}
	if message.Desc.FullName() == builder.root {
		return
	}

	definitionName := builder.definitionName(message.Desc.FullName())
	definition, exists := builder.defs[definitionName]
	if !exists || definition == nil {
		return
	}

	*definition = *schema.CloneSchemas()
}

func (builder *schemaBuilder) cloneDefinitions() map[string]*jsonschema.Schema {
	if len(builder.defs) == 0 {
		return nil
	}

	definitions := make(map[string]*jsonschema.Schema, len(builder.defs))
	for name, schema := range builder.defs {
		if schema == nil {
			definitions[name] = nil
			continue
		}
		definitions[name] = schema.CloneSchemas()
	}

	return definitions
}

func wellKnownTypeSchema(fullName protoreflect.FullName) (*jsonschema.Schema, bool) {
	switch fullName {
	case "google.protobuf.Empty":
		return &jsonschema.Schema{
			Type:                 "object",
			AdditionalProperties: disallowAdditionalProperties(),
		}, true
	case "google.protobuf.Timestamp":
		return &jsonschema.Schema{
			Type:   "string",
			Format: "date-time",
		}, true
	case "google.protobuf.Duration":
		return &jsonschema.Schema{
			Type:    "string",
			Pattern: durationPattern,
		}, true
	case "google.protobuf.FieldMask":
		return &jsonschema.Schema{Type: "string"}, true
	case "google.protobuf.Struct":
		return &jsonschema.Schema{
			Type:                 "object",
			AdditionalProperties: &jsonschema.Schema{},
		}, true
	case "google.protobuf.ListValue":
		return &jsonschema.Schema{
			Type:  "array",
			Items: &jsonschema.Schema{},
		}, true
	case "google.protobuf.Value":
		return &jsonschema.Schema{}, true
	case "google.protobuf.Any":
		return &jsonschema.Schema{
			Type:        "object",
			Description: "ProtoJSON representation of google.protobuf.Any. Provide @type with the embedded message type URL and include the embedded message JSON fields alongside it. For well-known types that use a custom JSON form, send @type plus a value field containing that custom representation.",
			Properties: map[string]*jsonschema.Schema{
				"@type": {
					Type:        "string",
					Description: "Type URL of the embedded protobuf message, usually in the form type.googleapis.com/<full.message.name>.",
				},
			},
			Required:             []string{"@type"},
			AdditionalProperties: &jsonschema.Schema{},
		}, true
	case "google.protobuf.BoolValue":
		return nullableSchema(&jsonschema.Schema{Type: "boolean"}), true
	case "google.protobuf.StringValue":
		return nullableSchema(&jsonschema.Schema{Type: "string"}), true
	case "google.protobuf.BytesValue":
		return nullableSchema(&jsonschema.Schema{
			Type:            "string",
			ContentEncoding: "base64",
		}), true
	case "google.protobuf.Int32Value",
		"google.protobuf.UInt32Value":
		return nullableSchema(&jsonschema.Schema{Type: "integer"}), true
	case "google.protobuf.Int64Value",
		"google.protobuf.UInt64Value":
		return nullableSchema(&jsonschema.Schema{Type: "string"}), true
	case "google.protobuf.FloatValue",
		"google.protobuf.DoubleValue":
		return protoJSONFloatSchema(true), true
	default:
		return nil, false
	}
}

func nullableSchema(schema *jsonschema.Schema) *jsonschema.Schema {
	if schema == nil {
		return &jsonschema.Schema{Type: "null"}
	}

	clone := schema.CloneSchemas()
	if isUnconstrainedSchema(clone) {
		return clone
	}
	if len(clone.Enum) > 0 || clone.Const != nil {
		return &jsonschema.Schema{
			AnyOf: []*jsonschema.Schema{
				clone,
				{Type: "null"},
			},
			Description: clone.Description,
			Examples:    clone.Examples,
		}
	}

	switch {
	case clone.Type != "":
		clone.Types = appendUniqueStrings(nil, clone.Type, "null")
		clone.Type = ""
	case len(clone.Types) > 0:
		clone.Types = appendUniqueStrings(clone.Types, "null")
	case len(clone.AnyOf) > 0:
		hasNull := false
		for _, branch := range clone.AnyOf {
			if branch != nil && branch.Type == "null" {
				hasNull = true
				break
			}
		}
		if !hasNull {
			clone.AnyOf = append(clone.AnyOf, &jsonschema.Schema{Type: "null"})
		}
	default:
		return &jsonschema.Schema{
			AnyOf: []*jsonschema.Schema{
				clone,
				{Type: "null"},
			},
			Description: clone.Description,
			Examples:    clone.Examples,
		}
	}

	return clone
}

func protoJSONFloatSchema(nullable bool) *jsonschema.Schema {
	anyOf := []*jsonschema.Schema{
		{Type: "number"},
		{
			Type: "string",
			Enum: []any{"NaN", "Infinity", "-Infinity"},
		},
	}
	if nullable {
		anyOf = append(anyOf, &jsonschema.Schema{Type: "null"})
	}

	return &jsonschema.Schema{AnyOf: anyOf}
}

func (builder *schemaBuilder) autoFieldExamples(field *protogen.Field) []any {
	if field == nil {
		return nil
	}

	if field.Desc.IsMap() {
		example, ok := builder.autoMapExample(field, make(map[protoreflect.FullName]int))
		if !ok {
			return nil
		}
		return []any{example}
	}

	examples := builder.autoSingularExamples(field, make(map[protoreflect.FullName]int))
	if len(examples) == 0 {
		return nil
	}
	if field.Desc.IsList() {
		arrayExamples := make([]any, 0, len(examples))
		for _, example := range examples {
			arrayExamples = append(arrayExamples, []any{example})
		}
		return arrayExamples
	}

	return examples
}

func (builder *schemaBuilder) autoSingularExamples(field *protogen.Field, seen map[protoreflect.FullName]int) []any {
	if field == nil {
		return nil
	}

	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		return []any{true}
	case protoreflect.StringKind:
		return []any{"example"}
	case protoreflect.BytesKind:
		return []any{"aGVsbG8="}
	case protoreflect.Int32Kind,
		protoreflect.Sint32Kind,
		protoreflect.Sfixed32Kind:
		return []any{-1}
	case protoreflect.Uint32Kind,
		protoreflect.Fixed32Kind:
		return []any{1}
	case protoreflect.Int64Kind,
		protoreflect.Sint64Kind,
		protoreflect.Sfixed64Kind:
		return []any{"-1"}
	case protoreflect.Uint64Kind,
		protoreflect.Fixed64Kind:
		return []any{"1"}
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return []any{1.25, "NaN"}
	case protoreflect.EnumKind:
		if field.Enum == nil || len(field.Enum.Values) == 0 {
			return nil
		}
		for _, value := range field.Enum.Values {
			if value.Desc.Number() != 0 {
				return []any{string(value.Desc.Name())}
			}
		}
		return []any{string(field.Enum.Values[0].Desc.Name())}
	case protoreflect.MessageKind:
		fullName := field.Desc.Message().FullName()
		switch fullName {
		case "google.protobuf.Empty":
			return []any{map[string]any{}}
		case "google.protobuf.Timestamp":
			return []any{"2026-03-09T10:11:12Z"}
		case "google.protobuf.Duration":
			return []any{"3600s"}
		case "google.protobuf.FieldMask":
			return []any{"fieldName,otherField"}
		case "google.protobuf.Struct":
			return []any{map[string]any{"kind": "demo", "nested": map[string]any{"ok": true}}}
		case "google.protobuf.ListValue":
			return []any{[]any{"item", 2.0, true}}
		case "google.protobuf.Value":
			return []any{map[string]any{"kind": "demo"}}
		case "google.protobuf.Any":
			return []any{
				map[string]any{
					"@type": "type.googleapis.com/example.v1.Message",
					"value": "replace-me",
				},
				map[string]any{
					"@type": "type.googleapis.com/google.protobuf.Duration",
					"value": "1s",
				},
			}
		case "google.protobuf.BoolValue":
			return []any{true}
		case "google.protobuf.StringValue":
			return []any{"example"}
		case "google.protobuf.BytesValue":
			return []any{"aGVsbG8="}
		case "google.protobuf.Int32Value",
			"google.protobuf.UInt32Value":
			return []any{1}
		case "google.protobuf.Int64Value",
			"google.protobuf.UInt64Value":
			return []any{"1"}
		case "google.protobuf.FloatValue",
			"google.protobuf.DoubleValue":
			return []any{1.25, "NaN"}
		}

		example, ok := builder.autoMessageExample(field.Message, seen)
		if !ok {
			return nil
		}
		return []any{example}
	default:
		return nil
	}
}

func (builder *schemaBuilder) autoMapExample(
	field *protogen.Field,
	seen map[protoreflect.FullName]int,
) (map[string]any, bool) {
	if field == nil || field.Message == nil || len(field.Message.Fields) != 2 {
		return nil, false
	}

	key := autoMapKeyExample(field.Message.Fields[0])
	if key == "" {
		return nil, false
	}

	valueExamples := builder.autoSingularExamples(field.Message.Fields[1], seen)
	if len(valueExamples) == 0 {
		return nil, false
	}

	return map[string]any{
		key: valueExamples[0],
	}, true
}

func autoMapKeyExample(field *protogen.Field) string {
	if field == nil {
		return ""
	}

	switch field.Desc.Kind() {
	case protoreflect.StringKind:
		return "key"
	case protoreflect.BoolKind:
		return "true"
	case protoreflect.Int32Kind,
		protoreflect.Sint32Kind,
		protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind,
		protoreflect.Sint64Kind,
		protoreflect.Sfixed64Kind:
		return "1"
	case protoreflect.Uint32Kind,
		protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind,
		protoreflect.Fixed64Kind:
		return "1"
	default:
		return ""
	}
}

func (builder *schemaBuilder) autoMessageExample(
	message *protogen.Message,
	seen map[protoreflect.FullName]int,
) (map[string]any, bool) {
	if message == nil {
		return nil, false
	}

	fullName := message.Desc.FullName()
	seen[fullName]++
	defer func() {
		seen[fullName]--
		if seen[fullName] == 0 {
			delete(seen, fullName)
		}
	}()

	if seen[fullName] > 5 {
		return nil, false
	}

	recursive := seen[fullName] > 1
	example := make(map[string]any)
	handledOneofs := make(map[protoreflect.Name]struct{})

	for _, field := range message.Fields {
		if shouldSkipField(field) {
			continue
		}

		if isRealOneofField(field) {
			oneofName := field.Desc.ContainingOneof().Name()
			if _, exists := handledOneofs[oneofName]; exists {
				continue
			}
			handledOneofs[oneofName] = struct{}{}
		}

		if recursive && !isRequiredField(field, lookupFieldMetadata(builder.options, field)) {
			continue
		}

		value, ok := builder.autoFieldExampleValue(field, seen)
		if !ok {
			continue
		}

		example[field.Desc.JSONName()] = value
	}

	if len(example) == 0 {
		return nil, false
	}

	return example, true
}

func (builder *schemaBuilder) autoFieldExampleValue(
	field *protogen.Field,
	seen map[protoreflect.FullName]int,
) (any, bool) {
	if field == nil {
		return nil, false
	}

	metadata := lookupFieldMetadata(builder.options, field)
	if len(metadata.Examples) > 0 {
		value, ok := parseExampleLiteral(metadata.Examples[0])
		if ok {
			return value, true
		}
	}

	if field.Desc.IsMap() {
		return builder.autoMapExample(field, seen)
	}

	examples := builder.autoSingularExamples(field, seen)
	if len(examples) == 0 {
		return nil, false
	}
	if field.Desc.IsList() {
		return []any{examples[0]}, true
	}

	return examples[0], true
}

func parseExampleLiteral(raw string) (any, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}

	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, false
	}

	return value, true
}

func isUnconstrainedSchema(schema *jsonschema.Schema) bool {
	if schema == nil {
		return true
	}

	return schema.Ref == "" &&
		schema.Type == "" &&
		len(schema.Types) == 0 &&
		len(schema.Enum) == 0 &&
		schema.Items == nil &&
		len(schema.Properties) == 0 &&
		schema.AdditionalProperties == nil &&
		schema.PropertyNames == nil &&
		len(schema.AllOf) == 0 &&
		len(schema.AnyOf) == 0 &&
		len(schema.OneOf) == 0 &&
		schema.Not == nil &&
		schema.Format == "" &&
		schema.Pattern == "" &&
		schema.ContentEncoding == "" &&
		len(schema.Defs) == 0
}

func appendUniqueStrings(values []string, additions ...string) []string {
	seen := make(map[string]struct{}, len(values)+len(additions))
	result := make([]string, 0, len(values)+len(additions))

	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	for _, value := range additions {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	return result
}

func lookupMessageMetadata(options Options, message *protogen.Message) Metadata {
	if options.MessageMetadata == nil {
		return Metadata{}
	}

	return options.MessageMetadata(message)
}

func lookupFieldMetadata(options Options, field *protogen.Field) FieldMetadata {
	if options.FieldMetadata == nil {
		return FieldMetadata{}
	}

	return options.FieldMetadata(field)
}

func lookupEnumMetadata(options Options, enum *protogen.Enum) EnumMetadata {
	if options.EnumMetadata == nil {
		return EnumMetadata{}
	}

	return options.EnumMetadata(enum)
}

func lookupEnumValueMetadata(options Options, enumValue *protogen.EnumValue) EnumValueMetadata {
	if options.EnumValueMetadata == nil {
		return EnumValueMetadata{}
	}

	return options.EnumValueMetadata(enumValue)
}

func isRequiredField(field *protogen.Field, metadata FieldMetadata) bool {
	if isRealOneofField(field) {
		return false
	}
	if field.Desc.IsList() || field.Desc.IsMap() {
		return false
	}
	if field.Desc.HasOptionalKeyword() {
		return false
	}

	return true
}

func shouldSkipField(field *protogen.Field) bool {
	return field == nil
}

func isRealOneofField(field *protogen.Field) bool {
	if field == nil {
		return false
	}

	oneof := field.Desc.ContainingOneof()
	return oneof != nil && !oneof.IsSynthetic()
}

func disallowAdditionalProperties() *jsonschema.Schema {
	return &jsonschema.Schema{
		Not: &jsonschema.Schema{},
	}
}

func materializeExamples(examples []string) []any {
	if len(examples) == 0 {
		return nil
	}

	values := make([]any, 0, len(examples))
	for _, example := range examples {
		if parsed, ok := parseExampleLiteral(example); ok {
			values = append(values, parsed)
			continue
		}
		values = append(values, example)
	}

	return values
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}

	return ""
}

func mergeDescriptions(values ...string) string {
	result := ""
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if result == "" {
			result = value
			continue
		}
		if value == result || strings.Contains(result, value) {
			continue
		}
		result += "\n\n" + value
	}

	return result
}

// applyStringConstraints sets JSON Schema string validation keywords from FieldMetadata.
// Zero values mean "not set" and are not emitted.
func applyStringConstraints(s *jsonschema.Schema, m FieldMetadata) {
	if m.Pattern != "" {
		s.Pattern = m.Pattern
	}
	if m.Format != "" {
		s.Format = m.Format
	}
	if m.MinLength != 0 {
		v := int(m.MinLength)
		s.MinLength = &v
	}
	if m.MaxLength != 0 {
		v := int(m.MaxLength)
		s.MaxLength = &v
	}
}

// applyNumberConstraints sets JSON Schema numeric validation keywords from FieldMetadata.
// Zero values mean "not set" and are not emitted.
func applyNumberConstraints(s *jsonschema.Schema, m FieldMetadata) {
	if m.Minimum != 0 {
		v := m.Minimum
		s.Minimum = &v
	}
	if m.Maximum != 0 {
		v := m.Maximum
		s.Maximum = &v
	}
	if m.ExclusiveMinimum != 0 {
		v := m.ExclusiveMinimum
		s.ExclusiveMinimum = &v
	}
	if m.ExclusiveMaximum != 0 {
		v := m.ExclusiveMaximum
		s.ExclusiveMaximum = &v
	}
	if m.MultipleOf != 0 {
		v := m.MultipleOf
		s.MultipleOf = &v
	}
}

// applyArrayConstraints sets JSON Schema array validation keywords from FieldMetadata.
// Zero values mean "not set" and are not emitted.
func applyArrayConstraints(s *jsonschema.Schema, m FieldMetadata) {
	if m.MinItems != 0 {
		v := int(m.MinItems)
		s.MinItems = &v
	}
	if m.MaxItems != 0 {
		v := int(m.MaxItems)
		s.MaxItems = &v
	}
	if m.UniqueItems {
		s.UniqueItems = true
	}
}

// marshalDefault converts a Go value to json.RawMessage for the Schema.Default field.
// Returns nil if marshaling fails.
func marshalDefault(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return raw
}
