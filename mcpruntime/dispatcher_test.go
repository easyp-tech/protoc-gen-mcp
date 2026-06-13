package mcpruntime

import (
	"context"
	"encoding/json"
	"testing"
)

func TestDispatcher_ValidRequest(t *testing.T) {
	d := newDispatcher()
	d.register("test/echo", func(_ context.Context, params json.RawMessage) (any, error) {
		var input map[string]any
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, err
		}
		return input, nil
	})

	req := `{"jsonrpc":"2.0","id":1,"method":"test/echo","params":{"hello":"world"}}`
	resp := d.dispatch(context.Background(), []byte(req))

	var result jsonrpcResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.JSONRPC != "2.0" {
		t.Fatalf("jsonrpc = %q, want 2.0", result.JSONRPC)
	}

	resultMap, ok := result.Result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result.Result)
	}
	if resultMap["hello"] != "world" {
		t.Fatalf("result[hello] = %v, want world", resultMap["hello"])
	}
}

func TestDispatcher_UnknownMethod(t *testing.T) {
	d := newDispatcher()

	req := `{"jsonrpc":"2.0","id":1,"method":"unknown/method"}`
	resp := d.dispatch(context.Background(), []byte(req))

	var result jsonrpcResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error, got nil")
	}
	if result.Error.Code != CodeMethodNotFound {
		t.Fatalf("error code = %d, want %d", result.Error.Code, CodeMethodNotFound)
	}
}

func TestDispatcher_InvalidJSON(t *testing.T) {
	d := newDispatcher()

	resp := d.dispatch(context.Background(), []byte("{broken"))

	var result jsonrpcResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error, got nil")
	}
	if result.Error.Code != CodeParseError {
		t.Fatalf("error code = %d, want %d", result.Error.Code, CodeParseError)
	}
}

func TestDispatcher_Notification(t *testing.T) {
	called := false
	d := newDispatcher()
	d.register("test/notify", func(_ context.Context, _ json.RawMessage) (any, error) {
		called = true
		return nil, nil
	})

	// Notification: no "id" field.
	req := `{"jsonrpc":"2.0","method":"test/notify","params":{}}`
	resp := d.dispatch(context.Background(), []byte(req))

	if resp != nil {
		t.Fatalf("notification should return nil response, got %s", resp)
	}
	if !called {
		t.Fatal("notification handler was not called")
	}
}

func TestDispatcher_Batch(t *testing.T) {
	d := newDispatcher()
	d.register("test/add", func(_ context.Context, params json.RawMessage) (any, error) {
		var input struct {
			A float64 `json:"a"`
			B float64 `json:"b"`
		}
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, err
		}
		return map[string]float64{"sum": input.A + input.B}, nil
	})

	batch := `[
		{"jsonrpc":"2.0","id":1,"method":"test/add","params":{"a":1,"b":2}},
		{"jsonrpc":"2.0","id":2,"method":"test/add","params":{"a":3,"b":4}}
	]`
	resp := d.dispatch(context.Background(), []byte(batch))

	var results []jsonrpcResponse
	if err := json.Unmarshal(resp, &results); err != nil {
		t.Fatalf("unmarshal batch response: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("batch response count = %d, want 2", len(results))
	}
	for _, r := range results {
		if r.Error != nil {
			t.Fatalf("unexpected error in batch: %v", r.Error)
		}
	}
}

func TestDispatcher_EmptyInput(t *testing.T) {
	d := newDispatcher()

	resp := d.dispatch(context.Background(), []byte{})

	var result jsonrpcResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error, got nil")
	}
	if result.Error.Code != CodeParseError {
		t.Fatalf("error code = %d, want %d", result.Error.Code, CodeParseError)
	}
}

func TestDispatcher_HandlerReturnsJSONRPCError(t *testing.T) {
	d := newDispatcher()
	d.register("test/fail", func(_ context.Context, _ json.RawMessage) (any, error) {
		return nil, &JSONRPCError{Code: CodeInvalidParams, Message: "bad params"}
	})

	req := `{"jsonrpc":"2.0","id":1,"method":"test/fail"}`
	resp := d.dispatch(context.Background(), []byte(req))

	var result jsonrpcResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error, got nil")
	}
	if result.Error.Code != CodeInvalidParams {
		t.Fatalf("error code = %d, want %d", result.Error.Code, CodeInvalidParams)
	}
}
