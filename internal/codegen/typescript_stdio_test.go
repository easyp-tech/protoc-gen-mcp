package codegen

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"google.golang.org/protobuf/compiler/protogen"
)

type tsJsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type tsJsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *tsJsonrpcError `json:"error,omitempty"`
}

type tsJsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type tsStdioClient struct {
	t       *testing.T
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	nextID  int
}

func newTSStdioClient(t *testing.T, cmd *exec.Cmd) *tsStdioClient {
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

	return &tsStdioClient{
		t:       t,
		stdin:   stdin,
		scanner: bufio.NewScanner(stdout),
		nextID:  1,
	}
}

func (c *tsStdioClient) call(method string, params any) tsJsonrpcResponse {
	c.t.Helper()

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		c.t.Fatalf("marshal params: %v", err)
	}

	id := c.nextID
	c.nextID++
	idBytes, _ := json.Marshal(id)

	req := tsJsonrpcRequest{
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

	var resp tsJsonrpcResponse
	if err := json.Unmarshal(c.scanner.Bytes(), &resp); err != nil {
		c.t.Fatalf("unmarshal response for %s: %v (raw: %s)", method, err, c.scanner.Bytes())
	}
	return resp
}

func (c *tsStdioClient) notify(method string, params any) {
	c.t.Helper()

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		c.t.Fatalf("marshal params: %v", err)
	}

	req := tsJsonrpcRequest{
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

func (c *tsStdioClient) initialize() {
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

type tsToolsListResult struct {
	Tools []tsToolInfo `json:"tools"`
}

type tsToolInfo struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"inputSchema,omitempty"`
}

type tsCallToolResult struct {
	Content           []json.RawMessage `json:"content,omitempty"`
	StructuredContent json.RawMessage   `json:"structuredContent,omitempty"`
	IsError           bool              `json:"isError,omitempty"`
}

type tsTextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (c *tsStdioClient) listTools() tsToolsListResult {
	c.t.Helper()
	resp := c.call("tools/list", map[string]any{})
	if resp.Error != nil {
		c.t.Fatalf("tools/list failed: code=%d msg=%s", resp.Error.Code, resp.Error.Message)
	}
	var result tsToolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		c.t.Fatalf("unmarshal tools/list result: %v", err)
	}
	return result
}

func (c *tsStdioClient) callTool(name string, arguments map[string]any) (tsCallToolResult, *tsJsonrpcError) {
	c.t.Helper()
	resp := c.call("tools/call", map[string]any{
		"name":      name,
		"arguments": arguments,
	})
	if resp.Error != nil {
		return tsCallToolResult{}, resp.Error
	}
	var result tsCallToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		c.t.Fatalf("unmarshal tools/call result: %v", err)
	}
	return result, nil
}

func TestTypeScriptGeneratedNodeServerOverStdio(t *testing.T) {
	client := connectTypeScriptGeneratedNodeServer(t, "")
	_ = client // client cleanup handled by t.Cleanup

	tools := client.listTools()

	var toolNames []string
	for _, tool := range tools.Tools {
		toolNames = append(toolNames, tool.Name)
	}
	slices.Sort(toolNames)
	if !slices.Equal(toolNames, []string{"Advanced", "Scalar"}) {
		t.Fatalf("tool names = %v, want [Advanced Scalar]", toolNames)
	}

	validateTypeScriptToolInputSchema(t, tools.Tools, "Scalar", typeScriptScalarStdioArguments())
	validateTypeScriptToolInputSchema(t, tools.Tools, "Advanced", typeScriptAdvancedStdioArguments())

	scalarResult, rpcErr := client.callTool("Scalar", typeScriptScalarStdioArguments())
	if rpcErr != nil {
		t.Fatalf("CallTool(Scalar) over generated Node stdio failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}
	if scalarResult.IsError {
		t.Fatalf("Scalar returned tool error over generated Node stdio: %+v", scalarResult)
	}
	assertTypeScriptTextStructuredContentMatch(t, "Scalar", scalarResult)
	scalarStructured := decodeTypeScriptStdioMap(t, scalarResult.StructuredContent)
	if got := scalarStructured["int64Value"]; got != "-4567890123" {
		t.Fatalf("int64Value = %v, want -4567890123", got)
	}
	if got := scalarStructured["bytesValue"]; got != "aGVsbG8=" {
		t.Fatalf("bytesValue = %v, want aGVsbG8=", got)
	}
	if got := scalarStructured["rawRatio"]; got != "NaN" {
		t.Fatalf("rawRatio = %v, want NaN", got)
	}

	advancedResult, rpcErr := client.callTool("Advanced", typeScriptAdvancedStdioArguments())
	if rpcErr != nil {
		t.Fatalf("CallTool(Advanced) over generated Node stdio failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}
	if advancedResult.IsError {
		t.Fatalf("Advanced returned tool error over generated Node stdio: %+v", advancedResult)
	}
	assertTypeScriptTextStructuredContentMatch(t, "Advanced", advancedResult)
	advancedStructured := decodeTypeScriptStdioMap(t, advancedResult.StructuredContent)
	if got := advancedStructured["rawRatio"]; got != "-Infinity" {
		t.Fatalf("rawRatio = %v, want -Infinity", got)
	}
	if got := advancedStructured["blob"]; got != "aGVsbG8=" {
		t.Fatalf("blob = %v, want aGVsbG8=", got)
	}
	if got := advancedStructured["cityAlias"]; got != "paris-fr" {
		t.Fatalf("cityAlias = %v, want paris-fr", got)
	}
	detailAny, ok := advancedStructured["detailAny"].(map[string]any)
	if !ok {
		t.Fatalf("detailAny has type %T, want map[string]any", advancedStructured["detailAny"])
	}
	if got := detailAny["@type"]; got != "type.googleapis.com/runtime.v1.Detail" {
		t.Fatalf("detailAny.@type = %v, want runtime detail type URL", got)
	}
	tree, ok := advancedStructured["tree"].(map[string]any)
	if !ok {
		t.Fatalf("tree has type %T, want map[string]any", advancedStructured["tree"])
	}
	if got := tree["name"]; got != "root" {
		t.Fatalf("tree.name = %v, want root", got)
	}
}

func TestTypeScriptGeneratedNodeServerRejectsInvalidInputOverStdio(t *testing.T) {
	client := connectTypeScriptGeneratedNodeServer(t, "")

	_, rpcErr := client.callTool("Scalar", map[string]any{
		"textValue": "missing-required-fields",
	})
	if rpcErr == nil {
		t.Fatal("CallTool(Scalar) unexpectedly succeeded with invalid input")
	}
	lower := strings.ToLower(rpcErr.Message)
	if !strings.Contains(lower, "invalid") || !strings.Contains(lower, "scalar") {
		t.Fatalf("CallTool(Scalar) error = %v, want invalid-input failure naming Scalar", rpcErr.Message)
	}
}

func TestTypeScriptGeneratedNodeServerRejectsInvalidOutputOverStdio(t *testing.T) {
	client := connectTypeScriptGeneratedNodeServer(t, "scalar")

	_, rpcErr := client.callTool("Scalar", typeScriptScalarStdioArguments())
	if rpcErr == nil {
		t.Fatal("CallTool(Scalar) unexpectedly succeeded with invalid output")
	}
	lower := strings.ToLower(rpcErr.Message)
	if !strings.Contains(lower, "validate output") && !strings.Contains(lower, "output") {
		t.Fatalf("CallTool(Scalar) error = %v, want output validation failure", rpcErr.Message)
	}
}

func connectTypeScriptGeneratedNodeServer(t *testing.T, invalidOutput string) *tsStdioClient {
	t.Helper()

	_, cancel := context_WithTimeout(t, 30*time.Second)
	t.Cleanup(cancel)

	cmd := buildTypeScriptGeneratedNodeServerCommand(t, invalidOutput)
	client := newTSStdioClient(t, cmd)
	client.initialize()

	return client
}

// context_WithTimeout wraps context.WithTimeout but returns a cancel function tied to testing.T.
func context_WithTimeout(t *testing.T, d time.Duration) (struct{}, func()) {
	t.Helper()
	timer := time.AfterFunc(d, func() {
		t.Errorf("TypeScript stdio test timed out after %v", d)
	})
	return struct{}{}, func() { timer.Stop() }
}

func buildTypeScriptGeneratedNodeServerCommand(t *testing.T, invalidOutput string) *exec.Cmd {
	t.Helper()

	spikeDir := requireTypeScriptSDKSpike(t)
	const protoPath = "runtime/v1/stdio-contract.proto"
	plugin := newTempProtogenPlugin(t, map[string]string{
		protoPath: strings.Join([]string{
			`syntax = "proto3";`,
			`package runtime.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/runtime;runtimev1";`,
			`import "google/protobuf/any.proto";`,
			`import "google/protobuf/duration.proto";`,
			`import "google/protobuf/timestamp.proto";`,
			`import "google/protobuf/wrappers.proto";`,
			`service RuntimeAPI {`,
			`  rpc Scalar(ScalarRequest) returns (ScalarResponse);`,
			`  rpc Advanced(AdvancedRequest) returns (AdvancedResponse);`,
			`}`,
			`message ScalarRequest {`,
			`  string text_value = 1;`,
			`  int64 int64_value = 2;`,
			`  uint64 uint64_value = 3;`,
			`  bytes bytes_value = 4;`,
			`  double raw_ratio = 5;`,
			`}`,
			`message ScalarResponse {`,
			`  string text_value = 1;`,
			`  int64 int64_value = 2;`,
			`  uint64 uint64_value = 3;`,
			`  bytes bytes_value = 4;`,
			`  double raw_ratio = 5;`,
			`}`,
			`message AdvancedRequest {`,
			`  map<string, string> labels = 1;`,
			`  optional google.protobuf.Timestamp observed_at = 2;`,
			`  optional google.protobuf.Duration ttl = 3;`,
			`  optional google.protobuf.Any detail_any = 4;`,
			`  optional RecursiveNode tree = 5;`,
			`  optional double raw_ratio = 6;`,
			`  optional google.protobuf.BytesValue blob = 7;`,
			`  oneof selector {`,
			`    string city_alias = 8;`,
			`    int64 city_id = 9;`,
			`    Detail city_details = 10;`,
			`  }`,
			`}`,
			`message AdvancedResponse {`,
			`  map<string, string> labels = 1;`,
			`  optional google.protobuf.Timestamp observed_at = 2;`,
			`  optional google.protobuf.Duration ttl = 3;`,
			`  optional google.protobuf.Any detail_any = 4;`,
			`  optional RecursiveNode tree = 5;`,
			`  optional double raw_ratio = 6;`,
			`  optional google.protobuf.BytesValue blob = 7;`,
			`  oneof selector {`,
			`    string city_alias = 8;`,
			`    int64 city_id = 9;`,
			`    Detail city_details = 10;`,
			`  }`,
			`}`,
			`message Detail { string label = 1; }`,
			`message RecursiveNode {`,
			`  string name = 1;`,
			`  optional RecursiveNode child = 2;`,
			`  repeated RecursiveNode children = 3;`,
			`}`,
			``,
		}, "\n"),
	}, protoPath)
	if err := Generate(plugin, Options{Language: LanguageTypeScript}); err != nil {
		t.Fatalf("Generate TypeScript stdio fixture: %v", err)
	}

	tempProject, err := os.MkdirTemp(spikeDir, ".tmp-typescript-stdio-*")
	if err != nil {
		t.Fatalf("create temporary TypeScript stdio project under sdk-spike: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(tempProject); err != nil {
			t.Errorf("remove temporary TypeScript stdio project: %v", err)
		}
	})

	sourceDir := filepath.Join(tempProject, "runtime", "v1")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("create TypeScript stdio fixture source dir: %v", err)
	}

	writeTypeScriptFixtureFile(t, filepath.Join(sourceDir, "stdio_contract_mcp.ts"), generatedFileContent(t, plugin, "runtime/v1/stdio_contract_mcp.ts"))
	writeTypeScriptFixtureFile(t, filepath.Join(sourceDir, "stdio_contract_pb.ts"), []byte(protobufESStdioFixtureModule(t, plugin, protoPath)))
	writeTypeScriptFixtureFile(t, filepath.Join(sourceDir, "stdio_server.ts"), []byte(typeScriptStdioServerFixture()))
	writeTypeScriptFixtureFile(t, filepath.Join(tempProject, "tsconfig.json"), []byte(typeScriptStdioFixtureTSConfig()))

	compile := exec.Command("npm", "exec", "--prefix", spikeDir, "--", "tsc", "-p", filepath.Join(tempProject, "tsconfig.json"))
	compile.Dir = spikeDir
	compile.Env = append(os.Environ(), "NO_COLOR=1")
	compileOutput, err := compile.CombinedOutput()
	if err != nil {
		t.Fatalf("generated TypeScript stdio server failed NodeNext compile via npm exec:\n%s", string(compileOutput))
	}

	cmd := exec.Command("node", filepath.Join(tempProject, "dist", "runtime", "v1", "stdio_server.js"))
	cmd.Dir = spikeDir
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	if invalidOutput != "" {
		cmd.Env = append(cmd.Env, "PROTOC_GEN_MCP_NODE_INVALID_OUTPUT="+invalidOutput)
	}
	return cmd
}

