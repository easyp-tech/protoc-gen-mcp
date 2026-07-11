package mcpruntime

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newHTTPTestServer(t *testing.T, opts StreamableHTTPOptions) (*Server, http.Handler) {
	t.Helper()
	server := NewServer("http-test", "v0.0.1")
	server.AddTool(&Tool{Name: "echo", Description: "echo"}, func(_ context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return &CallToolResult{Content: []Content{&TextContent{Type: "text", Text: "pong"}}}, nil
	})
	opts.AllowAllOrigins = true
	opts.HeartbeatInterval = -1 // disable in tests
	h := NewStreamableHTTPHandler(server, opts)
	return server, h
}

func mcpPOST(t *testing.T, h http.Handler, body any, sessionID string, extra http.Header) *http.Response {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(raw))
	req.Header.Set("Content-Type", contentTypeJSON)
	req.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		req.Header.Set(headerMCPSessionID, sessionID)
		req.Header.Set(headerMCPProtocolVersion, "2025-11-25")
	}
	for k, vs := range extra {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Result()
}

func readJSONBody(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return out
}

func TestStreamableHTTP_InitializeAndTools(t *testing.T) {
	_, h := newHTTPTestServer(t, StreamableHTTPOptions{})

	resp := mcpPOST(t, h, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-11-25",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "t", "version": "1"},
		},
	}, "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	sessionID := resp.Header.Get(headerMCPSessionID)
	if sessionID == "" {
		t.Fatal("missing MCP-Session-Id")
	}
	body := readJSONBody(t, resp)
	if body["error"] != nil {
		t.Fatalf("initialize error: %v", body["error"])
	}

	// notifications/initialized → 202
	resp = mcpPOST(t, h, map[string]any{
		"jsonrpc": "2.0", "method": "notifications/initialized", "params": map[string]any{},
	}, sessionID, nil)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("initialized status = %d, want 202", resp.StatusCode)
	}
	resp.Body.Close()

	resp = mcpPOST(t, h, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]any{},
	}, sessionID, nil)
	body = readJSONBody(t, resp)
	if body["error"] != nil {
		t.Fatalf("tools/list error: %v", body["error"])
	}
	result := body["result"].(map[string]any)
	tools := result["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools = %d, want 1", len(tools))
	}

	resp = mcpPOST(t, h, map[string]any{
		"jsonrpc": "2.0", "id": 3, "method": "tools/call",
		"params": map[string]any{"name": "echo", "arguments": map[string]any{}},
	}, sessionID, nil)
	body = readJSONBody(t, resp)
	if body["error"] != nil {
		t.Fatalf("tools/call error: %v", body["error"])
	}
}

func TestStreamableHTTP_MultiClient(t *testing.T) {
	_, h := newHTTPTestServer(t, StreamableHTTPOptions{})

	initSession := func() string {
		resp := mcpPOST(t, h, map[string]any{
			"jsonrpc": "2.0", "id": 1, "method": "initialize",
			"params": map[string]any{"protocolVersion": "2025-11-25"},
		}, "", nil)
		id := resp.Header.Get(headerMCPSessionID)
		readJSONBody(t, resp)
		resp2 := mcpPOST(t, h, map[string]any{
			"jsonrpc": "2.0", "method": "notifications/initialized",
		}, id, nil)
		resp2.Body.Close()
		return id
	}

	s1 := initSession()
	s2 := initSession()
	if s1 == s2 {
		t.Fatal("sessions should be unique")
	}

	for _, id := range []string{s1, s2} {
		resp := mcpPOST(t, h, map[string]any{
			"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]any{},
		}, id, nil)
		body := readJSONBody(t, resp)
		if body["error"] != nil {
			t.Fatalf("session %s tools/list error: %v", id, body["error"])
		}
	}
}

