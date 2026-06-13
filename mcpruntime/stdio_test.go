package mcpruntime

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"testing"
)

func TestServeIO_RoundTrip(t *testing.T) {
	server := NewServer("stdio-test", "v0.0.1")
	server.AddTool(&Tool{Name: "echo"}, func(_ context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return &CallToolResult{Content: []Content{&TextContent{Type: "text", Text: "ok"}}}, nil
	})

	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeIO(ctx, server, inReader, outWriter)
		outWriter.Close()
	}()

	scanner := bufio.NewScanner(outReader)

	// 1. Send initialize.
	writeJSONRPC(t, inWriter, 1, "initialize", map[string]any{
		"protocolVersion": "2025-11-25",
		"capabilities":    map[string]any{},
	})
	if !scanner.Scan() {
		t.Fatal("no response for initialize")
	}
	initResp := parseResponse(t, scanner.Bytes())
	if initResp["error"] != nil {
		t.Fatalf("initialize error: %v", initResp["error"])
	}

	// 2. Send notifications/initialized.
	writeJSONRPCNotification(t, inWriter, "notifications/initialized", map[string]any{})

	// 3. Send tools/list.
	writeJSONRPC(t, inWriter, 2, "tools/list", map[string]any{})
	if !scanner.Scan() {
		t.Fatal("no response for tools/list")
	}
	toolsResp := parseResponse(t, scanner.Bytes())
	if toolsResp["error"] != nil {
		t.Fatalf("tools/list error: %v", toolsResp["error"])
	}
	result := toolsResp["result"].(map[string]any)
	tools := result["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools count = %d, want 1", len(tools))
	}

	// 4. Send tools/call.
	writeJSONRPC(t, inWriter, 3, "tools/call", map[string]any{
		"name":      "echo",
		"arguments": map[string]any{},
	})
	if !scanner.Scan() {
		t.Fatal("no response for tools/call")
	}
	callResp := parseResponse(t, scanner.Bytes())
	if callResp["error"] != nil {
		t.Fatalf("tools/call error: %v", callResp["error"])
	}

	// Close stdin → server should exit.
	inWriter.Close()
	if err := <-errCh; err != nil {
		t.Fatalf("ServeIO error: %v", err)
	}
}

func TestServeIO_EOF(t *testing.T) {
	server := NewServer("eof-test", "v0.0.1")

	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()

	ctx := context.Background()
	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeIO(ctx, server, inReader, outWriter)
		outWriter.Close()
	}()

	// Immediately close stdin.
	inWriter.Close()

	// Drain output to prevent deadlock.
	go func() { io.ReadAll(outReader) }()

	if err := <-errCh; err != nil {
		t.Fatalf("ServeIO should return nil on EOF, got: %v", err)
	}
}

func TestServeIO_ContextCancel(t *testing.T) {
	server := NewServer("cancel-test", "v0.0.1")

	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()
	defer inWriter.Close()

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- ServeIO(ctx, server, inReader, outWriter)
		outWriter.Close()
	}()

	// Drain output.
	go func() { io.ReadAll(outReader) }()

	// Cancel context.
	cancel()
	// Close the pipe to unblock the scanner.
	inWriter.Close()

	if err := <-errCh; err != nil {
		t.Fatalf("ServeIO should return nil on cancel, got: %v", err)
	}
}

func writeJSONRPC(t *testing.T, w io.Writer, id int, method string, params any) {
	t.Helper()

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	idBytes, _ := json.Marshal(id)
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      idBytes,
		Method:  method,
		Params:  paramsBytes,
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	data = append(data, '\n')
	if _, err := w.Write(data); err != nil {
		t.Fatalf("write request: %v", err)
	}
}

func writeJSONRPCNotification(t *testing.T, w io.Writer, method string, params any) {
	t.Helper()

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsBytes,
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}
	data = append(data, '\n')
	if _, err := w.Write(data); err != nil {
		t.Fatalf("write notification: %v", err)
	}
}

func parseResponse(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal response: %v (data: %s)", err, data)
	}
	return result
}
