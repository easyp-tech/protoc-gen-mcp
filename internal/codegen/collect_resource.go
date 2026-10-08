package codegen

import (
	"fmt"
	"io/fs"
	"mime"
	"path"
	"regexp"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// paramRegex matches valid URI template parameter placeholders like {user_id}.
var paramRegex = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

// collectResources scans all messages in the file for (mcp.options.v1.resource).
// Returns fail-fast error on: both uri+uri_template, neither set,
// template without params, invalid param identifiers.
func collectResources(file *protogen.File) ([]ResourceModel, error) {
	var resources []ResourceModel

	for _, message := range file.Messages {
		opts, err := getResourceOptions(message)
		if err != nil {
			return nil, err
		}
		if opts == nil {
			continue
		}

		uri := strings.TrimSpace(opts.GetUri())
		uriTemplate := strings.TrimSpace(opts.GetUriTemplate())

		// Validate: uri XOR uri_template.
		if uri != "" && uriTemplate != "" {
			return nil, fmt.Errorf("resource %s: uri and uri_template are mutually exclusive", message.Desc.FullName())
		}
		if uri == "" && uriTemplate == "" {
			return nil, fmt.Errorf("resource %s: either uri or uri_template must be set", message.Desc.FullName())
		}

		isTemplate := uriTemplate != ""
		var params []ResourceParamModel

		if isTemplate {
			// Extract and validate template parameters.
			params, err = extractTemplateParams(string(message.Desc.FullName()), uriTemplate)
			if err != nil {
				return nil, err
			}
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

		mimeType := strings.TrimSpace(opts.GetMimeType())
		if mimeType == "" {
			mimeType = "application/json"
		}
		baseMIME, _, err := mime.ParseMediaType(mimeType)
		if err != nil {
			return nil, fmt.Errorf("resource %s: invalid mime_type %q: %w", message.Desc.FullName(), mimeType, err)
		}

		sourceFile := strings.TrimSpace(opts.GetSourceFile())
		if sourceFile != "" {
			if uri == "" || uriTemplate != "" {
				return nil, fmt.Errorf("resource %s: source_file requires a static uri", message.Desc.FullName())
			}
			if opts.GetContentField() != "" {
				return nil, fmt.Errorf("resource %s: source_file and content_field are mutually exclusive", message.Desc.FullName())
			}
			if err := validateEmbedFile(sourceFile); err != nil {
				return nil, fmt.Errorf("resource %s: %w", message.Desc.FullName(), err)
			}
		}
		contentField := strings.TrimSpace(opts.GetContentField())
		if contentField == "" && sourceFile == "" {
			if baseMIME != "application/json" && !strings.HasSuffix(baseMIME, "+json") {
				return nil, fmt.Errorf("resource %s: non-JSON mime_type %q requires content_field to avoid serving ProtoJSON as another format", message.Desc.FullName(), mimeType)
			}
		} else if contentField != "" {
			field := message.Desc.Fields().ByName(protoreflect.Name(contentField))
			if field == nil {
				return nil, fmt.Errorf("resource %s: content_field %q is not a declared protobuf field", message.Desc.FullName(), contentField)
			}
			if field.IsList() || field.IsMap() {
				return nil, fmt.Errorf("resource %s: content_field %q must be singular string or bytes", message.Desc.FullName(), contentField)
			}
			if field.Kind() != protoreflect.StringKind && field.Kind() != protoreflect.BytesKind {
				return nil, fmt.Errorf("resource %s: content_field %q must be string or bytes, got %s", message.Desc.FullName(), contentField, field.Kind())
			}
			if field.Kind() == protoreflect.BytesKind && strings.HasPrefix(baseMIME, "text/") {
				return nil, fmt.Errorf("resource %s: text mime_type %q requires a string content_field, not bytes", message.Desc.FullName(), mimeType)
			}
		}

		resource := ResourceModel{
			ProtoFullName: string(message.Desc.FullName()),
			ProtoName:     string(message.Desc.Name()),
			Name:          name,
			Description:   description,
			URI:           uri,
			URITemplate:   uriTemplate,
			MIMEType:      mimeType,
			ContentField:  contentField,
			SourceFile:    sourceFile,
			AppUI:         opts.GetAppUi(),
			IsTemplate:    isTemplate,
			Params:        params,
			Annotations:   opts.GetAnnotations(),
			Icons:         opts.GetIcons(),
			Output:        newTypeRef(message),
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

// extractTemplateParams extracts parameter names from a URI template and validates them.
func extractTemplateParams(msgFullName, uriTemplate string) ([]ResourceParamModel, error) {
	// Find all {param} placeholders.
	matches := paramRegex.FindAllStringSubmatch(uriTemplate, -1)
	if len(matches) == 0 {
		// Check if there are any {…} at all (with invalid identifiers).
		if strings.Contains(uriTemplate, "{") {
			return nil, fmt.Errorf("resource %s: invalid parameter in uri_template %q — identifiers must match [a-zA-Z_][a-zA-Z0-9_]*", msgFullName, uriTemplate)
		}
		return nil, fmt.Errorf("resource %s: uri_template %q has no parameters, use uri instead", msgFullName, uriTemplate)
	}

	// Verify there are no malformed placeholders (e.g., {123bad}).
	// Count all opening braces and compare with valid matches.
	braceCount := strings.Count(uriTemplate, "{")
	if braceCount != len(matches) {
		return nil, fmt.Errorf("resource %s: invalid parameter in uri_template %q — identifiers must match [a-zA-Z_][a-zA-Z0-9_]*", msgFullName, uriTemplate)
	}

	params := make([]ResourceParamModel, 0, len(matches))
	for _, match := range matches {
		params = append(params, ResourceParamModel{Name: match[1]})
	}

	return params, nil
}

func validateEmbedFile(name string) error {
	if !fs.ValidPath(name) || path.Clean(name) != name || strings.ContainsAny(name, "*?[\\]") {
		return fmt.Errorf("source_file %q is not a valid relative Go embed path", name)
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == "" || strings.HasPrefix(segment, ".") || strings.HasPrefix(segment, "_") {
			return fmt.Errorf("source_file %q uses an unsupported Go embed path segment", name)
		}
	}
	return nil
}
