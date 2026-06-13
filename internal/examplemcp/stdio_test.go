package examplemcp_test

import (
	"bufio"
	"encoding/json"
	"io"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/easyp-tech/protoc-gen-mcp/internal/examplemcp"
	"github.com/easyp-tech/protoc-gen-mcp/internal/pythontest"
	"github.com/google/jsonschema-go/jsonschema"
)

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

// stdioClient communicates with a subprocess MCP server over stdin/stdout JSON-RPC.
type stdioClient struct {
	t       *testing.T
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	nextID  int
}

func newStdioClient(t *testing.T, cmd *exec.Cmd) *stdioClient {
	t.Helper()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start command: %v", err)
	}

	t.Cleanup(func() {
		stdin.Close()
		cmd.Process.Kill()
		cmd.Wait()
	})

	return &stdioClient{
		t:       t,
		stdin:   stdin,
		scanner: bufio.NewScanner(stdout),
		nextID:  1,
	}
}

func (c *stdioClient) call(method string, params any) jsonrpcResponse {
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
	if _, err := c.stdin.Write(data); err != nil {
		c.t.Fatalf("write request: %v", err)
	}

	if !c.scanner.Scan() {
		c.t.Fatalf("no response for %s (scanner error: %v)", method, c.scanner.Err())
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(c.scanner.Bytes(), &resp); err != nil {
		c.t.Fatalf("unmarshal response for %s: %v (raw: %s)", method, err, c.scanner.Bytes())
	}
	return resp
}

func (c *stdioClient) notify(method string, params any) {
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
	if _, err := c.stdin.Write(data); err != nil {
		c.t.Fatalf("write notification: %v", err)
	}
}

func (c *stdioClient) initialize() {
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

type toolsListResult struct {
	Tools []toolInfo `json:"tools"`
}

type toolInfo struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"inputSchema,omitempty"`
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

func (c *stdioClient) listTools() toolsListResult {
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

func (c *stdioClient) callTool(name string, arguments map[string]any) (callToolResult, *jsonrpcError) {
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

func TestServerOverStdio(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Fatalf("go not found in PATH: %v", err)
	}

	root := repoRoot(t)
	cmd := exec.Command("go", "run", filepath.Join(root, "cmd/example-mcp-server"))
	cmd.Dir = root
	runServerOverStdioContract(t, cmd)
}

func TestPythonServerOverStdio(t *testing.T) {
	root := repoRoot(t)
	runServerOverStdioContract(t, pythonExampleServerCommand(t, root))
}

func TestPythonServerRejectsInvalidOutputOverStdio(t *testing.T) {
	root := repoRoot(t)
	cmd := pythonExampleServerCommand(t, root)
	cmd.Env = append(cmd.Env, "PROTOC_GEN_MCP_PYTHON_INVALID_OUTPUT=create_report")

	client := newStdioClient(t, cmd)
	client.initialize()

	_, rpcErr := client.callTool("example_CreateReport", map[string]any{
		"city":    "Paris",
		"count":   2,
		"details": map[string]any{"label": "today"},
	})
	if rpcErr == nil {
		t.Fatal("CallTool(CreateReport) unexpectedly succeeded with invalid output schema")
	}
	if !strings.Contains(rpcErr.Message, "mcpruntime: validate output for tool") || !strings.Contains(rpcErr.Message, "example_CreateReport") {
		t.Fatalf("CallTool(CreateReport) error = %v, want output validation failure", rpcErr.Message)
	}
}

func runServerOverStdioContract(t *testing.T, cmd *exec.Cmd) {
	t.Helper()

	client := newStdioClient(t, cmd)
	client.initialize()

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
	})
	validateToolInputSchema(t, tools.Tools, "example_CreateReport", map[string]any{
		"city":    "Paris",
		"count":   2,
		"details": map[string]any{"label": "today"},
		"units":   nil,
		"labels":  nil,
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
		"cityAlias":   nil,
	})
	validateToolInputSchema(t, tools.Tools, "example_DescribeScalarShapes", scalarShapeArguments())
	validateToolInputSchema(t, tools.Tools, "example_DescribeScalarShapes", nullableScalarShapeArguments())

	result, rpcErr := client.callTool("example_CreateReport", map[string]any{
		"city":    "Paris",
		"count":   2,
		"details": map[string]any{"label": "today"},
	})
	if rpcErr != nil {
		t.Fatalf("CallTool(CreateReport) over stdio failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}
	if result.IsError {
		t.Fatalf("CreateReport returned tool error over stdio: %+v", result)
	}

	assertTextStructuredContentMatch(t, "example_CreateReport", result)
	structured := decodeMap(t, result.StructuredContent)
	if got := structured["totalCount"]; got != "42" {
		t.Fatalf("totalCount = %v, want ProtoJSON string \"42\"", got)
	}
	if got := structured["status"]; got != "REPORT_STATUS_OK" {
		t.Fatalf("status = %v, want REPORT_STATUS_OK", got)
	}

	advancedResult, rpcErr := client.callTool("example_DescribeAdvancedShapes", map[string]any{
		"labels":     map[string]any{"env": "prod"},
		"quantities": map[string]any{"1": "one"},
		"limits":     map[string]any{"18446744073709551615": "max"},
		"observedAt": "2026-03-09T10:11:12Z",
		"ratio":      "NaN",
		"blob":       "aGVsbG8=",
		"weight":     1.5,
		"rawRatio":   "-Infinity",
		"tree": map[string]any{
			"name": "root",
			"child": map[string]any{
				"name": "leaf",
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
		t.Fatalf("CallTool(DescribeAdvancedShapes) over stdio failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}
	if advancedResult.IsError {
		t.Fatalf("DescribeAdvancedShapes returned tool error over stdio: %+v", advancedResult)
	}

	assertTextStructuredContentMatch(t, "example_DescribeAdvancedShapes", advancedResult)
	advancedStructured := decodeMap(t, advancedResult.StructuredContent)
	if got := advancedStructured["ratio"]; got != "NaN" {
		t.Fatalf("ratio = %v, want NaN", got)
	}
	if got := advancedStructured["rawRatio"]; got != "-Infinity" {
		t.Fatalf("rawRatio = %v, want -Infinity", got)
	}
	if got := advancedStructured["blob"]; got != "aGVsbG8=" {
		t.Fatalf("blob = %v, want aGVsbG8=", got)
	}
	tree, ok := advancedStructured["tree"].(map[string]any)
	if !ok {
		t.Fatalf("tree has type %T, want map[string]any", advancedStructured["tree"])
	}
	if got := tree["name"]; got != "root" {
		t.Fatalf("tree.name = %v, want root", got)
	}
	detailAny, ok := advancedStructured["detailAny"].(map[string]any)
	if !ok {
		t.Fatalf("detailAny has type %T, want map[string]any", advancedStructured["detailAny"])
	}
	if got := detailAny["@type"]; got != "type.googleapis.com/internal.testproto.example.v1.ReportDetails" {
		t.Fatalf("detailAny.@type = %v, want report details type URL", got)
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

	scalarResult, rpcErr := client.callTool("example_DescribeScalarShapes", scalarShapeArguments())
	if rpcErr != nil {
		t.Fatalf("CallTool(DescribeScalarShapes) over stdio failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}
	if scalarResult.IsError {
		t.Fatalf("DescribeScalarShapes returned tool error over stdio: %+v", scalarResult)
	}

	assertTextStructuredContentMatch(t, "example_DescribeScalarShapes", scalarResult)
	scalarStructured := decodeMap(t, scalarResult.StructuredContent)
	if got := scalarStructured["int64Value"]; got != "-4567890123" {
		t.Fatalf("int64Value = %v, want -4567890123", got)
	}
	if got := scalarStructured["optionalUint64Value"]; got != "15" {
		t.Fatalf("optionalUint64Value = %v, want 15", got)
	}
	if got := scalarStructured["optionalStatus"]; got != "REPORT_STATUS_FAILED" {
		t.Fatalf("optionalStatus = %v, want REPORT_STATUS_FAILED", got)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file path")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func pythonExampleServerCommand(t *testing.T, root string) *exec.Cmd {
	t.Helper()

	cmd := exec.Command(pythontest.Python(t), filepath.Join(root, "cmd/example-python-mcp-server/main.py"))
	cmd.Dir = root
	cmd.Env = pythontest.Env(t,
		"PYTHONPATH="+root,
		"PYTHONUNBUFFERED=1",
	)
	return cmd
}

func decodeMap(t *testing.T, value any) map[string]any {
	t.Helper()

	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case json.RawMessage:
		raw = []byte(v)
	default:
		var err error
		raw, err = json.Marshal(value)
		if err != nil {
			t.Fatalf("marshal JSON value: %v", err)
		}
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal JSON value: %v", err)
	}

	return decoded
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

func assertTextStructuredContentMatch(t *testing.T, toolName string, result callToolResult) {
	t.Helper()

	if len(result.Content) != 1 {
		t.Fatalf("%s returned %d content items, want 1", toolName, len(result.Content))
	}

	var tc textContent
	if err := json.Unmarshal(result.Content[0], &tc); err != nil {
		t.Fatalf("unmarshal text content for %s: %v", toolName, err)
	}

	var fromText map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &fromText); err != nil {
		t.Fatalf("decode text content for %s: %v", toolName, err)
	}

	fromStructured := decodeMap(t, result.StructuredContent)
	if !reflect.DeepEqual(fromText, fromStructured) {
		t.Fatalf("%s text content %v does not match structured content %v", toolName, fromText, fromStructured)
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

var _ = examplemcp.NewServer

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