func protobufESStdioFixtureModule(t *testing.T, plugin *protogen.Plugin, protoPath string) string {
	t.Helper()

	descriptorBase64 := protobufESFileDescriptorBase64(t, plugin, protoPath)
	return fmt.Sprintf(`
import type { Message } from "@bufbuild/protobuf";
import { fileDesc, messageDesc, type GenFile, type GenMessage } from "@bufbuild/protobuf/codegenv2";
import {
  file_google_protobuf_any,
  file_google_protobuf_duration,
  file_google_protobuf_timestamp,
  file_google_protobuf_wrappers,
} from "@bufbuild/protobuf/wkt";

export const file_runtime_v1_stdio_contract: GenFile = fileDesc(%q, [
  file_google_protobuf_any,
  file_google_protobuf_duration,
  file_google_protobuf_timestamp,
  file_google_protobuf_wrappers,
]);

export type ScalarRequest = Message<"runtime.v1.ScalarRequest"> & Record<string, unknown>;
export type ScalarResponse = Message<"runtime.v1.ScalarResponse"> & Record<string, unknown>;
export type AdvancedRequest = Message<"runtime.v1.AdvancedRequest"> & Record<string, unknown>;
export type AdvancedResponse = Message<"runtime.v1.AdvancedResponse"> & Record<string, unknown>;

export const ScalarRequestSchema: GenMessage<ScalarRequest> = messageDesc(file_runtime_v1_stdio_contract, 0);
export const ScalarResponseSchema: GenMessage<ScalarResponse> = messageDesc(file_runtime_v1_stdio_contract, 1);
export const AdvancedRequestSchema: GenMessage<AdvancedRequest> = messageDesc(file_runtime_v1_stdio_contract, 2);
export const AdvancedResponseSchema: GenMessage<AdvancedResponse> = messageDesc(file_runtime_v1_stdio_contract, 3);
`, descriptorBase64)
}