func TestStreamableHTTP_OriginForbidden(t *testing.T) {
	server := NewServer("http-test", "v0.0.1")
	h := NewStreamableHTTPHandler(server, StreamableHTTPOptions{
		AllowedOrigins:    []string{"http://localhost:3000"},
		HeartbeatInterval: -1,
	})

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req.Header.Set("Content-Type", contentTypeJSON)
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestStreamableHTTP_MissingSession(t *testing.T) {
	_, h := newHTTPTestServer(t, StreamableHTTPOptions{})
	resp := mcpPOST(t, h, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": map[string]any{},
	}, "", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestStreamableHTTP_DeleteSession(t *testing.T) {
	_, h := newHTTPTestServer(t, StreamableHTTPOptions{})

	resp := mcpPOST(t, h, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{"protocolVersion": "2025-11-25"},
	}, "", nil)
	sessionID := resp.Header.Get(headerMCPSessionID)
	readJSONBody(t, resp)

	req := httptest.NewRequest(http.MethodDelete, "/mcp", nil)
	req.Header.Set(headerMCPSessionID, sessionID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want 204", rec.Code)
	}

	resp = mcpPOST(t, h, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]any{},
	}, sessionID, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("after DELETE status = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestStreamableHTTP_PreferSSE(t *testing.T) {
	_, h := newHTTPTestServer(t, StreamableHTTPOptions{PreferSSE: true})

	resp := mcpPOST(t, h, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{"protocolVersion": "2025-11-25"},
	}, "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, contentTypeSSE) {
		t.Fatalf("Content-Type = %q, want SSE", ct)
	}
	sessionID := resp.Header.Get(headerMCPSessionID)
	if sessionID == "" {
		t.Fatal("missing session id on SSE initialize")
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(raw), "data:") {
		t.Fatalf("expected SSE data frames, got %q", raw)
	}
	if !strings.Contains(string(raw), "protocolVersion") {
		t.Fatalf("expected initialize result in SSE body: %s", raw)
	}
}

func TestStreamableHTTP_GETListenAndNotify(t *testing.T) {
	_, h := newHTTPTestServer(t, StreamableHTTPOptions{})
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)

	// Initialize session via real HTTP.
	initBody, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{"protocolVersion": "2025-11-25"},
	})
	initReq, _ := http.NewRequest(http.MethodPost, ts.URL, bytes.NewReader(initBody))
	initReq.Header.Set("Content-Type", contentTypeJSON)
	initReq.Header.Set("Accept", "application/json, text/event-stream")
	initResp, err := http.DefaultClient.Do(initReq)
	if err != nil {
		t.Fatal(err)
	}
	sessionID := initResp.Header.Get(headerMCPSessionID)
	_, _ = io.ReadAll(initResp.Body)
	initResp.Body.Close()
	if sessionID == "" {
		t.Fatal("missing session id")
	}

	notifBody, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	nreq, _ := http.NewRequest(http.MethodPost, ts.URL, bytes.NewReader(notifBody))
	nreq.Header.Set("Content-Type", contentTypeJSON)
	nreq.Header.Set("Accept", "application/json, text/event-stream")
	nreq.Header.Set(headerMCPSessionID, sessionID)
	nreq.Header.Set(headerMCPProtocolVersion, "2025-11-25")
	nresp, err := http.DefaultClient.Do(nreq)
	if err != nil {
		t.Fatal(err)
	}
	nresp.Body.Close()

	// Open GET SSE stream.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	greq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL, nil)
	greq.Header.Set("Accept", contentTypeSSE)
	greq.Header.Set(headerMCPSessionID, sessionID)
	greq.Header.Set(headerMCPProtocolVersion, "2025-11-25")
	gresp, err := http.DefaultClient.Do(greq)
	if err != nil {
		t.Fatal(err)
	}
	defer gresp.Body.Close()
	if gresp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d", gresp.StatusCode)
	}
	if !strings.HasPrefix(gresp.Header.Get("Content-Type"), contentTypeSSE) {
		t.Fatalf("Content-Type = %q", gresp.Header.Get("Content-Type"))
	}

	reader := bufio.NewReader(gresp.Body)
	lastID := readSSEUntil(t, reader, 2*time.Second, func(id, data string) bool {
		// Prime event has empty data.
		return id != ""
	})

	sh := h.(*streamableHandler)
	sess := sh.sessions.Get(sessionID)
	if sess == nil {
		t.Fatal("session missing")
	}
	if err := sess.SendNotification("notifications/message", map[string]any{"level": "info", "data": "hi"}); err != nil {
		t.Fatalf("SendNotification: %v", err)
	}

	lastID = readSSEUntil(t, reader, 2*time.Second, func(id, data string) bool {
		return strings.Contains(data, "notifications/message")
	})
	if lastID == "" {
		t.Fatal("expected event id for notification")
	}

	// Close first GET.
	cancel()
	_, _ = io.Copy(io.Discard, gresp.Body)

	// Queue event while disconnected.
	if err := sess.SendNotification("notifications/message", map[string]any{"level": "info", "data": "queued"}); err != nil {
		t.Fatalf("SendNotification queued: %v", err)
	}

	// Resume with Last-Event-ID.
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	greq2, _ := http.NewRequestWithContext(ctx2, http.MethodGet, ts.URL, nil)
	greq2.Header.Set("Accept", contentTypeSSE)
	greq2.Header.Set(headerMCPSessionID, sessionID)
	greq2.Header.Set(headerMCPProtocolVersion, "2025-11-25")
	greq2.Header.Set(headerLastEventID, lastID)
	gresp2, err := http.DefaultClient.Do(greq2)
	if err != nil {
		t.Fatal(err)
	}
	defer gresp2.Body.Close()

	reader2 := bufio.NewReader(gresp2.Body)
	_ = readSSEUntil(t, reader2, 2*time.Second, func(id, data string) bool {
		return strings.Contains(data, "queued")
	})
	cancel2()
}

