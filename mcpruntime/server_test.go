package mcpruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

// testServerHelper creates a server, initializes it, and returns a helper for sending JSON-RPC requests.
type testServerHelper struct {
	t      *testing.T
	server *Server
	nextID int
}

func newTestServer(t *testing.T) *testServerHelper {
	t.Helper()
	return &testServerHelper{
		t:      t,
		server: NewServer("test-server", "v0.0.1"),
		nextID: 1,
	}
}

func (h *testServerHelper) initialize() map[string]any {
	h.t.Helper()
	return h.call("initialize", map[string]any{
		"protocolVersion": "2025-11-25",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test-client", "version": "v0.0.1"},
	})
}

func (h *testServerHelper) call(method string, params any) map[string]any {
	h.t.Helper()

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		h.t.Fatalf("marshal params: %v", err)
	}

	reqID := h.nextID
	h.nextID++

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      marshalID(reqID),
		Method:  method,
		Params:  paramsBytes,
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		h.t.Fatalf("marshal request: %v", err)
	}

	resp := h.server.HandleRaw(context.Background(), reqBytes)
	if resp == nil {
		return nil
	}

	var result map[string]any
	if err := json.Unmarshal(resp, &result); err != nil {
		h.t.Fatalf("unmarshal response: %v", err)
	}
	return result
}

func (h *testServerHelper) notify(method string, params any) {
	h.t.Helper()

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		h.t.Fatalf("marshal params: %v", err)
	}

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsBytes,
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		h.t.Fatalf("marshal notification: %v", err)
	}

	h.server.HandleRaw(context.Background(), reqBytes)
}

func marshalID(id int) json.RawMessage {
	b, _ := json.Marshal(id)
	return b
}

func assertNoError(t *testing.T, resp map[string]any) {
	t.Helper()
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
}

func assertErrorCode(t *testing.T, resp map[string]any, wantCode int) {
	t.Helper()
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object, got %v", resp["error"])
	}
	code, ok := errObj["code"].(float64)
	if !ok {
		t.Fatalf("expected error code, got %v", errObj["code"])
	}
	if int(code) != wantCode {
		t.Fatalf("error code = %d, want %d", int(code), wantCode)
	}
}

func TestServer_Initialize(t *testing.T) {
	h := newTestServer(t)
	resp := h.initialize()
	assertNoError(t, resp)

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map", resp["result"])
	}
	if result["protocolVersion"] != "2025-11-25" {
		t.Fatalf("protocolVersion = %v, want 2025-11-25", result["protocolVersion"])
	}
	serverInfo, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatalf("serverInfo type = %T, want map", result["serverInfo"])
	}
	if serverInfo["name"] != "test-server" {
		t.Fatalf("serverInfo.name = %v, want test-server", serverInfo["name"])
	}
}

func TestServer_Ping(t *testing.T) {
	h := newTestServer(t)
	h.initialize()

	resp := h.call("ping", nil)
	assertNoError(t, resp)
}

func TestServer_ToolsList(t *testing.T) {
	h := newTestServer(t)
	h.server.AddTool(&Tool{Name: "echo", Description: "echoes input"}, func(_ context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return &CallToolResult{Content: []Content{&TextContent{Type: "text", Text: "ok"}}}, nil
	})
	h.server.AddTool(&Tool{Name: "greet", Description: "greets"}, func(_ context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return &CallToolResult{Content: []Content{&TextContent{Type: "text", Text: "hi"}}}, nil
	})

	h.initialize()
	resp := h.call("tools/list", map[string]any{})
	assertNoError(t, resp)

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map", resp["result"])
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("tools type = %T, want []any", result["tools"])
	}
	if len(tools) != 2 {
		t.Fatalf("tools count = %d, want 2", len(tools))
	}
}

func TestServer_ToolsCall_HappyPath(t *testing.T) {
	h := newTestServer(t)
	h.server.AddTool(&Tool{Name: "echo"}, func(_ context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return &CallToolResult{
			Content: []Content{&TextContent{Type: "text", Text: "hello"}},
		}, nil
	})

	h.initialize()
	resp := h.call("tools/call", map[string]any{"name": "echo", "arguments": map[string]any{}})
	assertNoError(t, resp)

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map", resp["result"])
	}
	content, ok := result["content"].([]any)
	if !ok {
		t.Fatalf("content type = %T, want []any", result["content"])
	}
	if len(content) != 1 {
		t.Fatalf("content count = %d, want 1", len(content))
	}
}

func TestServer_ToolsCall_UnknownTool(t *testing.T) {
	h := newTestServer(t)
	h.initialize()

	resp := h.call("tools/call", map[string]any{"name": "nonexistent"})
	assertErrorCode(t, resp, CodeInvalidParams)
}

func TestServer_ToolsCall_HandlerError(t *testing.T) {
	h := newTestServer(t)
	h.server.AddTool(&Tool{Name: "fail"}, func(_ context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return nil, fmt.Errorf("something broke")
	})

	h.initialize()
	resp := h.call("tools/call", map[string]any{"name": "fail"})
	assertNoError(t, resp) // Application errors → isError, not JSON-RPC error.

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map", resp["result"])
	}
	if result["isError"] != true {
		t.Fatalf("isError = %v, want true", result["isError"])
	}
}