func typeScriptStdioServerFixture() string {
	return `
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { registerRuntimeAPITools, type RuntimeAPIToolHandler } from "./stdio_contract_mcp.js";
import type { AdvancedRequest, AdvancedResponse, ScalarRequest, ScalarResponse } from "./stdio_contract_pb.js";

const invalidOutput = process.env.PROTOC_GEN_MCP_NODE_INVALID_OUTPUT ?? "";

const handler: RuntimeAPIToolHandler = {
  scalar(_ctx, request: ScalarRequest): ScalarResponse {
    if (invalidOutput === "scalar") {
      return {
        ...(request as Record<string, unknown>),
        $typeName: "runtime.v1.ScalarResponse",
        textValue: 123 as unknown as string,
      } as ScalarResponse;
    }
    return {
      ...(request as Record<string, unknown>),
      $typeName: "runtime.v1.ScalarResponse",
    } as ScalarResponse;
  },
  advanced(_ctx, request: AdvancedRequest): AdvancedResponse {
    return {
      ...(request as Record<string, unknown>),
      $typeName: "runtime.v1.AdvancedResponse",
    } as AdvancedResponse;
  },
};

const server = new Server(
  { name: "generated-typescript-stdio", version: "0.0.0" },
  { capabilities: { tools: {} } },
);
registerRuntimeAPITools(server, handler, "");
await server.connect(new StdioServerTransport());
`
}

