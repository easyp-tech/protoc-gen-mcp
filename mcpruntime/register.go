package mcpruntime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
	"google.golang.org/protobuf/proto"
)

// ToolAnnotations preserves the protobuf-derived tri-state MCP hints.
// They are converted to official SDK types during registration.
type ToolAnnotations struct {
	ReadOnlyHint    *bool
	DestructiveHint *bool
	IdempotentHint  *bool
	OpenWorldHint   *bool
}

// Icon is an icon generated from the protobuf MCP options.
type Icon struct {
	URL      string
	MIMEType string
}

// ToolSpec describes a generated typed protobuf-backed MCP tool.
type ToolSpec[Req proto.Message, Resp proto.Message] struct {
	Name             string
	Title            string
	Description      string
	Namespace        string
	InputSchemaJSON  string
	OutputSchemaJSON string
	Annotations      *ToolAnnotations
	Icons            []Icon
	AppUI            *AppUI
	NewRequest       func() Req
	NewResponse      func() Resp
	Handler          func(context.Context, Req) (Resp, error)
}

// serverRegistry rejects duplicate names before they can replace an SDK tool.
type serverRegistry struct {
	mu    sync.Mutex
	tools map[string]struct{}
}

func loadSchema(schemaJSON string) (*jsonschema.Schema, *jsonschema.Resolved, error) {
	if strings.TrimSpace(schemaJSON) == "" {
		return nil, nil, errors.New("schema JSON is empty")
	}

	var schema jsonschema.Schema
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		return nil, nil, err
	}

	resolved, err := schema.Resolve(&jsonschema.ResolveOptions{
		ValidateDefaults: true,
	})
	if err != nil {
		return nil, nil, err
	}

	return &schema, resolved, nil
}

func validateJSON(rawJSON []byte, resolved *jsonschema.Resolved) error {
	if resolved == nil {
		return errors.New("schema is nil")
	}

	var instance any
	if len(rawJSON) == 0 {
		instance = map[string]any{}
	} else if err := json.Unmarshal(rawJSON, &instance); err != nil {
		return err
	}

	return resolved.Validate(instance)
}