// readSSEUntil reads SSE frames until match returns true or timeout.
// Returns the last seen event id.
func readSSEUntil(t *testing.T, reader *bufio.Reader, timeout time.Duration, match func(id, data string) bool) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var curID, curData string
	lastID := ""
	type lineResult struct {
		line string
		err  error
	}
	for time.Now().Before(deadline) {
		ch := make(chan lineResult, 1)
		go func() {
			line, err := reader.ReadString('\n')
			ch <- lineResult{line, err}
		}()
		var lr lineResult
		select {
		case lr = <-ch:
		case <-time.After(time.Until(deadline)):
			t.Fatal("timeout waiting for SSE")
		}
		if lr.err != nil {
			t.Fatalf("read SSE: %v", lr.err)
		}
		line := lr.line
		switch {
		case strings.HasPrefix(line, "id: "):
			curID = strings.TrimSpace(strings.TrimPrefix(line, "id: "))
			lastID = curID
		case strings.HasPrefix(line, "data:"):
			curData = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			curData = strings.TrimSpace(curData)
		case line == "\n" || line == "\r\n":
			if match(curID, curData) {
				return lastID
			}
			curID, curData = "", ""
		}
	}
	t.Fatal("timeout waiting for matching SSE event")
	return lastID
}

func TestStreamableHTTP_UnsupportedProtocolVersion(t *testing.T) {
	_, h := newHTTPTestServer(t, StreamableHTTPOptions{})
	resp := mcpPOST(t, h, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{"protocolVersion": "2025-11-25"},
	}, "", nil)
	sessionID := resp.Header.Get(headerMCPSessionID)
	readJSONBody(t, resp)

	extra := http.Header{}
	extra.Set(headerMCPProtocolVersion, "1999-01-01")
	// Override default version header by building request manually.
	raw, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]any{},
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(raw))
	req.Header.Set("Content-Type", contentTypeJSON)
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set(headerMCPSessionID, sessionID)
	req.Header.Set(headerMCPProtocolVersion, "1999-01-01")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestAcceptIncludes(t *testing.T) {
	if !acceptIncludes("application/json, text/event-stream", contentTypeJSON) {
		t.Fatal("expected json accepted")
	}
	if !acceptIncludes("application/json, text/event-stream", contentTypeSSE) {
		t.Fatal("expected sse accepted")
	}
	if acceptIncludes("application/json", contentTypeSSE) {
		t.Fatal("sse should not be accepted")
	}
}
