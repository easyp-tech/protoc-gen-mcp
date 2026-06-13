package mcpruntime_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	examplev1 "github.com/easyp-tech/protoc-gen-mcp/internal/testproto/example/v1"
	"github.com/easyp-tech/protoc-gen-mcp/mcpruntime"
	"github.com/google/jsonschema-go/jsonschema"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// testClient is a minimal JSON-RPC client for testing, communicating via io.Pipe.
type testClient struct {
	t       *testing.T
	scanner *bufio.Scanner
	writer  io.Writer
	nextID  int
}

type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolsListResult struct {
	Tools []toolInfo `json:"tools"`
}

type toolInfo struct {
	Name        string     `json:"name"`
	Title       string     `json:"title,omitempty"`
	Description string     `json:"description,omitempty"`
	InputSchema any        `json:"inputSchema,omitempty"`
	Annotations any        `json:"annotations,omitempty"`
	Icons       any        `json:"icons,omitempty"`
}

type callToolResult struct {
	Content           []json.RawMessage `json:"content,omitempty"`
	StructuredContent json.RawMessage   `json:"structuredContent,omitempty"`
	IsError           bool              `json:"isError,omitempty"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (c *testClient) call(method string, params any) jsonrpcResponse {
	c.t.Helper()

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		c.t.Fatalf("marshal params: %v", err)
	}

	id := c.nextID
	c.nextID++

	idBytes, _ := json.Marshal(id)
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      idBytes,
		Method:  method,
		Params:  paramsBytes,
	}
	data, err := json.Marshal(req)
	if err != nil {
		c.t.Fatalf("marshal request: %v", err)
	}
	data = append(data, '\n')
	if _, err := c.writer.Write(data); err != nil {
		c.t.Fatalf("write request: %v", err)
	}

	if !c.scanner.Scan() {
		c.t.Fatalf("no response for %s (scanner error: %v)", method, c.scanner.Err())
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(c.scanner.Bytes(), &resp); err != nil {
		c.t.Fatalf("unmarshal response for %s: %v", method, err)
	}
	return resp
}

func (c *testClient) notify(method string, params any) {
	c.t.Helper()

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		c.t.Fatalf("marshal params: %v", err)
	}

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsBytes,
	}
	data, err := json.Marshal(req)
	if err != nil {
		c.t.Fatalf("marshal notification: %v", err)
	}
	data = append(data, '\n')
	if _, err := c.writer.Write(data); err != nil {
		c.t.Fatalf("write notification: %v", err)
	}
}

func (c *testClient) initialize() {
	c.t.Helper()
	resp := c.call("initialize", map[string]any{
		"protocolVersion": "2025-11-25",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test-client", "version": "v0.0.1"},
	})
	if resp.Error != nil {
		c.t.Fatalf("initialize failed: code=%d msg=%s", resp.Error.Code, resp.Error.Message)
	}
	c.notify("notifications/initialized", map[string]any{})
}

func (c *testClient) listTools() toolsListResult {
	c.t.Helper()
	resp := c.call("tools/list", map[string]any{})
	if resp.Error != nil {
		c.t.Fatalf("tools/list failed: code=%d msg=%s", resp.Error.Code, resp.Error.Message)
	}
	var result toolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		c.t.Fatalf("unmarshal tools/list result: %v", err)
	}
	return result
}

func (c *testClient) callTool(name string, arguments map[string]any) (callToolResult, *jsonrpcError) {
	c.t.Helper()
	resp := c.call("tools/call", map[string]any{
		"name":      name,
		"arguments": arguments,
	})
	if resp.Error != nil {
		return callToolResult{}, resp.Error
	}
	var result callToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		c.t.Fatalf("unmarshal tools/call result: %v", err)
	}
	return result, nil
}

func newExampleSession(t *testing.T, handler exampleHandler, options ...mcpruntime.RegisterOption) (*testClient, func()) {
	t.Helper()

	server := newServer()
	if err := examplev1.RegisterExampleAPITools(server, handler, options...); err != nil {
		t.Fatalf("RegisterExampleAPITools() failed: %v", err)
	}

	return connectClient(t, server)
}

func newServer() *mcpruntime.Server {
	return mcpruntime.NewServer("protoc-gen-mcp-test-server", "v0.0.1")
}

func connectClient(t *testing.T, server *mcpruntime.Server) (*testClient, func()) {
	t.Helper()

	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		mcpruntime.ServeIO(ctx, server, serverReader, serverWriter)
		serverWriter.Close()
	}()

	scanner := bufio.NewScanner(clientReader)
	scanner.Buffer(make([]byte, 0, 1<<20), 1<<20)

	client := &testClient{
		t:       t,
		scanner: scanner,
		writer:  clientWriter,
		nextID:  1,
	}
	client.initialize()

	return client, func() {
		cancel()
		clientWriter.Close()
	}
}

func TestRegisterExampleAPIToolsHappyPath(t *testing.T) {
	client, cleanup := newExampleSession(t, exampleHandler{})
	defer cleanup()

	tools := client.listTools()

	var toolNames []string
	for _, tool := range tools.Tools {
		toolNames = append(toolNames, tool.Name)
	}
	slices.Sort(toolNames)
	if !slices.Equal(toolNames, []string{"example_CreateReport", "example_DescribeAdvancedShapes", "example_DescribeScalarShapes", "example_Health", "example_HiddenThing"}) {
		t.Fatalf("tool names = %v, want [example_CreateReport example_DescribeAdvancedShapes example_DescribeScalarShapes example_Health example_HiddenThing]", toolNames)
	}

	validateToolInputSchema(t, tools.Tools, "example_CreateReport", map[string]any{
		"city":    "Paris",
		"count":   2,
		"details": map[string]any{"label": "today"},
		"labels":  []string{"primary", "daily"},
		"units":   "metric",
	})
	validateToolInputSchema(t, tools.Tools, "example_CreateReport", map[string]any{
		"city":    "Paris",
		"count":   2,
		"details": map[string]any{"label": "today"},
		"labels":  nil,
		"units":   nil,
	})
	validateToolInputSchema(t, tools.Tools, "example_Health", map[string]any{})
	validateToolInputSchema(t, tools.Tools, "example_DescribeAdvancedShapes", map[string]any{
		"labels":     map[string]any{"env": "prod", "team": "core"},
		"quantities": map[string]any{"1": "one", "2": "two"},
		"toggles":    map[string]any{"true": "enabled", "false": "disabled"},
		"limits":     map[string]any{"18446744073709551615": "max"},
		"observedAt": "2026-03-09T10:11:12Z",
		"ttl":        "3600s",
		"payload":    map[string]any{"kind": "demo", "nested": map[string]any{"ok": true}},
		"items":      []any{"a", 2.0, false, map[string]any{"x": "y"}},
		"dynamic":    map[string]any{"city": "Paris", "score": 7.0},
		"note":       "hello",
		"total":      "42",
		"enabled":    true,
		"ratio":      "NaN",
		"mask":       "labels,observedAt",
		"blob":       "aGVsbG8=",
		"smallTotal": 7,
		"uintTotal":  11,
		"hugeTotal":  "99",
		"weight":     1.5,
		"rawRatio":   "Infinity",
		"tree": map[string]any{
			"name": "root",
			"child": map[string]any{
				"name": "leaf",
			},
			"children": []any{
				map[string]any{"name": "branch"},
			},
		},
		"detailAny": map[string]any{
			"@type": "type.googleapis.com/internal.testproto.example.v1.ReportDetails",
			"label": "from-any",
		},
		"durationAny": map[string]any{
			"@type": "type.googleapis.com/google.protobuf.Duration",
			"value": "3600s",
		},
		"cityAlias": "paris-fr",
	})
	validateToolInputSchema(t, tools.Tools, "example_DescribeAdvancedShapes", map[string]any{
		"labels":      map[string]any{"env": "prod"},
		"observedAt":  nil,
		"ttl":         nil,
		"payload":     nil,
		"items":       nil,
		"note":        nil,
		"total":       nil,
		"enabled":     nil,
		"ratio":       nil,
		"mask":        nil,
		"blob":        nil,
		"smallTotal":  nil,
		"uintTotal":   nil,
		"hugeTotal":   nil,
		"weight":      nil,
		"rawRatio":    nil,
		"tree":        nil,
		"detailAny":   nil,
		"durationAny": nil,
		"cityDetails": nil,
	})
	validateToolInputSchema(t, tools.Tools, "example_DescribeScalarShapes", scalarShapeArguments())
	validateToolInputSchema(t, tools.Tools, "example_DescribeScalarShapes", nullableScalarShapeArguments())

	result, rpcErr := client.callTool("example_CreateReport", map[string]any{
		"city":    "Paris",
		"count":   2,
		"details": map[string]any{"label": "today"},
		"labels":  []string{"primary", "daily"},
		"units":   "metric",
	})
	if rpcErr != nil {
		t.Fatalf("CallTool(CreateReport) failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}
	if result.IsError {
		t.Fatalf("CreateReport returned tool error: %+v", result)
	}
	if len(result.Content) != 1 {
		t.Fatalf("CreateReport content count = %d, want 1", len(result.Content))
	}

	var tc textContent
	if err := json.Unmarshal(result.Content[0], &tc); err != nil {
		t.Fatalf("unmarshal text content: %v", err)
	}

	textJSON := decodeMap(t, []byte(tc.Text))
	structuredJSON := decodeMap(t, result.StructuredContent)
	if !mapsEqual(textJSON, structuredJSON) {
		t.Fatalf("text content and structured content differ:\ntext=%v\nstructured=%v", textJSON, structuredJSON)
	}
	if got := textJSON["totalCount"]; got != "42" {
		t.Fatalf("totalCount = %v, want ProtoJSON string \"42\"", got)
	}
	if got := textJSON["status"]; got != "REPORT_STATUS_OK" {
		t.Fatalf("status = %v, want REPORT_STATUS_OK", got)
	}

	details, ok := textJSON["details"].(map[string]any)
	if !ok {
		t.Fatalf("details has type %T, want map[string]any", textJSON["details"])
	}
	if got := details["label"]; got != "today" {
		t.Fatalf("details.label = %v, want today", got)
	}

	pingResult, rpcErr := client.callTool("example_Health", map[string]any{})
	if rpcErr != nil {
		t.Fatalf("CallTool(Health) failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}
	if pingResult.IsError {
		t.Fatalf("Health returned tool error: %+v", pingResult)
	}

	pingStructured := decodeMap(t, pingResult.StructuredContent)
	ack, ok := pingStructured["ack"].(map[string]any)
	if !ok {
		t.Fatalf("ack has type %T, want map[string]any", pingStructured["ack"])
	}
	if len(ack) != 0 {
		t.Fatalf("ack = %v, want empty object", ack)
	}

	advancedResult, rpcErr := client.callTool("example_DescribeAdvancedShapes", map[string]any{
		"labels":     map[string]any{"env": "prod"},
		"quantities": map[string]any{"1": "one", "2": "two"},
		"toggles":    map[string]any{"true": "enabled"},
		"limits":     map[string]any{"18446744073709551615": "max"},
		"observedAt": "2026-03-09T10:11:12Z",
		"ttl":        "3600s",
		"payload":    map[string]any{"kind": "demo", "nested": map[string]any{"ok": true}},
		"items":      []any{"a", 2.0, false, map[string]any{"x": "y"}},
		"dynamic":    map[string]any{"city": "Paris"},
		"note":       "hello",
		"total":      "42",
		"enabled":    true,
		"ratio":      "NaN",
		"mask":       "labels,observedAt",
		"blob":       "aGVsbG8=",
		"smallTotal": 7,
		"uintTotal":  11,
		"hugeTotal":  "99",
		"weight":     1.5,
		"rawRatio":   "Infinity",
		"tree": map[string]any{
			"name": "root",
			"child": map[string]any{
				"name": "leaf",
			},
			"children": []any{
				map[string]any{"name": "branch"},
			},
		},
		"detailAny": map[string]any{
			"@type": "type.googleapis.com/internal.testproto.example.v1.ReportDetails",
			"label": "from-any",
		},
		"durationAny": map[string]any{
			"@type": "type.googleapis.com/google.protobuf.Duration",
			"value": "3600s",
		},
		"cityAlias": "paris-fr",
	})
	if rpcErr != nil {
		t.Fatalf("CallTool(DescribeAdvancedShapes) failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}
	if advancedResult.IsError {
		t.Fatalf("DescribeAdvancedShapes returned tool error: %+v", advancedResult)
	}

	advancedStructured := decodeMap(t, advancedResult.StructuredContent)
	if got := advancedStructured["observedAt"]; got != "2026-03-09T10:11:12Z" {
		t.Fatalf("observedAt = %v, want 2026-03-09T10:11:12Z", got)
	}
	if got := advancedStructured["ttl"]; got != "3600s" {
		t.Fatalf("ttl = %v, want 3600s", got)
	}
	if got := advancedStructured["total"]; got != "42" {
		t.Fatalf("total = %v, want ProtoJSON string \"42\"", got)
	}
	if got := advancedStructured["ratio"]; got != "NaN" {
		t.Fatalf("ratio = %v, want NaN", got)
	}
	if got := advancedStructured["mask"]; got != "labels,observedAt" {
		t.Fatalf("mask = %v, want labels,observedAt", got)
	}
	if got := advancedStructured["blob"]; got != "aGVsbG8=" {
		t.Fatalf("blob = %v, want aGVsbG8=", got)
	}
	if got := advancedStructured["smallTotal"]; got != float64(7) {
		t.Fatalf("smallTotal = %v, want 7", got)
	}
	if got := advancedStructured["uintTotal"]; got != float64(11) {
		t.Fatalf("uintTotal = %v, want 11", got)
	}
	if got := advancedStructured["hugeTotal"]; got != "99" {
		t.Fatalf("hugeTotal = %v, want ProtoJSON string \"99\"", got)
	}
	if got := advancedStructured["weight"]; got != float64(1.5) {
		t.Fatalf("weight = %v, want 1.5", got)
	}
	if got := advancedStructured["rawRatio"]; got != "Infinity" {
		t.Fatalf("rawRatio = %v, want Infinity", got)
	}
	tree, ok := advancedStructured["tree"].(map[string]any)
	if !ok {
		t.Fatalf("tree has type %T, want map[string]any", advancedStructured["tree"])
	}
	if got := tree["name"]; got != "root" {
		t.Fatalf("tree.name = %v, want root", got)
	}
	treeChild, ok := tree["child"].(map[string]any)
	if !ok {
		t.Fatalf("tree.child has type %T, want map[string]any", tree["child"])
	}
	if got := treeChild["name"]; got != "leaf" {
		t.Fatalf("tree.child.name = %v, want leaf", got)
	}
	detailAny, ok := advancedStructured["detailAny"].(map[string]any)
	if !ok {
		t.Fatalf("detailAny has type %T, want map[string]any", advancedStructured["detailAny"])
	}
	if got := detailAny["@type"]; got != "type.googleapis.com/internal.testproto.example.v1.ReportDetails" {
		t.Fatalf("detailAny.@type = %v, want report details type URL", got)
	}
	if got := detailAny["label"]; got != "from-any" {
		t.Fatalf("detailAny.label = %v, want from-any", got)
	}
	durationAny, ok := advancedStructured["durationAny"].(map[string]any)
	if !ok {
		t.Fatalf("durationAny has type %T, want map[string]any", advancedStructured["durationAny"])
	}
	if got := durationAny["@type"]; got != "type.googleapis.com/google.protobuf.Duration" {
		t.Fatalf("durationAny.@type = %v, want duration type URL", got)
	}
	if got := durationAny["value"]; got != "3600s" {
		t.Fatalf("durationAny.value = %v, want 3600s", got)
	}
	if got := advancedStructured["cityAlias"]; got != "paris-fr" {
		t.Fatalf("cityAlias = %v, want paris-fr", got)
	}

	quantities, ok := advancedStructured["quantities"].(map[string]any)
	if !ok {
		t.Fatalf("quantities has type %T, want map[string]any", advancedStructured["quantities"])
	}
	if got := quantities["2"]; got != "two" {
		t.Fatalf("quantities[2] = %v, want two", got)
	}
	limits, ok := advancedStructured["limits"].(map[string]any)
	if !ok {
		t.Fatalf("limits has type %T, want map[string]any", advancedStructured["limits"])
	}
	if got := limits["18446744073709551615"]; got != "max" {
		t.Fatalf("limits[max] = %v, want max", got)
	}

	scalarResult, rpcErr := client.callTool("example_DescribeScalarShapes", scalarShapeArguments())
	if rpcErr != nil {
		t.Fatalf("CallTool(DescribeScalarShapes) failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}
	if scalarResult.IsError {
		t.Fatalf("DescribeScalarShapes returned tool error: %+v", scalarResult)
	}

	scalarStructured := decodeMap(t, scalarResult.StructuredContent)
	if got := scalarStructured["boolFlag"]; got != true {
		t.Fatalf("boolFlag = %v, want true", got)
	}
	if got := scalarStructured["textValue"]; got != "scalar-demo" {
		t.Fatalf("textValue = %v, want scalar-demo", got)
	}
	if got := scalarStructured["bytesValue"]; got != "aGVsbG8=" {
		t.Fatalf("bytesValue = %v, want aGVsbG8=", got)
	}
	if got := scalarStructured["int32Value"]; got != float64(-123) {
		t.Fatalf("int32Value = %v, want -123", got)
	}
	if got := scalarStructured["uint32Value"]; got != float64(123) {
		t.Fatalf("uint32Value = %v, want 123", got)
	}
	if got := scalarStructured["int64Value"]; got != "-4567890123" {
		t.Fatalf("int64Value = %v, want -4567890123", got)
	}
	if got := scalarStructured["uint64Value"]; got != "4567890123" {
		t.Fatalf("uint64Value = %v, want 4567890123", got)
	}
	if got := scalarStructured["floatValue"]; got != float64(1.25) {
		t.Fatalf("floatValue = %v, want 1.25", got)
	}
	if got := scalarStructured["doubleValue"]; got != 2.5 {
		t.Fatalf("doubleValue = %v, want 2.5", got)
	}
	if got := scalarStructured["status"]; got != "REPORT_STATUS_OK" {
		t.Fatalf("status = %v, want REPORT_STATUS_OK", got)
	}
	if got := scalarStructured["optionalInt64Value"]; got != "12" {
		t.Fatalf("optionalInt64Value = %v, want 12", got)
	}
	if got := scalarStructured["optionalUint64Value"]; got != "15" {
		t.Fatalf("optionalUint64Value = %v, want 15", got)
	}
	if got := scalarStructured["optionalStatus"]; got != "REPORT_STATUS_FAILED" {
		t.Fatalf("optionalStatus = %v, want REPORT_STATUS_FAILED", got)
	}
}

func TestRegisterExampleAPIToolsNullableInput(t *testing.T) {
	client, cleanup := newExampleSession(t, exampleHandler{})
	defer cleanup()

	if _, rpcErr := client.callTool("example_CreateReport", map[string]any{
		"city":    "Paris",
		"count":   2,
		"details": map[string]any{"label": "today"},
		"units":   nil,
		"labels":  nil,
	}); rpcErr != nil {
		t.Fatalf("CallTool(CreateReport) with null optional fields failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}

	if _, rpcErr := client.callTool("example_DescribeAdvancedShapes", map[string]any{
		"labels":      map[string]any{"env": "prod"},
		"observedAt":  nil,
		"ttl":         nil,
		"payload":     nil,
		"items":       nil,
		"dynamic":     nil,
		"note":        nil,
		"total":       nil,
		"enabled":     nil,
		"ratio":       nil,
		"mask":        nil,
		"blob":        nil,
		"smallTotal":  nil,
		"uintTotal":   nil,
		"hugeTotal":   nil,
		"weight":      nil,
		"rawRatio":    nil,
		"tree":        nil,
		"detailAny":   nil,
		"durationAny": nil,
		"cityDetails": nil,
	}); rpcErr != nil {
		t.Fatalf("CallTool(DescribeAdvancedShapes) with null optional fields failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}

	if _, rpcErr := client.callTool("example_DescribeScalarShapes", nullableScalarShapeArguments()); rpcErr != nil {
		t.Fatalf("CallTool(DescribeScalarShapes) with null optional fields failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}
}

func TestRegisterExampleAPIToolsInvalidInput(t *testing.T) {
	client, cleanup := newExampleSession(t, exampleHandler{})
	defer cleanup()

	testCases := []struct {
		name      string
		toolName  string
		arguments map[string]any
	}{
		{
			name:     "missing required field",
			toolName: "example_CreateReport",
			arguments: map[string]any{
				"city":  "Paris",
				"count": 2,
			},
		},
		{
			name:     "unknown field",
			toolName: "example_CreateReport",
			arguments: map[string]any{
				"city":    "Paris",
				"count":   2,
				"details": map[string]any{"label": "today"},
				"extra":   true,
			},
		},
		{
			name:     "wrong scalar type",
			toolName: "example_CreateReport",
			arguments: map[string]any{
				"city":    "Paris",
				"count":   "two",
				"details": map[string]any{"label": "today"},
			},
		},
		{
			name:     "invalid map key",
			toolName: "example_DescribeAdvancedShapes",
			arguments: map[string]any{
				"labels":     map[string]any{"env": "prod"},
				"quantities": map[string]any{"oops": "bad"},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, rpcErr := client.callTool(testCase.toolName, testCase.arguments)
			assertJSONRPCErrorCode(t, rpcErr, mcpruntime.CodeInvalidParams)
		})
	}
}

func TestRegisterExampleAPIToolsOutputValidationFailure(t *testing.T) {
	client, cleanup := newExampleSession(t, exampleHandler{
		createReport: func(context.Context, *examplev1.CreateReportRequest) (*examplev1.CreateReportResponse, error) {
			return &examplev1.CreateReportResponse{
				ReportId:   "report-1",
				TotalCount: 42,
				Status:     examplev1.ReportStatus_REPORT_STATUS_OK,
			}, nil
		},
	})
	defer cleanup()

	_, rpcErr := client.callTool("example_CreateReport", map[string]any{
		"city":    "Paris",
		"count":   2,
		"details": map[string]any{"label": "today"},
	})
	if rpcErr == nil {
		t.Fatal("CallTool(CreateReport) unexpectedly succeeded")
	}
	if !strings.Contains(rpcErr.Message, "validate output") {
		t.Fatalf("output validation error = %v, want validate output message", rpcErr.Message)
	}
}

func TestRegisterExampleAPIToolsNamespaceOverride(t *testing.T) {
	server := newServer()
	handler := exampleHandler{}

	if err := examplev1.RegisterExampleAPITools(server, handler); err != nil {
		t.Fatalf("RegisterExampleAPITools(default) failed: %v", err)
	}
	if err := examplev1.RegisterExampleAPITools(server, handler, mcpruntime.WithNamespace("custom.v1")); err != nil {
		t.Fatalf("RegisterExampleAPITools(custom.v1) failed: %v", err)
	}

	client, cleanup := connectClient(t, server)
	defer cleanup()

	tools := client.listTools()

	var toolNames []string
	for _, tool := range tools.Tools {
		toolNames = append(toolNames, tool.Name)
	}

	want := []string{
		"custom_v1_CreateReport",
		"custom_v1_DescribeAdvancedShapes",
		"custom_v1_DescribeScalarShapes",
		"custom_v1_Health",
		"custom_v1_HiddenThing",
		"example_CreateReport",
		"example_DescribeAdvancedShapes",
		"example_DescribeScalarShapes",
		"example_Health",
		"example_HiddenThing",
	}
	slices.Sort(toolNames)
	if !slices.Equal(toolNames, want) {
		t.Fatalf("tool names = %v, want %v", toolNames, want)
	}
}

func TestRegisterExampleAPIToolsDuplicateNameFails(t *testing.T) {
	server := newServer()
	handler := exampleHandler{}

	if err := examplev1.RegisterExampleAPITools(server, handler); err != nil {
		t.Fatalf("first RegisterExampleAPITools() failed: %v", err)
	}

	err := examplev1.RegisterExampleAPITools(server, handler)
	if err == nil {
		t.Fatal("second RegisterExampleAPITools() unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("duplicate registration error = %v, want already registered", err)
	}
}

type exampleHandler struct {
	createReport    func(context.Context, *examplev1.CreateReportRequest) (*examplev1.CreateReportResponse, error)
	describe        func(context.Context, *examplev1.DescribeAdvancedShapesRequest) (*examplev1.DescribeAdvancedShapesResponse, error)
	describeScalars func(context.Context, *examplev1.DescribeScalarShapesRequest) (*examplev1.DescribeScalarShapesResponse, error)
	ping            func(context.Context, *examplev1.PingRequest) (*examplev1.PingResponse, error)
}

func (handler exampleHandler) CreateReport(
	ctx context.Context,
	req *examplev1.CreateReportRequest,
) (*examplev1.CreateReportResponse, error) {
	if handler.createReport != nil {
		return handler.createReport(ctx, req)
	}

	return &examplev1.CreateReportResponse{
		ReportId:   "report-1",
		TotalCount: 42,
		Status:     examplev1.ReportStatus_REPORT_STATUS_OK,
		Details:    &examplev1.ReportDetails{Label: req.GetDetails().GetLabel()},
		Warnings:   []string{"none"},
	}, nil
}

func (handler exampleHandler) DescribeAdvancedShapes(
	ctx context.Context,
	req *examplev1.DescribeAdvancedShapesRequest,
) (*examplev1.DescribeAdvancedShapesResponse, error) {
	if handler.describe != nil {
		return handler.describe(ctx, req)
	}

	resp := &examplev1.DescribeAdvancedShapesResponse{
		Labels:      req.GetLabels(),
		Quantities:  req.GetQuantities(),
		Toggles:     req.GetToggles(),
		Limits:      req.GetLimits(),
		ObservedAt:  req.GetObservedAt(),
		Ttl:         req.GetTtl(),
		Payload:     req.GetPayload(),
		Items:       req.GetItems(),
		Dynamic:     req.GetDynamic(),
		Note:        req.GetNote(),
		Total:       req.GetTotal(),
		Enabled:     req.GetEnabled(),
		Ratio:       req.GetRatio(),
		Mask:        req.GetMask(),
		Blob:        req.GetBlob(),
		SmallTotal:  req.GetSmallTotal(),
		UintTotal:   req.GetUintTotal(),
		HugeTotal:   req.GetHugeTotal(),
		Weight:      req.GetWeight(),
		RawRatio:    req.RawRatio,
		Tree:        req.GetTree(),
		DetailAny:   req.GetDetailAny(),
		DurationAny: req.GetDurationAny(),
	}

	switch selector := req.Selector.(type) {
	case *examplev1.DescribeAdvancedShapesRequest_CityAlias:
		resp.Selector = &examplev1.DescribeAdvancedShapesResponse_CityAlias{CityAlias: selector.CityAlias}
	case *examplev1.DescribeAdvancedShapesRequest_CityId:
		resp.Selector = &examplev1.DescribeAdvancedShapesResponse_CityId{CityId: selector.CityId}
	case *examplev1.DescribeAdvancedShapesRequest_CityDetails:
		resp.Selector = &examplev1.DescribeAdvancedShapesResponse_CityDetails{CityDetails: selector.CityDetails}
	}

	return resp, nil
}

func (handler exampleHandler) DescribeScalarShapes(
	ctx context.Context,
	req *examplev1.DescribeScalarShapesRequest,
) (*examplev1.DescribeScalarShapesResponse, error) {
	if handler.describeScalars != nil {
		return handler.describeScalars(ctx, req)
	}

	return &examplev1.DescribeScalarShapesResponse{
		BoolFlag:              req.GetBoolFlag(),
		TextValue:             req.GetTextValue(),
		BytesValue:            req.GetBytesValue(),
		Int32Value:            req.GetInt32Value(),
		Sint32Value:           req.GetSint32Value(),
		Sfixed32Value:         req.GetSfixed32Value(),
		Uint32Value:           req.GetUint32Value(),
		Fixed32Value:          req.GetFixed32Value(),
		Int64Value:            req.GetInt64Value(),
		Sint64Value:           req.GetSint64Value(),
		Sfixed64Value:         req.GetSfixed64Value(),
		Uint64Value:           req.GetUint64Value(),
		Fixed64Value:          req.GetFixed64Value(),
		FloatValue:            req.GetFloatValue(),
		DoubleValue:           req.GetDoubleValue(),
		Status:                req.GetStatus(),
		Details:               req.GetDetails(),
		Samples:               req.GetSamples(),
		OptionalBoolFlag:      req.OptionalBoolFlag,
		OptionalTextValue:     req.OptionalTextValue,
		OptionalBytesValue:    req.OptionalBytesValue,
		OptionalInt32Value:    req.OptionalInt32Value,
		OptionalSint32Value:   req.OptionalSint32Value,
		OptionalSfixed32Value: req.OptionalSfixed32Value,
		OptionalUint32Value:   req.OptionalUint32Value,
		OptionalFixed32Value:  req.OptionalFixed32Value,
		OptionalInt64Value:    req.OptionalInt64Value,
		OptionalSint64Value:   req.OptionalSint64Value,
		OptionalSfixed64Value: req.OptionalSfixed64Value,
		OptionalUint64Value:   req.OptionalUint64Value,
		OptionalFixed64Value:  req.OptionalFixed64Value,
		OptionalFloatValue:    req.OptionalFloatValue,
		OptionalDoubleValue:   req.OptionalDoubleValue,
		OptionalStatus:        req.OptionalStatus,
	}, nil
}

func (handler exampleHandler) Ping(
	ctx context.Context,
	req *examplev1.PingRequest,
) (*examplev1.PingResponse, error) {
	if handler.ping != nil {
		return handler.ping(ctx, req)
	}

	return &examplev1.PingResponse{
		Ack: &emptypb.Empty{},
	}, nil
}

func (handler exampleHandler) HiddenThing(
	_ context.Context,
	_ *examplev1.HiddenThingRequest,
) (*examplev1.HiddenThingResponse, error) {
	return &examplev1.HiddenThingResponse{}, nil
}

func assertJSONRPCErrorCode(t *testing.T, rpcErr *jsonrpcError, want int) {
	t.Helper()

	if rpcErr == nil {
		t.Fatal("got nil error, want non-nil")
	}
	if rpcErr.Code != want {
		t.Fatalf("jsonrpc error code = %d, want %d", rpcErr.Code, want)
	}
}

func decodeMap(t *testing.T, raw []byte) map[string]any {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal JSON map: %v", err)
	}

	return decoded
}

func decodeAnyMap(t *testing.T, value any) map[string]any {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal arbitrary JSON value: %v", err)
	}

	return decodeMap(t, raw)
}

func scalarShapeArguments() map[string]any {
	return map[string]any{
		"boolFlag":              true,
		"textValue":             "scalar-demo",
		"bytesValue":            "aGVsbG8=",
		"int32Value":            -123,
		"sint32Value":           -321,
		"sfixed32Value":         -456,
		"uint32Value":           123,
		"fixed32Value":          456,
		"int64Value":            "-4567890123",
		"sint64Value":           "-77",
		"sfixed64Value":         "-88",
		"uint64Value":           "4567890123",
		"fixed64Value":          "42",
		"floatValue":            1.25,
		"doubleValue":           2.5,
		"status":                "REPORT_STATUS_OK",
		"details":               map[string]any{"label": "plain"},
		"samples":               []any{1, 2, 3},
		"optionalBoolFlag":      true,
		"optionalTextValue":     "optional-demo",
		"optionalBytesValue":    "d29ybGQ=",
		"optionalInt32Value":    7,
		"optionalSint32Value":   -8,
		"optionalSfixed32Value": -9,
		"optionalUint32Value":   10,
		"optionalFixed32Value":  11,
		"optionalInt64Value":    "12",
		"optionalSint64Value":   "-13",
		"optionalSfixed64Value": "-14",
		"optionalUint64Value":   "15",
		"optionalFixed64Value":  "16",
		"optionalFloatValue":    3.5,
		"optionalDoubleValue":   4.5,
		"optionalStatus":        "REPORT_STATUS_FAILED",
	}
}

func nullableScalarShapeArguments() map[string]any {
	return map[string]any{
		"boolFlag":              true,
		"textValue":             "scalar-demo",
		"bytesValue":            "aGVsbG8=",
		"int32Value":            -123,
		"sint32Value":           -321,
		"sfixed32Value":         -456,
		"uint32Value":           123,
		"fixed32Value":          456,
		"int64Value":            "-4567890123",
		"sint64Value":           "-77",
		"sfixed64Value":         "-88",
		"uint64Value":           "4567890123",
		"fixed64Value":          "42",
		"floatValue":            1.25,
		"doubleValue":           2.5,
		"status":                "REPORT_STATUS_OK",
		"details":               map[string]any{"label": "plain"},
		"samples":               []any{1, 2, 3},
		"optionalBoolFlag":      nil,
		"optionalTextValue":     nil,
		"optionalBytesValue":    nil,
		"optionalInt32Value":    nil,
		"optionalSint32Value":   nil,
		"optionalSfixed32Value": nil,
		"optionalUint32Value":   nil,
		"optionalFixed32Value":  nil,
		"optionalInt64Value":    nil,
		"optionalSint64Value":   nil,
		"optionalSfixed64Value": nil,
		"optionalUint64Value":   nil,
		"optionalFixed64Value":  nil,
		"optionalFloatValue":    nil,
		"optionalDoubleValue":   nil,
		"optionalStatus":        nil,
	}
}

func mapsEqual(left map[string]any, right map[string]any) bool {
	leftRaw, leftErr := json.Marshal(left)
	rightRaw, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}

	return string(leftRaw) == string(rightRaw)
}

func validateToolInputSchema(t *testing.T, tools []toolInfo, toolName string, arguments map[string]any) {
	t.Helper()

	tool := findTool(t, tools, toolName)
	if tool.InputSchema == nil {
		t.Fatalf("tool %q input schema is nil", toolName)
	}

	rawSchema, err := json.Marshal(tool.InputSchema)
	if err != nil {
		t.Fatalf("marshal input schema for tool %q: %v", toolName, err)
	}

	var schema jsonschema.Schema
	if err := json.Unmarshal(rawSchema, &schema); err != nil {
		t.Fatalf("unmarshal input schema for tool %q: %v", toolName, err)
	}

	resolved, err := schema.Resolve(&jsonschema.ResolveOptions{
		ValidateDefaults: true,
	})
	if err != nil {
		t.Fatalf("resolve input schema for tool %q: %v", toolName, err)
	}
	if err := resolved.Validate(arguments); err != nil {
		t.Fatalf("input schema for tool %q rejected valid arguments %v: %v", toolName, arguments, err)
	}
}

func findTool(t *testing.T, tools []toolInfo, toolName string) toolInfo {
	t.Helper()

	for _, tool := range tools {
		if tool.Name == toolName {
			return tool
		}
	}

	t.Fatalf("tool %q not found in tools/list", toolName)
	return toolInfo{}
}

// Ensure unused imports are referenced.
var _ = errors.New
