package mcpruntime

import (
	"context"
	"encoding/json"
	"testing"
)

func TestMultiSession_IndependentInitialize(t *testing.T) {
	server := NewServer("multi", "v1")
	server.AddTool(&Tool{Name: "echo"}, func(_ context.Context, _ *CallToolRequest) (*CallToolResult, error) {
		return &CallToolResult{Content: []Content{&TextContent{Type: "text", Text: "ok"}}}, nil
	})

	s1 := NewSession()
	s2 := NewSession()

	init1 := server.HandleRaw(WithSession(context.Background(), s1), mustMarshal(t, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{"protocolVersion": "2025-11-25"},
	}))
	assertRawNoError(t, init1)

	init2 := server.HandleRaw(WithSession(context.Background(), s2), mustMarshal(t, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{"protocolVersion": "2025-11-25"},
	}))
	assertRawNoError(t, init2)

	// Second initialize on same session fails.
	again := server.HandleRaw(WithSession(context.Background(), s1), mustMarshal(t, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "initialize",
		"params": map[string]any{"protocolVersion": "2025-11-25"},
	}))
	assertRawErrorCode(t, again, CodeInvalidRequest)

	// Both sessions can call tools.
	for _, s := range []*Session{s1, s2} {
		resp := server.HandleRaw(WithSession(context.Background(), s), mustMarshal(t, map[string]any{
			"jsonrpc": "2.0", "id": 3, "method": "tools/list", "params": map[string]any{},
		}))
		assertRawNoError(t, resp)
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func assertRawNoError(t *testing.T, raw []byte) {
	t.Helper()
	var resp map[string]any
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v raw=%s", err, raw)
	}
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
}

func assertRawErrorCode(t *testing.T, raw []byte, want int) {
	t.Helper()
	var resp map[string]any
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v raw=%s", err, raw)
	}
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got %v", resp)
	}
	code, _ := errObj["code"].(float64)
	if int(code) != want {
		t.Fatalf("error code = %d, want %d", int(code), want)
	}
}