func typeScriptStdioFixtureTSConfig() string {
	return `
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "strict": true,
    "verbatimModuleSyntax": true,
    "types": ["node"],
    "rootDir": ".",
    "outDir": "dist"
  },
  "include": [
    "runtime/v1/stdio_contract_mcp.ts",
    "runtime/v1/stdio_contract_pb.ts",
    "runtime/v1/stdio_server.ts"
  ]
}
`
}

func typeScriptScalarStdioArguments() map[string]any {
	return map[string]any{
		"textValue":   "scalar-demo",
		"int64Value":  "-4567890123",
		"uint64Value": "4567890123",
		"bytesValue":  "aGVsbG8=",
		"rawRatio":    "NaN",
	}
}

func typeScriptAdvancedStdioArguments() map[string]any {
	return map[string]any{
		"labels":     map[string]any{"env": "prod", "team": "core"},
		"observedAt": "2026-03-09T10:11:12Z",
		"ttl":        "3600s",
		"detailAny": map[string]any{
			"@type": "type.googleapis.com/runtime.v1.Detail",
			"label": "from-any",
		},
		"tree": map[string]any{
			"name": "root",
			"child": map[string]any{
				"name": "leaf",
			},
		},
		"rawRatio":  "-Infinity",
		"blob":      "aGVsbG8=",
		"cityAlias": "paris-fr",
	}
}

