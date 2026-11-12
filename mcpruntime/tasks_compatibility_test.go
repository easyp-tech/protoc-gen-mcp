package mcpruntime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/types/known/emptypb"
)

// This is an executable compatibility boundary, not an implementation of Tasks.
// The declarations in options.proto describe the 2025-11-25 protocol. Its
// top-level task opt-in is lost before the SDK's receiving middleware, and a
// custom method cannot replace tools/call. If an SDK upgrade changes these
// probes, revisit the Go generation-time rejection and test the entire task
// lifecycle before advertising support. The 2026-07-28 Tasks extension is a
// different protocol, with server-directed creation and no execution.taskSupport.
func TestSDKTasksCompatibilityBoundary(t *testing.T) {
	for _, test := range []struct {
		name, field, wire string
		value             any
	}{
		{"client task opt-in", "task", `{"name":"work","task":{"ttl":1000}}`, &mcp.CallToolParams{}},
		{"server task opt-in", "task", `{"name":"work","task":{"ttl":1000}}`, &mcp.CallToolParamsRaw{}},
		{"tool execution", "execution", `{"name":"work","inputSchema":{},"execution":{"taskSupport":"required"}}`, &mcp.Tool{}},
		{"server capability", "tasks", `{"tasks":{"list":{},"cancel":{},"requests":{"tools":{"call":{}}}}}`, &mcp.ServerCapabilities{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(test.wire), test.value); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(test.value)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			if _, exists := fields[test.field]; exists {
				t.Fatalf("SDK now preserves %q: reassess Go task-support rejection and add lifecycle coverage before enabling Tasks", test.field)
			}
		})
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "task-boundary", Version: "v1"}, nil)
	err := mcp.AddReceivingCustomMethod(server, "tools/call", func(context.Context, *mcp.ServerSession, *mcp.ParamsBase) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{}, nil
	})
	if err == nil {
		t.Fatalf("SDK tools/call replacement boundary changed: %v; reassess Go Tasks integration", err)
	}
}

