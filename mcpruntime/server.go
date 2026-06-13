package mcpruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

const protocolVersion = "2025-11-25"

// ToolHandler processes a tools/call request.
type ToolHandler func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error)

// ResourceHandler processes a resources/read request.
type ResourceHandler func(ctx context.Context, req *ReadResourceRequest) (*ReadResourceResult, error)

// PromptHandler processes a prompts/get request.
type PromptHandler func(ctx context.Context, req *GetPromptRequest) (*GetPromptResult, error)

type serverState int

const (
	stateInit serverState = iota
	stateReady
)

type registeredTool struct {
	tool    *Tool
	handler ToolHandler
}

type registeredResource struct {
	resource *Resource
	handler  ResourceHandler
}

type registeredResourceTemplate struct {
	template *ResourceTemplate
	handler  ResourceHandler
}

type registeredPrompt struct {
	prompt  *Prompt
	handler PromptHandler
}

// Server is an MCP server that manages tools, resources, and prompts.
type Server struct {
	impl       Implementation
	dispatcher *dispatcher
	state      serverState

	mu                sync.RWMutex
	tools             map[string]*registeredTool
	toolOrder         []string
	resources         []registeredResource
	resourceTemplates []registeredResourceTemplate
	prompts           map[string]*registeredPrompt
	promptOrder       []string
}

// NewServer creates a new MCP server with the given name and version.
func NewServer(name, version string) *Server {
	s := &Server{
		impl: Implementation{
			Name:    name,
			Version: version,
		},
		dispatcher: newDispatcher(),
		state:      stateInit,
		tools:      make(map[string]*registeredTool),
		prompts:    make(map[string]*registeredPrompt),
	}

	s.dispatcher.register("initialize", s.handleInitialize)
	s.dispatcher.register("notifications/initialized", s.handleInitialized)
	s.dispatcher.register("ping", s.handlePing)
	s.dispatcher.register("tools/list", s.requireReady(s.handleToolsList))
	s.dispatcher.register("tools/call", s.requireReady(s.handleToolsCall))
	s.dispatcher.register("resources/list", s.requireReady(s.handleResourcesList))
	s.dispatcher.register("resources/templates/list", s.requireReady(s.handleResourceTemplatesList))
	s.dispatcher.register("resources/read", s.requireReady(s.handleResourcesRead))
	s.dispatcher.register("prompts/list", s.requireReady(s.handlePromptsList))
	s.dispatcher.register("prompts/get", s.requireReady(s.handlePromptsGet))

	return s
}

// AddTool registers a tool with its handler on the server.
func (s *Server) AddTool(tool *Tool, handler ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tools[tool.Name]; !exists {
		s.toolOrder = append(s.toolOrder, tool.Name)
	}
	s.tools[tool.Name] = &registeredTool{tool: tool, handler: handler}
}

// AddResource registers a static resource with its handler on the server.
func (s *Server) AddResource(resource *Resource, handler ResourceHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.resources = append(s.resources, registeredResource{resource: resource, handler: handler})
}

// AddResourceTemplate registers a resource template with its handler on the server.
func (s *Server) AddResourceTemplate(template *ResourceTemplate, handler ResourceHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.resourceTemplates = append(s.resourceTemplates, registeredResourceTemplate{template: template, handler: handler})
}

// AddPrompt registers a prompt with its handler on the server.
func (s *Server) AddPrompt(prompt *Prompt, handler PromptHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.prompts[prompt.Name]; !exists {
		s.promptOrder = append(s.promptOrder, prompt.Name)
	}
	s.prompts[prompt.Name] = &registeredPrompt{prompt: prompt, handler: handler}
}

// HandleRaw processes a raw JSON-RPC message and returns the serialized response.
func (s *Server) HandleRaw(ctx context.Context, raw []byte) []byte {
	return s.dispatcher.dispatch(ctx, raw)
}

// requireReady wraps a handler to reject requests before the server is initialized.
func (s *Server) requireReady(handler methodHandler) methodHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		if s.state != stateReady {
			return nil, &JSONRPCError{
				Code:    CodeInvalidRequest,
				Message: "Server not initialized",
			}
		}
		return handler(ctx, params)
	}
}

// capabilities returns the server's dynamic capabilities based on registered primitives.
func (s *Server) capabilities() *ServerCapabilities {
	s.mu.RLock()
	defer s.mu.RUnlock()

	caps := &ServerCapabilities{}
	if len(s.tools) > 0 {
		caps.Tools = &ToolsCapability{}
	}
	if len(s.resources) > 0 || len(s.resourceTemplates) > 0 {
		caps.Resources = &ResourcesCapability{}
	}
	if len(s.prompts) > 0 {
		caps.Prompts = &PromptsCapability{}
	}
	return caps
}

