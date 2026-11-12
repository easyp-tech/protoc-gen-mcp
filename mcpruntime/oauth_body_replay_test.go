package mcpruntime

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestOAuthScopePreflightReplaysAcceptedBody(t *testing.T) {
	const message = `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"write","arguments":{}}}`
	for _, tc := range []struct {
		name    string
		chunked bool
		atLimit bool
	}{
		{name: "fixed-length"},
		{name: "chunked", chunked: true},
		{name: "fixed-length-at-limit", atLimit: true},
		{name: "chunked-at-limit", chunked: true, atLimit: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			handler := newStepUpTestHandler(t, &calls)
			body := message
			if tc.atLimit {
				// Whitespace is valid JSON padding and exercises the precise limit
				// without changing the protobuf tool's argument schema.
				body = strings.Repeat(" ", mcp.DefaultMaxRequestBodyBytes-len(message)) + message
			}
			req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
			if tc.chunked {
				req.ContentLength = -1
				req.TransferEncoding = []string{"chunked"}
			}
			req.Header.Set("Authorization", "Bearer writer")
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			req.Header.Set("MCP-Protocol-Version", "2025-03-26")
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status=%d, want 200: %s", recorder.Code, recorder.Body)
			}
			if calls.Load() != 1 {
				t.Fatalf("handler calls=%d, want 1", calls.Load())
			}
			if !strings.Contains(recorder.Body.String(), `"id":7`) {
				t.Fatalf("request ID was not preserved through preflight body replay: %s", recorder.Body)
			}
		})
	}
}
