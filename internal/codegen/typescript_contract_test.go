package codegen

import (
	"strings"
	"testing"
)

func TestTypeScriptContract_PublicAPIAndLowLevelImports(t *testing.T) {
	generated := renderBasicTypeScriptFixture(t)

	wantSnippets := []string{
		`import type { Server } from "@modelcontextprotocol/sdk/server/index.js";`,
		`import * as protocGenMcp from "@easyp-tech/protoc-gen-mcp-runtime";`,
		`import type { CreateReportRequest, CreateReportResponse } from "./example_pb.js";`,
		`import { CreateReportRequestSchema, CreateReportResponseSchema, file_test_v1_example } from "./example_pb.js";`,
		"export type ToolRequestContext = protocGenMcp.ToolRequestContext;",
		"protocGenMcp.checkGencodeVersion(1);",
	}
	assertTypeScriptContains(t, generated, wantSnippets...)
}

func TestTypeScriptContract_PublicHandlerAndRegisterShape(t *testing.T) {
	generated := renderBasicTypeScriptFixture(t)

	wantSnippets := []string{
		"export interface ExampleAPIToolHandler {",
		"  createReport(",
		"    ctx: ToolRequestContext,",
		"    request: CreateReportRequest,",
		"  ): CreateReportResponse | Promise<CreateReportResponse>;",
		"export function registerExampleAPITools(",
		"  server: Server,",
		"  impl: ExampleAPIToolHandler,",
		"  namespace?: string | null,",
		"): void",
	}
	assertTypeScriptContains(t, generated, wantSnippets...)
}

func TestTypeScriptContract_SchemaConstantsAndRegistryMetadata(t *testing.T) {
	generated := renderBasicTypeScriptFixture(t)

	wantSnippets := []string{
		"export const EXAMPLE_API_CREATE_REPORT_INPUT_SCHEMA_JSON =",
		"export const EXAMPLE_API_CREATE_REPORT_OUTPUT_SCHEMA_JSON =",
		"name: protocGenMcp.resolveToolName(namespace, \"CreateReport\", \"example\"),",
		"handler: impl.createReport.bind(impl),",
		"inputSchemaJson: EXAMPLE_API_CREATE_REPORT_INPUT_SCHEMA_JSON,",
		"outputSchemaJson: EXAMPLE_API_CREATE_REPORT_OUTPUT_SCHEMA_JSON,",
		"inputSchema: CreateReportRequestSchema,",
		"outputSchema: CreateReportResponseSchema,",
		"registry: protocGenMcp.buildProtoRegistry(file_test_v1_example, file_test_v1_example, file_test_v1_example),",
		"annotations: {",
		"readOnlyHint: true",
		"destructiveHint: false",
		"idempotentHint: true",
		"openWorldHint: false",
		"icons: [",
		`src: "https://example.com/tool.svg"`,
		`mimeType: "image/svg+xml"`,
		`sizes: ["64x64"]`,
		`theme: "light"`,
		"execution: {",
		`taskSupport: "optional"`,
	}
	assertTypeScriptContains(t, generated, wantSnippets...)
}

func TestTypeScriptContract_ToolNameNormalization(t *testing.T) {
	generated := renderTypeScriptToolNameFixture(t)

	wantSnippets := []string{
		`name: protocGenMcp.resolveToolName(namespace, "Create.Report", "example.default"),`,
	}
	assertTypeScriptContains(t, generated, wantSnippets...)
	assertTypeScriptOmits(t, generated, "function normalizeNamespace(namespace:", "return `${resolvedNamespace}_${defaultName}`;")
}

func TestTypeScriptContract_RuntimeDispatchPaths(t *testing.T) {
	generated := renderBasicTypeScriptFixture(t)

	wantSnippets := []string{
		`import * as protocGenMcp from "@easyp-tech/protoc-gen-mcp-runtime";`,
		"protocGenMcp.checkGencodeVersion(1);",
		"protocGenMcp.addTools(server, [",
		"validateInput: protocGenMcp.compileSchema(EXAMPLE_API_CREATE_REPORT_INPUT_SCHEMA_JSON),",
		"validateOutput: protocGenMcp.compileSchema(EXAMPLE_API_CREATE_REPORT_OUTPUT_SCHEMA_JSON),",
	}
	assertTypeScriptContains(t, generated, wantSnippets...)

	notWantSnippets := []string{
		"new WeakMap<Server, ServerToolRegistry>()",
		"registerTool",
		"zod",
		"toJsonSchemaCompat",
		"async function dispatchToolCall(",
		"new Ajv2020(",
		"function buildProtoRegistry(",
	}
	assertTypeScriptOmits(t, generated, notWantSnippets...)
}

func renderBasicTypeScriptFixture(t *testing.T) string {
	t.Helper()

	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/example.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/example;examplev1";`,
			`import "mcp/options/v1/options.proto";`,
			`message CreateReportRequest { string title = 1; }`,
			`message CreateReportResponse { string report_id = 1; }`,
			`service ExampleAPI {`,
			`  option (mcp.options.v1.service) = { namespace: "example" };`,
			`  rpc CreateReport(CreateReportRequest) returns (CreateReportResponse) {`,
			`    option (mcp.options.v1.method) = {`,
			`      title: "Create report"`,
			`      description: "Creates a report."`,
			`      annotations: {`,
			`        read_only_hint: true`,
			`        destructive_hint: false`,
			`        idempotent_hint: true`,
			`        open_world_hint: false`,
			`      }`,
			`      icons: [{`,
			`        src: "https://example.com/tool.svg"`,
			`        mime_type: "image/svg+xml"`,
			`        sizes: "64x64"`,
			`        theme: "light"`,
			`      }]`,
			`      execution: { task_support: TASK_SUPPORT_OPTIONAL }`,
			`    };`,
			`  }`,
			`}`,
			``,
		}, "\n"),
	}, "test/v1/example.proto")

	if err := Generate(plugin, Options{Language: LanguageTypeScript}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return string(generatedFileContent(t, plugin, "test/v1/example_mcp.ts"))
}

func renderTypeScriptToolNameFixture(t *testing.T) string {
	t.Helper()

	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/tool-name.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/toolname;toolnamev1";`,
			`import "mcp/options/v1/options.proto";`,
			`message CreateReportRequest { string title = 1; }`,
			`message CreateReportResponse { string report_id = 1; }`,
			`service ExampleAPI {`,
			`  option (mcp.options.v1.service) = { namespace: "example.default" };`,
			`  rpc CreateReport(CreateReportRequest) returns (CreateReportResponse) {`,
			`    option (mcp.options.v1.method) = { name: "Create.Report" };`,
			`  }`,
			`}`,
			``,
		}, "\n"),
	}, "test/v1/tool-name.proto")

	if err := Generate(plugin, Options{Language: LanguageTypeScript}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return string(generatedFileContent(t, plugin, "test/v1/tool_name_mcp.ts"))
}

func assertTypeScriptContains(t *testing.T, generated string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if !strings.Contains(generated, snippet) {
			t.Fatalf("generated TypeScript missing snippet %q\n%s", snippet, generated)
		}
	}
}

func assertTypeScriptOmits(t *testing.T, generated string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if strings.Contains(generated, snippet) {
			t.Fatalf("generated TypeScript must omit snippet %q\n%s", snippet, generated)
		}
	}
}
