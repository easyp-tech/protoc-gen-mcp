package examplemcp_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/easyp-tech/protoc-gen-mcp/internal/examplemcp"
)

// call sends a single JSON-RPC request through the server and returns the parsed
// response. It fails the test on transport-level errors.
func call(t *testing.T, srv interface {
	HandleRaw(context.Context, []byte) []byte
}, method string, params any,
) jsonrpcResponse {
	t.Helper()

	req := map[string]any{"jsonrpc": "2.0", "id": 1, "method": method}
	if params != nil {
		req["params"] = params
	}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	out := srv.HandleRaw(context.Background(), raw)
	var resp jsonrpcResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("unmarshal response for %s: %v (raw: %s)", method, err, out)
	}
	return resp
}

// readResourceText extracts the first resource content's text from a
// resources/read result.
func readResourceText(t *testing.T, result json.RawMessage) string {
	t.Helper()
	var res struct {
		Contents []struct {
			Text string `json:"text"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(result, &res); err != nil {
		t.Fatalf("unmarshal resources/read result: %v (raw: %s)", err, result)
	}
	if len(res.Contents) == 0 {
		t.Fatalf("resources/read returned no contents: %s", result)
	}
	return res.Contents[0].Text
}

func TestResourcesPromptsRoundTrip(t *testing.T) {
	srv, err := examplemcp.NewResourcesServer(context.Background())
	if err != nil {
		t.Fatalf("NewResourcesServer: %v", err)
	}

	// Handshake.
	if resp := call(t, srv, "initialize", map[string]any{"protocolVersion": "2025-11-25"}); resp.Error != nil {
		t.Fatalf("initialize error: %+v", resp.Error)
	}

	// resources/list → static server_status resource.
	resp := call(t, srv, "resources/list", nil)
	if resp.Error != nil {
		t.Fatalf("resources/list error: %+v", resp.Error)
	}
	if !strings.Contains(string(resp.Result), "server://status") {
		t.Fatalf("resources/list missing static resource: %s", resp.Result)
	}

	// resources/templates/list → both templates.
	resp = call(t, srv, "resources/templates/list", nil)
	if resp.Error != nil {
		t.Fatalf("resources/templates/list error: %+v", resp.Error)
	}
	for _, want := range []string{"users://{user_id}/profile", "projects://{project_id}/documents/{document_id}"} {
		if !strings.Contains(string(resp.Result), want) {
			t.Fatalf("templates/list missing %q: %s", want, resp.Result)
		}
	}

	// resources/read static → ProtoJSON body of ServerStatus.
	resp = call(t, srv, "resources/read", map[string]any{"uri": "server://status"})
	if resp.Error != nil {
		t.Fatalf("resources/read static error: %+v", resp.Error)
	}
	if text := readResourceText(t, resp.Result); !strings.Contains(text, `"healthy":true`) {
		t.Fatalf("resources/read static missing ProtoJSON body: %s", text)
	}

	// resources/read template → URI param routed into the handler.
	resp = call(t, srv, "resources/read", map[string]any{"uri": "users://ada/profile"})
	if resp.Error != nil {
		t.Fatalf("resources/read template error: %+v", resp.Error)
	}
	if text := readResourceText(t, resp.Result); !strings.Contains(text, `"userId":"ada"`) {
		t.Fatalf("resources/read template did not route uri param: %s", text)
	}

	// prompts/list → all three prompts.
	resp = call(t, srv, "prompts/list", nil)
	if resp.Error != nil {
		t.Fatalf("prompts/list error: %+v", resp.Error)
	}
	for _, want := range []string{"code_review", "summarize", "explain_error"} {
		if !strings.Contains(string(resp.Result), want) {
			t.Fatalf("prompts/list missing %q: %s", want, resp.Result)
		}
	}

	// prompts/get → arguments parsed into the proto message and rendered.
	resp = call(t, srv, "prompts/get", map[string]any{
		"name":      "code_review",
		"arguments": map[string]string{"code": "print(1)", "language": "python"},
	})
	if resp.Error != nil {
		t.Fatalf("prompts/get error: %+v", resp.Error)
	}
	if !strings.Contains(string(resp.Result), "Review this python code: print(1)") {
		t.Fatalf("prompts/get did not render arguments: %s", resp.Result)
	}
}

// TestPromptsGetMissingRequiredArg verifies required-argument enforcement in the
// generated prompt handler wiring.
func TestPromptsGetMissingRequiredArg(t *testing.T) {
	srv, err := examplemcp.NewResourcesServer(context.Background())
	if err != nil {
		t.Fatalf("NewResourcesServer: %v", err)
	}
	if resp := call(t, srv, "initialize", map[string]any{"protocolVersion": "2025-11-25"}); resp.Error != nil {
		t.Fatalf("initialize error: %+v", resp.Error)
	}

	resp := call(t, srv, "prompts/get", map[string]any{
		"name":      "code_review",
		"arguments": map[string]string{"code": "print(1)"}, // missing required "language"
	})
	if resp.Error == nil {
		t.Fatalf("expected error for missing required argument, got result: %s", resp.Result)
	}
}
