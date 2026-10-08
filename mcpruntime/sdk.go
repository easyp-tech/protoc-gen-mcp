package mcpruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// sdkServerRegistries keeps duplicate-name validation scoped to the SDK server.
var sdkServerRegistries sync.Map

// RegisterSDKProtoTool registers a generated protobuf-backed tool with the
// official Go MCP SDK. JSON Schema and ProtoJSON remain generator-owned.
func RegisterSDKProtoTool[Req proto.Message, Resp proto.Message](
	server *mcp.Server,
	spec ToolSpec[Req, Resp],
	options ...RegisterOption,
) error {
	if server == nil {
		return errors.New("mcpruntime: server is nil")
	}
	if strings.TrimSpace(spec.Name) == "" || normalizeToolSegment(spec.Name) == "" {
		return fmt.Errorf("mcpruntime: invalid tool name %q", spec.Name)
	}
	if spec.NewRequest == nil || spec.NewResponse == nil || spec.Handler == nil {
		return fmt.Errorf("mcpruntime: tool %q requires request, response and handler constructors", spec.Name)
	}
	inputSchema, inputResolved, err := loadSchema(spec.InputSchemaJSON)
	if err != nil {
		return fmt.Errorf("mcpruntime: tool %q input schema: %w", spec.Name, err)
	}
	outputSchema, outputResolved, err := loadSchema(spec.OutputSchemaJSON)
	if err != nil {
		return fmt.Errorf("mcpruntime: tool %q output schema: %w", spec.Name, err)
	}

	opts := resolveOptions(spec.Namespace, options)
	fullName := qualifyToolName(opts.Namespace, spec.Name)
	if err := reserveSDKToolName(server, fullName); err != nil {
		return err
	}

	icons := make([]mcp.Icon, 0, len(spec.Icons))
	for _, icon := range spec.Icons {
		icons = append(icons, mcp.Icon{Source: icon.URL, MIMEType: icon.MIMEType})
	}
	tool := &mcp.Tool{
		Name:         fullName,
		Title:        spec.Title,
		Description:  spec.Description,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Annotations:  sdkToolAnnotations(spec.Annotations),
		Icons:        icons,
	}

	// Metadata is advertised by the tool; rendering is optional on the client.
	if ui, ok := opts.appUIFor(spec.Name, fullName); ok {
		if err := validateAppUI(ui); err != nil {
			return fmt.Errorf("mcpruntime: tool %q UI: %w", fullName, err)
		}
		tool.Meta = mcp.Meta{"ui": ui.metadata()}
	}

	server.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rawArgs := json.RawMessage("{}")
		if req != nil && req.Params != nil && len(req.Params.Arguments) > 0 {
			rawArgs = req.Params.Arguments
		}
		if err := validateJSON(rawArgs, inputResolved); err != nil {
			return sdkToolError(fmt.Errorf("invalid arguments for tool %q: %w", fullName, err)), nil
		}
		request := spec.NewRequest()
		if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(rawArgs, request); err != nil {
			return sdkToolError(fmt.Errorf("invalid arguments for tool %q: %w", fullName, err)), nil
		}

		response, err := spec.Handler(ctx, request)
		if err != nil {
			return sdkToolError(err), nil
		}
		var empty Resp
		if any(response) == any(empty) {
			response = spec.NewResponse()
		}

		raw, err := (protojson.MarshalOptions{EmitDefaultValues: true}).Marshal(response)
		if err != nil {
			return nil, fmt.Errorf("mcpruntime: marshal output for tool %q: %w", fullName, err)
		}
		if err := validateJSON(raw, outputResolved); err != nil {
			return nil, fmt.Errorf("mcpruntime: validate output for tool %q: %w", fullName, err)
		}
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: string(raw)}},
			StructuredContent: json.RawMessage(raw),
		}, nil
	})
	return nil
}

func sdkToolError(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
	}
}

func sdkToolAnnotations(a *ToolAnnotations) *mcp.ToolAnnotations {
	if a == nil {
		return nil
	}
	result := &mcp.ToolAnnotations{
		DestructiveHint: a.DestructiveHint,
		OpenWorldHint:   a.OpenWorldHint,
	}
	if a.ReadOnlyHint != nil {
		result.ReadOnlyHint = *a.ReadOnlyHint
	}
	if a.IdempotentHint != nil {
		result.IdempotentHint = *a.IdempotentHint
	}
	return result
}

func reserveSDKToolName(server *mcp.Server, name string) error {
	registryAny, _ := sdkServerRegistries.LoadOrStore(server, &serverRegistry{
		tools: make(map[string]struct{}),
	})
	registry := registryAny.(*serverRegistry)
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.tools[name]; exists {
		return fmt.Errorf("mcpruntime: tool %q is already registered on this server", name)
	}
	registry.tools[name] = struct{}{}
	return nil
}

// MarshalSDKResourceContent returns official-SDK resource contents encoded
// using the same ProtoJSON rules as the generator's tool outputs.
func MarshalSDKResourceContent(uri, mimeType string, msg proto.Message) ([]*mcp.ResourceContents, error) {
	if msg == nil {
		return nil, errors.New("mcpruntime: nil resource message")
	}
	data, err := (protojson.MarshalOptions{EmitDefaultValues: true}).Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("mcpruntime: marshal resource content: %w", err)
	}
	return []*mcp.ResourceContents{{URI: uri, MIMEType: mimeType, Text: string(data)}}, nil
}
