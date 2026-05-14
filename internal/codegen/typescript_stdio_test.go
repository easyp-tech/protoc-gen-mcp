package codegen

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/compiler/protogen"
)

func TestTypeScriptGeneratedNodeServerOverStdio(t *testing.T) {
	session := connectTypeScriptGeneratedNodeServer(t, "")
	defer session.Close()

	ctx := context.Background()
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools() over generated Node stdio failed: %v", err)
	}

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

	scalarResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "Scalar",
		Arguments: typeScriptScalarStdioArguments(),
	})
	if err != nil {
		t.Fatalf("CallTool(Scalar) over generated Node stdio failed: %v", err)
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

	advancedResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "Advanced",
		Arguments: typeScriptAdvancedStdioArguments(),
	})
	if err != nil {
		t.Fatalf("CallTool(Advanced) over generated Node stdio failed: %v", err)
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
	session := connectTypeScriptGeneratedNodeServer(t, "")
	defer session.Close()

	_, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "Scalar",
		Arguments: map[string]any{
			"textValue": "missing-required-fields",
		},
	})
	if err == nil {
		t.Fatal("CallTool(Scalar) unexpectedly succeeded with invalid input")
	}
	lower := strings.ToLower(err.Error())
	if !strings.Contains(lower, "invalid") || !strings.Contains(lower, "scalar") {
		t.Fatalf("CallTool(Scalar) error = %v, want invalid-input failure naming Scalar", err)
	}
}

func TestTypeScriptGeneratedNodeServerRejectsInvalidOutputOverStdio(t *testing.T) {
	session := connectTypeScriptGeneratedNodeServer(t, "scalar")
	defer session.Close()

	_, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "Scalar",
		Arguments: typeScriptScalarStdioArguments(),
	})
	if err == nil {
		t.Fatal("CallTool(Scalar) unexpectedly succeeded with invalid output")
	}
	lower := strings.ToLower(err.Error())
	if !strings.Contains(lower, "validate output") && !strings.Contains(lower, "output") {
		t.Fatalf("CallTool(Scalar) error = %v, want output validation failure", err)
	}
}

func connectTypeScriptGeneratedNodeServer(t *testing.T, invalidOutput string) *mcp.ClientSession {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "protoc-gen-mcp-typescript-stdio-test-client",
		Version: "v0.0.1",
	}, nil)

	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: buildTypeScriptGeneratedNodeServerCommand(t, invalidOutput)}, nil)
	if err != nil {
		t.Fatalf("client.Connect() to generated Node server over stdio failed: %v", err)
	}
	return session
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

func validateTypeScriptToolInputSchema(t *testing.T, tools []*mcp.Tool, toolName string, arguments map[string]any) {
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

func assertTypeScriptTextStructuredContentMatch(t *testing.T, toolName string, result *mcp.CallToolResult) {
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

	fromStructured := decodeTypeScriptStdioMap(t, result.StructuredContent)
	if !reflect.DeepEqual(fromText, fromStructured) {
		t.Fatalf("%s text content %v does not match structured content %v", toolName, fromText, fromStructured)
	}
}

func decodeTypeScriptStdioMap(t *testing.T, value any) map[string]any {
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

func findTypeScriptStdioTool(t *testing.T, tools []*mcp.Tool, toolName string) *mcp.Tool {
	t.Helper()

	for _, tool := range tools {
		if tool != nil && tool.Name == toolName {
			return tool
		}
	}
	t.Fatalf("tool %q not found in tools/list", toolName)
	return nil
}
