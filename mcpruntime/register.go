package mcpruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type serverRegistry struct {
	mu    sync.Mutex
	tools map[string]struct{}
}

var serverRegistries sync.Map

// ToolSpec describes a generated protobuf-backed MCP tool.
type ToolSpec[Req proto.Message, Resp proto.Message] struct {
	Name             string
	Title            string
	Description      string
	Namespace        string
	InputSchemaJSON  string
	OutputSchemaJSON string
	Annotations      *ToolAnnotations
	Icons            []Icon
	NewRequest       func() Req
	NewResponse      func() Resp
	Handler          func(context.Context, Req) (Resp, error)
}

// RegisterProtoTool registers a protobuf-backed tool on the target MCP server.
func RegisterProtoTool[Req proto.Message, Resp proto.Message](
	server *Server,
	spec ToolSpec[Req, Resp],
	options ...RegisterOption,
) error {
	if server == nil {
		return errors.New("mcpruntime: server is nil")
	}
	if spec.Name == "" {
		return errors.New("mcpruntime: tool name is empty")
	}
	if normalizedName := normalizeToolSegment(spec.Name); normalizedName == "" {
		return fmt.Errorf("mcpruntime: tool name %q resolves to empty", spec.Name)
	}
	if spec.NewRequest == nil {
		return fmt.Errorf("mcpruntime: tool %q is missing request constructor", spec.Name)
	}
	if spec.NewResponse == nil {
		return fmt.Errorf("mcpruntime: tool %q is missing response constructor", spec.Name)
	}
	if spec.Handler == nil {
		return fmt.Errorf("mcpruntime: tool %q is missing handler", spec.Name)
	}

	inputSchema, inputResolved, err := loadSchema(spec.InputSchemaJSON)
	if err != nil {
		return fmt.Errorf("mcpruntime: tool %q input schema: %w", spec.Name, err)
	}

	outputSchema, outputResolved, err := loadSchema(spec.OutputSchemaJSON)
	if err != nil {
		return fmt.Errorf("mcpruntime: tool %q output schema: %w", spec.Name, err)
	}

	resolvedOptions := resolveOptions(spec.Namespace, options)
	fullName := qualifyToolName(resolvedOptions.Namespace, spec.Name)
	if err := reserveToolName(server, fullName); err != nil {
		return err
	}

	server.AddTool(&Tool{
		Name:         fullName,
		Title:        spec.Title,
		Description:  spec.Description,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Annotations:  spec.Annotations,
		Icons:        spec.Icons,
	}, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		rawArguments, err := marshalArguments(req)
		if err != nil {
			return nil, invalidParamsError(fullName, err)
		}

		if err := validateJSON(rawArguments, inputResolved); err != nil {
			return nil, invalidParamsError(fullName, err)
		}

		request := spec.NewRequest()
		if err := (protojson.UnmarshalOptions{
			DiscardUnknown: false,
		}).Unmarshal(rawArguments, request); err != nil {
			return nil, invalidParamsError(fullName, err)
		}

		response, err := spec.Handler(ctx, request)
		if err != nil {
			return &CallToolResult{
				IsError: true,
				Content: []Content{&TextContent{Type: "text", Text: err.Error()}},
			}, nil
		}

		var zeroResponse Resp
		if any(response) == any(zeroResponse) {
			response = spec.NewResponse()
		}

		structuredContent, err := protojson.MarshalOptions{
			EmitDefaultValues: true,
		}.Marshal(response)
		if err != nil {
			return nil, &JSONRPCError{Code: CodeInternalError, Message: fmt.Sprintf("mcpruntime: marshal output for tool %q: %v", fullName, err)}
		}

		if err := validateJSON(structuredContent, outputResolved); err != nil {
			return nil, &JSONRPCError{Code: CodeInternalError, Message: fmt.Sprintf("mcpruntime: validate output for tool %q: %v", fullName, err)}
		}

		return &CallToolResult{
			Content: []Content{
				&TextContent{Type: "text", Text: string(structuredContent)},
			},
			StructuredContent: json.RawMessage(structuredContent),
		}, nil
	})

	return nil
}

func reserveToolName(server *Server, toolName string) error {
	registryAny, _ := serverRegistries.LoadOrStore(server, &serverRegistry{
		tools: make(map[string]struct{}),
	})
	registry := registryAny.(*serverRegistry)

	registry.mu.Lock()
	defer registry.mu.Unlock()

	if _, exists := registry.tools[toolName]; exists {
		return fmt.Errorf("mcpruntime: tool %q is already registered on this server", toolName)
	}

	registry.tools[toolName] = struct{}{}

	return nil
}

func marshalArguments(req *CallToolRequest) ([]byte, error) {
	if req == nil || req.Arguments == nil {
		return []byte("{}"), nil
	}

	if len(req.Arguments) == 0 {
		return []byte("{}"), nil
	}

	return req.Arguments, nil
}

func invalidParamsError(toolName string, err error) error {
	return &JSONRPCError{
		Code:    CodeInvalidParams,
		Message: fmt.Sprintf("invalid arguments for tool %q: %v", toolName, err),
	}
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