func (s *Server) handleInitialize(_ context.Context, params json.RawMessage) (any, error) {
	// Parse but don't require specific fields from client.
	s.state = stateReady

	return &InitializeResult{
		ProtocolVersion: protocolVersion,
		Capabilities:    s.capabilities(),
		ServerInfo: &Implementation{
			Name:    s.impl.Name,
			Version: s.impl.Version,
		},
	}, nil
}

func (s *Server) handleInitialized(_ context.Context, _ json.RawMessage) (any, error) {
	// No-op notification.
	return nil, nil
}

func (s *Server) handlePing(_ context.Context, _ json.RawMessage) (any, error) {
	return struct{}{}, nil
}

func (s *Server) handleToolsList(_ context.Context, _ json.RawMessage) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]*Tool, 0, len(s.tools))
	for _, name := range s.toolOrder {
		if rt, ok := s.tools[name]; ok {
			tools = append(tools, rt.tool)
		}
	}

	return struct {
		Tools []*Tool `json:"tools"`
	}{Tools: tools}, nil
}

func (s *Server) handleToolsCall(ctx context.Context, params json.RawMessage) (any, error) {
	var req CallToolRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &JSONRPCError{
			Code:    CodeInvalidParams,
			Message: fmt.Sprintf("invalid tools/call params: %v", err),
		}
	}

	s.mu.RLock()
	rt, ok := s.tools[req.Name]
	s.mu.RUnlock()

	if !ok {
		return nil, &JSONRPCError{
			Code:    CodeInvalidParams,
			Message: fmt.Sprintf("unknown tool: %q", req.Name),
		}
	}

	result, err := rt.handler(ctx, &req)
	if err != nil {
		// Check if the handler returned a JSONRPCError (e.g. validation failure).
		if rpcErr, ok := err.(*JSONRPCError); ok {
			return nil, rpcErr
		}
		// Application-level errors → isError: true, not JSON-RPC error.
		return &CallToolResult{
			IsError: true,
			Content: []Content{&TextContent{Type: "text", Text: err.Error()}},
		}, nil
	}

	return result, nil
}

func (s *Server) handleResourcesList(_ context.Context, _ json.RawMessage) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resources := make([]*Resource, 0, len(s.resources))
	for _, r := range s.resources {
		resources = append(resources, r.resource)
	}

	return struct {
		Resources []*Resource `json:"resources"`
	}{Resources: resources}, nil
}

func (s *Server) handleResourceTemplatesList(_ context.Context, _ json.RawMessage) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	templates := make([]*ResourceTemplate, 0, len(s.resourceTemplates))
	for _, rt := range s.resourceTemplates {
		templates = append(templates, rt.template)
	}

	return struct {
		ResourceTemplates []*ResourceTemplate `json:"resourceTemplates"`
	}{ResourceTemplates: templates}, nil
}

func (s *Server) handleResourcesRead(ctx context.Context, params json.RawMessage) (any, error) {
	var req ReadResourceRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &JSONRPCError{
			Code:    CodeInvalidParams,
			Message: fmt.Sprintf("invalid resources/read params: %v", err),
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Try static resources first.
	for _, r := range s.resources {
		if r.resource.URI == req.URI {
			return r.handler(ctx, &req)
		}
	}

	// Try resource templates.
	for _, rt := range s.resourceTemplates {
		if _, err := ExtractURIParams(req.URI, rt.template.URITemplate); err == nil {
			return rt.handler(ctx, &req)
		}
	}

	return nil, &JSONRPCError{
		Code:    CodeInvalidParams,
		Message: fmt.Sprintf("unknown resource: %q", req.URI),
	}
}

func (s *Server) handlePromptsList(_ context.Context, _ json.RawMessage) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prompts := make([]*Prompt, 0, len(s.prompts))
	for _, name := range s.promptOrder {
		if rp, ok := s.prompts[name]; ok {
			prompts = append(prompts, rp.prompt)
		}
	}

	return struct {
		Prompts []*Prompt `json:"prompts"`
	}{Prompts: prompts}, nil
}

func (s *Server) handlePromptsGet(ctx context.Context, params json.RawMessage) (any, error) {
	var req GetPromptRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &JSONRPCError{
			Code:    CodeInvalidParams,
			Message: fmt.Sprintf("invalid prompts/get params: %v", err),
		}
	}

	s.mu.RLock()
	rp, ok := s.prompts[req.Name]
	s.mu.RUnlock()

	if !ok {
		return nil, &JSONRPCError{
			Code:    CodeInvalidParams,
			Message: fmt.Sprintf("unknown prompt: %q", req.Name),
		}
	}

	return rp.handler(ctx, &req)
}