// Use raw JSON-RPC because an SDK client would itself erase unsupported fields
// and could hide an incorrect wire contract from the test.
func TestSDKTasksCompatibilitySynchronousWireContract(t *testing.T) {
	for _, version := range []string{"2025-11-25", "2026-07-28"} {
		t.Run(version, func(t *testing.T) {
			server, err := NewConfiguredServer("task-boundary", "v1", true)
			if err != nil {
				t.Fatal(err)
			}
			var calls atomic.Int32
			if err := RegisterSDKProtoTool(server, ToolSpec[*emptypb.Empty, *emptypb.Empty]{
				Name: "sync", InputSchemaJSON: `{"type":"object"}`, OutputSchemaJSON: `{"type":"object"}`,
				NewRequest: func() *emptypb.Empty { return &emptypb.Empty{} }, NewResponse: func() *emptypb.Empty { return &emptypb.Empty{} },
				Handler: func(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
					calls.Add(1)
					return &emptypb.Empty{}, nil
				},
			}); err != nil {
				t.Fatal(err)
			}
			observed := make(chan json.RawMessage, 2)
			server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
				return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
					if method == "tools/call" {
						raw, err := json.Marshal(req.GetParams())
						if err != nil {
							return nil, err
						}
						observed <- raw
					}
					return next(ctx, method, req)
				}
			})
			call := taskCompatibilityWireClient(t, server, version)
			var discovery map[string]json.RawMessage
			method := "server/discover"
			params := map[string]any{}
			if version == "2025-11-25" {
				method = "initialize"
				params = map[string]any{"protocolVersion": version, "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "tasks-test", "version": "v1"}}
			}
			if response := call(method, params); response.Error != nil {
				t.Fatal(response.Error)
			} else if err := json.Unmarshal(response.Result, &discovery); err != nil {
				t.Fatal(err)
			}
			var capabilities struct {
				Tasks      json.RawMessage            `json:"tasks"`
				Extensions map[string]json.RawMessage `json:"extensions"`
			}
			if err := json.Unmarshal(discovery["capabilities"], &capabilities); err != nil {
				t.Fatal(err)
			}
			if len(capabilities.Tasks) != 0 || len(capabilities.Extensions["io.modelcontextprotocol/tasks"]) != 0 {
				t.Fatal("server advertised a task lifecycle that it does not implement")
			}
			if version == "2025-11-25" {
				call("notifications/initialized", nil)
			}
			list := call("tools/list", map[string]any{})
			if list.Error != nil {
				t.Fatal(list.Error)
			}
			var listed struct {
				Tools []map[string]json.RawMessage `json:"tools"`
			}
			if err := json.Unmarshal(list.Result, &listed); err != nil {
				t.Fatal(err)
			}
			if len(listed.Tools) != 1 || len(listed.Tools[0]["execution"]) != 0 {
				t.Fatalf("unexpected synchronous tools/list contract: %s", list.Result)
			}
			for _, task := range []any{nil, map[string]any{"ttl": 1000}} {
				params := map[string]any{"name": "sync", "arguments": map[string]any{}}
				if task != nil {
					params["task"] = task
				}
				response := call("tools/call", params)
				if response.Error != nil {
					t.Fatal(response.Error)
				}
				var result map[string]json.RawMessage
				if err := json.Unmarshal(response.Result, &result); err != nil {
					t.Fatal(err)
				}
				if len(result["task"]) != 0 || len(result["content"]) == 0 || string(result["structuredContent"]) != "{}" {
					t.Fatalf("expected ordinary protobuf result, got %s", response.Result)
				}
				var middlewareParams map[string]json.RawMessage
				select {
				case raw := <-observed:
					if err := json.Unmarshal(raw, &middlewareParams); err != nil {
						t.Fatal(err)
					}
				default:
					t.Fatal("tools/call returned without invoking SDK receiving middleware")
				}
				if len(middlewareParams["task"]) != 0 {
					t.Fatal("SDK task opt-in now reaches middleware; reassess Go Tasks integration")
				}
			}
			if got := calls.Load(); got != 2 {
				t.Fatalf("synchronous handler calls = %d, want 2", got)
			}
			for _, method := range []string{"tasks/get", "tasks/result", "tasks/list", "tasks/cancel", "tasks/update"} {
				response := call(method, map[string]any{"taskId": "not-a-task"})
				var rpcError *jsonrpc.Error
				if !errors.As(response.Error, &rpcError) || rpcError.Code != jsonrpc.CodeMethodNotFound {
					t.Fatalf("%s error = %v, want method not found", method, response.Error)
				}
			}
		})
	}
}

func taskCompatibilityWireClient(t *testing.T, server *mcp.Server, version string) func(string, map[string]any) *jsonrpc.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	session, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	connection, err := clientTransport.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	id := 0
	return func(method string, params map[string]any) *jsonrpc.Response {
		t.Helper()
		if version == "2026-07-28" {
			if params == nil {
				params = map[string]any{}
			}
			params["_meta"] = map[string]any{
				"io.modelcontextprotocol/protocolVersion": version,
				"io.modelcontextprotocol/clientInfo":      map[string]any{"name": "tasks-test", "version": "v1"},
				"io.modelcontextprotocol/clientCapabilities": map[string]any{
					"extensions": map[string]any{"io.modelcontextprotocol/tasks": map[string]any{}},
				},
			}
		}
		wire := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
		notification := strings.HasPrefix(method, "notifications/")
		if !notification {
			id++
			wire["id"] = id
		}
		encoded, err := json.Marshal(wire)
		if err != nil {
			t.Fatal(err)
		}
		message, err := jsonrpc.DecodeMessage(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if err := connection.Write(ctx, message); err != nil {
			t.Fatal(err)
		}
		requestID := message.(*jsonrpc.Request).ID
		if notification {
			return nil
		}
		for {
			message, err := connection.Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if response, ok := message.(*jsonrpc.Response); ok {
				if response.ID != requestID {
					t.Fatalf("response ID %v does not match request %v", response.ID, requestID)
				}
				return response
			}
		}
	}
}