func validateTypeScriptToolInputSchema(t *testing.T, tools []tsToolInfo, toolName string, arguments map[string]any) {
	t.Helper()

	tool := findTypeScriptStdioTool(t, tools, toolName)
	rawSchema, err := json.Marshal(tool.InputSchema)
	if err != nil {
		t.Fatalf("marshal input schema for tool %q: %v", toolName, err)
	}

	var schema jsonschema.Schema
	if err := json.Unmarshal(rawSchema, &schema); err != nil {
		t.Fatalf("unmarshal input schema for tool %q: %v", toolName, err)
	}

	resolved, err := schema.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	if err != nil {
		t.Fatalf("resolve input schema for tool %q: %v", toolName, err)
	}
	if err := resolved.Validate(arguments); err != nil {
		t.Fatalf("input schema for tool %q rejected valid arguments %v: %v", toolName, arguments, err)
	}
}

func assertTypeScriptTextStructuredContentMatch(t *testing.T, toolName string, result tsCallToolResult) {
	t.Helper()

	if len(result.Content) != 1 {
		t.Fatalf("%s returned %d content items, want 1", toolName, len(result.Content))
	}

	var tc tsTextContent
	if err := json.Unmarshal(result.Content[0], &tc); err != nil {
		t.Fatalf("unmarshal text content for %s: %v", toolName, err)
	}

	var fromText map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &fromText); err != nil {
		t.Fatalf("decode text content for %s: %v", toolName, err)
	}

	fromStructured := decodeTypeScriptStdioMap(t, result.StructuredContent)
	if !reflect.DeepEqual(fromText, fromStructured) {
		t.Fatalf("%s text content %v does not match structured content %v", toolName, fromText, fromStructured)
	}
}

func decodeTypeScriptStdioMap(t *testing.T, value any) map[string]any {
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

func findTypeScriptStdioTool(t *testing.T, tools []tsToolInfo, toolName string) tsToolInfo {
	t.Helper()

	for _, tool := range tools {
		if tool.Name == toolName {
			return tool
		}
	}
	t.Fatalf("tool %q not found in tools/list", toolName)
	return tsToolInfo{}
}
