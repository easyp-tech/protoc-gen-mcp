package examplemcp_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/easyp-tech/protoc-gen-mcp/internal/examplemcp"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "protoc-gen-mcp-python-invalid-output-test-client",
		Version: "v0.0.1",
	}, nil)

	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("client.Connect() over stdio failed: %v", err)
	}
	defer session.Close()

	_, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "example_CreateReport",
		Arguments: map[string]any{
			"city":    "Paris",
			"count":   2,
			"details": map[string]any{"label": "today"},
		},
	})
	if err == nil {
		t.Fatal("CallTool(CreateReport) unexpectedly succeeded with invalid output schema")
	}
	if !strings.Contains(err.Error(), "mcpruntime: validate output for tool") || !strings.Contains(err.Error(), "example_CreateReport") {
		t.Fatalf("CallTool(CreateReport) error = %v, want output validation failure", err)
	}
}

func runServerOverStdioContract(t *testing.T, cmd *exec.Cmd) {
	t.Helper()

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "protoc-gen-mcp-stdio-test-client",
		Version: "v0.0.1",
	}, nil)

	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("client.Connect() over stdio failed: %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools() over stdio failed: %v", err)
	}

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

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "example_CreateReport",
		Arguments: map[string]any{
			"city":    "Paris",
			"count":   2,
			"details": map[string]any{"label": "today"},
		},
	})
	if err != nil {
		t.Fatalf("CallTool(CreateReport) over stdio failed: %v", err)
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

	advancedResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "example_DescribeAdvancedShapes",
		Arguments: map[string]any{
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
		},
	})
	if err != nil {
		t.Fatalf("CallTool(DescribeAdvancedShapes) over stdio failed: %v", err)
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

	scalarResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "example_DescribeScalarShapes",
		Arguments: scalarShapeArguments(),
	})
	if err != nil {
		t.Fatalf("CallTool(DescribeScalarShapes) over stdio failed: %v", err)
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

	python := pythonCommand(t)
	probe := exec.Command(python, "-c", "import anyio, google.protobuf, jsonschema, mcp")
	probe.Dir = root
	if output, err := probe.CombinedOutput(); err != nil {
		t.Fatalf("python runtime dependencies are not available: %v\n%s", err, output)
	}

	cmd := exec.Command(python, filepath.Join(root, "cmd/example-python-mcp-server/main.py"))
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"PYTHONPATH="+root,
		"PYTHONUNBUFFERED=1",
	)
	return cmd
}

func pythonCommand(t *testing.T) string {
	t.Helper()

	if path, err := exec.LookPath("python3"); err == nil {
		return path
	}
	if path, err := exec.LookPath("python"); err == nil {
		return path
	}

	t.Fatal("python3/python not found in PATH")
	return ""
}

func decodeMap(t *testing.T, value any) map[string]any {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON value: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal JSON value: %v", err)
	}

	return decoded
}

func validateToolInputSchema(t *testing.T, tools []*mcp.Tool, toolName string, arguments map[string]any) {
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

func assertTextStructuredContentMatch(t *testing.T, toolName string, result *mcp.CallToolResult) {
	t.Helper()

	if len(result.Content) != 1 {
		t.Fatalf("%s returned %d content items, want 1", toolName, len(result.Content))
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("%s content[0] has type %T, want *mcp.TextContent", toolName, result.Content[0])
	}

	var fromText map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &fromText); err != nil {
		t.Fatalf("decode text content for %s: %v", toolName, err)
	}

	fromStructured := decodeMap(t, result.StructuredContent)
	if !reflect.DeepEqual(fromText, fromStructured) {
		t.Fatalf("%s text content %v does not match structured content %v", toolName, fromText, fromStructured)
	}
}

func findTool(t *testing.T, tools []*mcp.Tool, toolName string) *mcp.Tool {
	t.Helper()

	for _, tool := range tools {
		if tool != nil && tool.Name == toolName {
			return tool
		}
	}

	t.Fatalf("tool %q not found in tools/list", toolName)
	return nil
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
