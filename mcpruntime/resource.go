package mcpruntime

import (
	"errors"
	"fmt"
	"mime"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var templateParamRe = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

// ExtractURIParams extracts named parameters from a URI using the given
// URI template. Template parameters use the {name} syntax.
func ExtractURIParams(uri, uriTemplate string) (map[string]string, error) {
	matches := templateParamRe.FindAllStringSubmatch(uriTemplate, -1)
	paramNames := make([]string, 0, len(matches))
	for _, m := range matches {
		paramNames = append(paramNames, m[1])
	}
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
		return nil, fmt.Errorf("mcpruntime: uri %q does not match template %q: %w", uri, uriTemplate, err)
	}
	groups := re.FindStringSubmatch(uri)
	if groups == nil {
		return nil, fmt.Errorf("mcpruntime: uri %q does not match template %q", uri, uriTemplate)
	}
	result := make(map[string]string, len(paramNames))
	for i, name := range paramNames {
		if groups[i+1] == "" {
			return nil, fmt.Errorf("mcpruntime: parameter %q has empty value in uri %q", name, uri)
		}
		result[name] = groups[i+1]
	}
	return result, nil
}

// MarshalResourceContent serializes a protobuf-backed MCP resource. Without
// contentField, the protobuf message is encoded as ProtoJSON (requiring a
// JSON-compatible MIME type). With a contentField, a singular string field
// becomes raw text and a singular bytes field becomes a base64-encoded MCP blob.
// Field names are protobuf names, not ProtoJSON names.
func MarshalResourceContent(uri, mimeType string, msg proto.Message, contentField ...string) ([]*mcp.ResourceContents, error) {
	if msg == nil {
		return nil, errors.New("mcpruntime: resource message is nil")
	}
	reflection := msg.ProtoReflect()
	if !reflection.IsValid() {
		return nil, errors.New("mcpruntime: resource message is invalid")
	}
	baseMIME, _, err := mime.ParseMediaType(mimeType)
	if err != nil {
		return nil, fmt.Errorf("mcpruntime: invalid MIME type %q: %w", mimeType, err)
	}
	if len(contentField) > 1 {
		return nil, errors.New("mcpruntime: at most one content field is supported")
	}
	contents := &mcp.ResourceContents{URI: uri, MIMEType: mimeType}
	if len(contentField) == 0 {
		if baseMIME != "application/json" && !strings.HasSuffix(baseMIME, "+json") {
			return nil, fmt.Errorf("mcpruntime: non-JSON MIME type %q requires a raw content field", mimeType)
		}
		encoded, err := (protojson.MarshalOptions{EmitDefaultValues: true}).Marshal(msg)
		if err != nil {
			return nil, fmt.Errorf("mcpruntime: marshal resource content: %w", err)
		}
		contents.Text = string(encoded)
		return []*mcp.ResourceContents{contents}, nil
	}
	fieldName := contentField[0]
	if fieldName == "" {
		return nil, errors.New("mcpruntime: content field cannot be empty")
	}
	field := reflection.Descriptor().Fields().ByName(protoreflect.Name(fieldName))
	if field == nil {
		return nil, fmt.Errorf("mcpruntime: resource content field %q not found", fieldName)
	}
	if field.IsList() || field.IsMap() {
		return nil, fmt.Errorf("mcpruntime: resource content field %q must be singular", fieldName)
	}
	switch field.Kind() {
	case protoreflect.StringKind:
		contents.Text = reflection.Get(field).String()
	case protoreflect.BytesKind:
		if strings.HasPrefix(baseMIME, "text/") {
			return nil, fmt.Errorf("mcpruntime: text MIME type %q requires a string content field", mimeType)
		}
		contents.Blob = append([]byte{}, reflection.Get(field).Bytes()...)
	case protoreflect.MessageKind:
		return nil, fmt.Errorf("mcpruntime: resource content field %q must be string or bytes, got message", fieldName)
	default:
		return nil, fmt.Errorf("mcpruntime: resource content field %q must be string or bytes, got %s", fieldName, field.Kind())
	}
	return []*mcp.ResourceContents{contents}, nil
}
