package codegen

import (
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
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

		resource := ResourceModel{
			ProtoFullName: string(message.Desc.FullName()),
			ProtoName:     string(message.Desc.Name()),
			Name:          name,
			Description:   description,
			URI:           uri,
			URITemplate:   uriTemplate,
			MIMEType:      mimeType,
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
