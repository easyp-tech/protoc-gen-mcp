package mcpruntime

import (
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var templateParamRe = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

// ExtractURIParams extracts named parameters from a URI using the given
// URI template. Template parameters use the {name} syntax.
func ExtractURIParams(uri, uriTemplate string) (map[string]string, error) {
	// Collect parameter names in order.
	matches := templateParamRe.FindAllStringSubmatch(uriTemplate, -1)
	paramNames := make([]string, 0, len(matches))
	for _, m := range matches {
		paramNames = append(paramNames, m[1])
	}

	// Build a matching regex by splitting the template at each {param},
	// quoting the static parts, and joining with capture groups.
	parts := templateParamRe.Split(uriTemplate, -1)
	var pattern strings.Builder
	pattern.WriteString("^")
	for i, part := range parts {
		pattern.WriteString(regexp.QuoteMeta(part))
		if i < len(paramNames) {
			pattern.WriteString("([^/]+)")
		}
	}
	pattern.WriteString("$")

	re, err := regexp.Compile(pattern.String())
	if err != nil {
		return nil, fmt.Errorf("mcpruntime: uri %q does not match template %q", uri, uriTemplate)
	}

	groups := re.FindStringSubmatch(uri)
	if groups == nil {
		return nil, fmt.Errorf("mcpruntime: uri %q does not match template %q", uri, uriTemplate)
	}

	result := make(map[string]string, len(paramNames))
	for i, name := range paramNames {
		value := groups[i+1]
		if value == "" {
			return nil, fmt.Errorf("mcpruntime: parameter %q has empty value in uri %q", name, uri)
		}
		result[name] = value
	}

	return result, nil
}

// MarshalResourceContent serializes a proto message as MCP ResourceContents
// using ProtoJSON encoding.
func MarshalResourceContent(uri, mimeType string, msg proto.Message) ([]*ResourceContents, error) {
	jsonBytes, err := protojson.MarshalOptions{
		EmitDefaultValues: true,
	}.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("mcpruntime: marshal resource content: %w", err)
	}

	contents := &ResourceContents{
		URI:      uri,
		MIMEType: mimeType,
		Text:     string(jsonBytes),
	}

	return []*ResourceContents{contents}, nil
}

