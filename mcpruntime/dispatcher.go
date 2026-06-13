package mcpruntime

import (
	"context"
	"encoding/json"
)

// methodHandler processes a JSON-RPC method call and returns a result or error.
type methodHandler func(ctx context.Context, params json.RawMessage) (any, error)

// jsonrpcRequest represents a JSON-RPC 2.0 request message.
type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// jsonrpcResponse represents a JSON-RPC 2.0 response message.
type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// dispatcher routes JSON-RPC method calls to registered handlers.
type dispatcher struct {
	handlers map[string]methodHandler
}

// newDispatcher creates a new dispatcher with an empty handler map.
func newDispatcher() *dispatcher {
	return &dispatcher{
		handlers: make(map[string]methodHandler),
	}
}

// register adds a handler for the given JSON-RPC method name.
func (d *dispatcher) register(method string, handler methodHandler) {
	d.handlers[method] = handler
}

// dispatch processes a raw JSON-RPC message and returns the serialized response.
// For notifications (requests without an id), the handler is called but nil is returned.
// For batch requests (JSON array), each element is dispatched individually and
// an array of responses is returned.
func (d *dispatcher) dispatch(ctx context.Context, raw []byte) []byte {
	if len(raw) == 0 {
		return marshalResponse(jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage("null"),
			Error:   &JSONRPCError{Code: CodeParseError, Message: "Parse error"},
		})
	}

	// Try to detect batch (JSON array).
	trimmed := trimLeft(raw)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		return d.dispatchBatch(ctx, raw)
	}

	return d.dispatchSingle(ctx, raw)
}

// dispatchSingle handles a single JSON-RPC request.
func (d *dispatcher) dispatchSingle(ctx context.Context, raw []byte) []byte {
	var req jsonrpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return marshalResponse(jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage("null"),
			Error:   &JSONRPCError{Code: CodeParseError, Message: "Parse error"},
		})
	}

	handler, ok := d.handlers[req.Method]
	if !ok {
		// If it's a notification (no id), silently ignore unknown methods.
		if req.ID == nil {
			return nil
		}
		return marshalResponse(jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &JSONRPCError{Code: CodeMethodNotFound, Message: "Method not found: " + req.Method},
		})
	}

	// Call handler.
	result, err := handler(ctx, req.Params)

	// If it's a notification (no id), don't send a response.
	if req.ID == nil {
		return nil
	}

	if err != nil {
		// Check if the error is already a JSONRPCError.
		if rpcErr, ok := err.(*JSONRPCError); ok {
			return marshalResponse(jsonrpcResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   rpcErr,
			})
		}
		// Wrap unknown errors as internal errors.
		return marshalResponse(jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &JSONRPCError{Code: CodeInternalError, Message: err.Error()},
		})
	}

	return marshalResponse(jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	})
}

// dispatchBatch handles a batch of JSON-RPC requests.
func (d *dispatcher) dispatchBatch(ctx context.Context, raw []byte) []byte {
	var batch []json.RawMessage
	if err := json.Unmarshal(raw, &batch); err != nil {
		return marshalResponse(jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage("null"),
			Error:   &JSONRPCError{Code: CodeParseError, Message: "Parse error"},
		})
	}

	if len(batch) == 0 {
		return marshalResponse(jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage("null"),
			Error:   &JSONRPCError{Code: CodeInvalidRequest, Message: "Invalid Request: empty batch"},
		})
	}

	var responses []json.RawMessage
	for _, item := range batch {
		resp := d.dispatchSingle(ctx, item)
		if resp != nil {
			responses = append(responses, resp)
		}
	}

	if len(responses) == 0 {
		return nil
	}

	result, err := json.Marshal(responses)
	if err != nil {
		return marshalResponse(jsonrpcResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage("null"),
			Error:   &JSONRPCError{Code: CodeInternalError, Message: "Internal error: failed to marshal batch response"},
		})
	}

	return result
}

// marshalResponse serializes a JSON-RPC response to bytes.
func marshalResponse(resp jsonrpcResponse) []byte {
	data, err := json.Marshal(resp)
	if err != nil {
		// Fallback: return a minimal error response.
		return []byte(`{"jsonrpc":"2.0","id":null,"error":{"code":-32603,"message":"Internal error"}}`)
	}
	return data
}

// trimLeft returns raw with leading ASCII whitespace removed.
func trimLeft(raw []byte) []byte {
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return raw[i:]
		}
	}
	return nil
}