func TestServer_ResourcesList(t *testing.T) {
	h := newTestServer(t)
	h.server.AddResource(&Resource{URI: "file:///test.txt", Name: "test"}, func(_ context.Context, req *ReadResourceRequest) (*ReadResourceResult, error) {
		return &ReadResourceResult{Contents: []*ResourceContents{{URI: req.URI, Text: "content"}}}, nil
	})

	h.initialize()
	resp := h.call("resources/list", map[string]any{})
	assertNoError(t, resp)

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map", resp["result"])
	}
	resources, ok := result["resources"].([]any)
	if !ok {
		t.Fatalf("resources type = %T, want []any", result["resources"])
	}
	if len(resources) != 1 {
		t.Fatalf("resources count = %d, want 1", len(resources))
	}
}

func TestServer_ResourcesTemplatesList(t *testing.T) {
	h := newTestServer(t)
	h.server.AddResourceTemplate(&ResourceTemplate{URITemplate: "file:///{path}", Name: "files"}, func(_ context.Context, req *ReadResourceRequest) (*ReadResourceResult, error) {
		return &ReadResourceResult{Contents: []*ResourceContents{{URI: req.URI, Text: "content"}}}, nil
	})

	h.initialize()
	resp := h.call("resources/templates/list", map[string]any{})
	assertNoError(t, resp)

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map", resp["result"])
	}
	templates, ok := result["resourceTemplates"].([]any)
	if !ok {
		t.Fatalf("templates type = %T, want []any", result["resourceTemplates"])
	}
	if len(templates) != 1 {
		t.Fatalf("templates count = %d, want 1", len(templates))
	}
}

func TestServer_ResourcesRead(t *testing.T) {
	h := newTestServer(t)
	h.server.AddResource(&Resource{URI: "file:///test.txt", Name: "test"}, func(_ context.Context, req *ReadResourceRequest) (*ReadResourceResult, error) {
		return &ReadResourceResult{Contents: []*ResourceContents{{URI: req.URI, Text: "hello world"}}}, nil
	})

	h.initialize()
	resp := h.call("resources/read", map[string]any{"uri": "file:///test.txt"})
	assertNoError(t, resp)

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map", resp["result"])
	}
	contents, ok := result["contents"].([]any)
	if !ok {
		t.Fatalf("contents type = %T, want []any", result["contents"])
	}
	if len(contents) != 1 {
		t.Fatalf("contents count = %d, want 1", len(contents))
	}
	c := contents[0].(map[string]any)
	if c["text"] != "hello world" {
		t.Fatalf("text = %v, want hello world", c["text"])
	}
}

func TestServer_ResourcesRead_Unknown(t *testing.T) {
	h := newTestServer(t)
	h.initialize()

	resp := h.call("resources/read", map[string]any{"uri": "file:///nonexistent"})
	assertErrorCode(t, resp, CodeInvalidParams)
}

func TestServer_PromptsList(t *testing.T) {
	h := newTestServer(t)
	h.server.AddPrompt(&Prompt{Name: "greeting", Description: "A greeting prompt"}, func(_ context.Context, req *GetPromptRequest) (*GetPromptResult, error) {
		return &GetPromptResult{Messages: []PromptMessage{{Role: RoleAssistant, Content: &TextContent{Type: "text", Text: "Hello!"}}}}, nil
	})

	h.initialize()
	resp := h.call("prompts/list", map[string]any{})
	assertNoError(t, resp)

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map", resp["result"])
	}
	prompts, ok := result["prompts"].([]any)
	if !ok {
		t.Fatalf("prompts type = %T, want []any", result["prompts"])
	}
	if len(prompts) != 1 {
		t.Fatalf("prompts count = %d, want 1", len(prompts))
	}
}

func TestServer_PromptsGet(t *testing.T) {
	h := newTestServer(t)
	h.server.AddPrompt(&Prompt{Name: "greeting"}, func(_ context.Context, req *GetPromptRequest) (*GetPromptResult, error) {
		return &GetPromptResult{Messages: []PromptMessage{{Role: RoleAssistant, Content: &TextContent{Type: "text", Text: "Hello!"}}}}, nil
	})

	h.initialize()
	resp := h.call("prompts/get", map[string]any{"name": "greeting"})
	assertNoError(t, resp)
}

func TestServer_PromptsGet_Unknown(t *testing.T) {
	h := newTestServer(t)
	h.initialize()

	resp := h.call("prompts/get", map[string]any{"name": "nonexistent"})
	assertErrorCode(t, resp, CodeInvalidParams)
}

func TestServer_PreHandshakeReject(t *testing.T) {
	h := newTestServer(t)
	// Don't initialize — send tools/list directly.
	resp := h.call("tools/list", map[string]any{})
	assertErrorCode(t, resp, CodeInvalidRequest)
}

func TestServer_CapabilitiesDynamic(t *testing.T) {
	h := newTestServer(t)
	h.server.AddTool(&Tool{Name: "t1"}, func(_ context.Context, _ *CallToolRequest) (*CallToolResult, error) {
		return &CallToolResult{}, nil
	})

	resp := h.initialize()
	assertNoError(t, resp)

	result := resp["result"].(map[string]any)
	caps := result["capabilities"].(map[string]any)

	if caps["tools"] == nil {
		t.Fatal("capabilities.tools should not be nil when tools are registered")
	}
	// No resources or prompts registered.
	if caps["resources"] != nil {
		t.Fatal("capabilities.resources should be nil when no resources registered")
	}
	if caps["prompts"] != nil {
		t.Fatal("capabilities.prompts should be nil when no prompts registered")
	}
}
