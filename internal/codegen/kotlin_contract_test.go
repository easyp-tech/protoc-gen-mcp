package codegen

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/compiler/protogen"
)

func TestKotlinContract_PublicAPIAndRegistrationShape(t *testing.T) {
	generated := renderBasicKotlinFixture(t)

	wantSnippets := []string{
		"interface ExampleAPIToolHandler",
		"suspend fun createReport(ctx: ClientConnection, request: CreateReportRequest): CreateReportResponse",
		"fun registerExampleAPITools(server: Server, impl: ExampleAPIToolHandler, namespace: String? = null)",
		"installMcpHandlers(server)",
	}
	assertKotlinContains(t, generated, wantSnippets...)

	notWantSnippets := []string{
		"server." + "addTool(",
		"addTool" + "(name =",
		"Dynamic" + "Message",
		"Descr" + "iptor",
	}
	assertKotlinOmits(t, generated, notWantSnippets...)
}

func TestKotlinContract_LowLevelServerContractShape(t *testing.T) {
	generated := renderBasicKotlinFixture(t)

	wantSnippets := []string{
		"private class ServerToolRegistry",
		"private suspend fun dispatchToolCall",
		"ListToolsRequest",
		"CallToolRequest",
		"private suspend fun listRegisteredTools",
		"session.setRequestHandler(Method.Defined.ToolsList)",
		"session.setRequestHandler(Method.Defined.ToolsCall)",
	}
	assertKotlinContains(t, generated, wantSnippets...)
}

func TestKotlinContract_JavaPackageAndWrapperImports(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/shared.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.shared.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/shared;sharedv1";`,
			`option java_package = "com.example.shared";`,
			`option java_outer_classname = "SharedTypes";`,
			`message SharedRequest {}`,
			`message SharedResponse {}`,
			``,
		}, "\n"),
		"test/v1/shared_multi.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.sharedmulti.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/sharedmulti;sharedmultiv1";`,
			`option java_package = "com.example.sharedmulti";`,
			`option java_multiple_files = true;`,
			`message MultiRequest {}`,
			`message MultiResponse {}`,
			``,
		}, "\n"),
		"test/v1/shared_default_outer.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.defaultouter.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/defaultouter;defaultouterv1";`,
			`option java_package = "com.example.defaultouter";`,
			`message DefaultRequest {}`,
			`message DefaultResponse {}`,
			``,
		}, "\n"),
		"test/v1/service.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.service.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/service;servicev1";`,
			`option java_package = "com.example.service";`,
			`option java_multiple_files = true;`,
			`import "test/v1/shared.proto";`,
			`import "test/v1/shared_multi.proto";`,
			`import "test/v1/shared_default_outer.proto";`,
			`service CrossFileAPI {`,
			`  rpc UseShared(test.shared.v1.SharedRequest) returns (test.shared.v1.SharedResponse);`,
			`  rpc UseMulti(test.sharedmulti.v1.MultiRequest) returns (test.sharedmulti.v1.MultiResponse);`,
			`  rpc UseDefault(test.defaultouter.v1.DefaultRequest) returns (test.defaultouter.v1.DefaultResponse);`,
			`}`,
			``,
		}, "\n"),
	}, "test/v1/service.proto")

	generated := renderKotlinFileFromPlugin(t, plugin, "test/v1/service.proto")
	wantSnippets := []string{
		"package com.example.service",
		"import com.example.shared.SharedTypes",
		"import com.example.sharedmulti.MultiRequest",
		"import com.example.sharedmulti.MultiResponse",
		"import com.example.defaultouter.SharedDefaultOuter",
		"suspend fun useShared(ctx: ClientConnection, request: SharedTypes.SharedRequest): SharedTypes.SharedResponse",
		"suspend fun useMulti(ctx: ClientConnection, request: MultiRequest): MultiResponse",
		"suspend fun useDefault(ctx: ClientConnection, request: SharedDefaultOuter.DefaultRequest): SharedDefaultOuter.DefaultResponse",
	}
	assertKotlinContains(t, generated, wantSnippets...)
}

func TestKotlinContract_RawSchemaProjectionAndValidation(t *testing.T) {
	generated := renderBasicKotlinFixture(t)

	wantSnippets := []string{
		"private fun buildListTool(tool: RegisteredTool): Tool",
		"private fun loadSchema(rawSchemaJson: String): JsonObject",
		"validateJson(tool.inputSchemaJson, arguments)",
		"parseProtoJson(arguments.toString(), tool.requestBuilder())",
		"marshalProtoJson(responseMessage)",
		"validateJson(tool.outputSchemaJson, payload)",
		"CallToolResult(content = listOf(TextContent(text = textPayload)), structuredContent = payload)",
	}
	assertKotlinContains(t, generated, wantSnippets...)
}

func TestKotlinContract_UsesPublicServerClientConnection(t *testing.T) {
	generated := renderBasicKotlinFixture(t)

	assertKotlinContains(t, generated, "server.clientConnection(session.sessionId)")
	assertKotlinOmits(t, generated, "session.clientConnection")
}

func TestKotlinContract_UsesNetworkntJsonSchemaValidation(t *testing.T) {
	generated := renderBasicKotlinFixture(t)

	wantSnippets := []string{
		"import com.networknt.schema.InputFormat",
		"import com.networknt.schema.JsonSchemaFactory",
		"import com.networknt.schema.SpecVersion",
		"private val jsonSchemaFactory = JsonSchemaFactory.getInstance(SpecVersion.VersionFlag.V202012)",
		"jsonSchemaFactory.getSchema(rawSchemaJson).validate(payload.toString(), InputFormat.JSON)",
		`invalidParams(tool.name, error.message ?: error.toString())`,
		`IllegalStateException("mcpruntime: validate output for tool '${tool.name}': ${error.message}", error)`,
	}
	assertKotlinContains(t, generated, wantSnippets...)
}

func TestKotlinContract_EscapesSchemaDollarSigns(t *testing.T) {
	generated := renderRepositoryExampleKotlinFixture(t)

	assertKotlinContains(t, generated, `\$ref`, `\$defs`)
	assertKotlinOmits(t, generated, `\"$ref\"`, `\"$defs\"`)
}

func TestKotlinContract_AnnotationsIconsAndThemeValidation(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/metadata.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.metadata.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/metadata;metadatav1";`,
			`option java_package = "com.example.metadata";`,
			`option java_multiple_files = true;`,
			`import "mcp/options/v1/options.proto";`,
			`message MetadataRequest {}`,
			`message MetadataResponse {}`,
			`service MetadataAPI {`,
			`  rpc OptionalTask(MetadataRequest) returns (MetadataResponse) {`,
			`    option (mcp.options.v1.method) = {`,
			`      annotations: { read_only_hint: true open_world_hint: false }`,
			`      icons: [{ src: "https://example.com/light.svg" mime_type: "image/svg+xml" sizes: "64x64" theme: "light" }]`,
			`      execution: { task_support: TASK_SUPPORT_OPTIONAL }`,
			`    };`,
			`  }`,
			`  rpc RequiredTask(MetadataRequest) returns (MetadataResponse) {`,
			`    option (mcp.options.v1.method) = {`,
			`      icons: [{ src: "https://example.com/dark.svg" mime_type: "image/svg+xml" sizes: "64x64" theme: "dark" }]`,
			`      execution: { task_support: TASK_SUPPORT_REQUIRED }`,
			`    };`,
			`  }`,
			`}`,
			``,
		}, "\n"),
	}, "test/v1/metadata.proto")

	generated := renderKotlinFileFromPlugin(t, plugin, "test/v1/metadata.proto")
	wantSnippets := []string{
		"ToolAnnotations(readOnlyHint = true, openWorldHint = false)",
		"ToolExecution(taskSupport = TaskSupport.Optional)",
		"ToolExecution(taskSupport = TaskSupport.Required)",
		"Icon.Theme.Light",
		"Icon.Theme.Dark",
		`theme = iconThemeOrError("light")`,
		`theme = iconThemeOrError("dark")`,
	}
	assertKotlinContains(t, generated, wantSnippets...)

	invalidPlugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/invalid_theme.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.invalidtheme.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/invalidtheme;invalidthemev1";`,
			`option java_package = "com.example.invalidtheme";`,
			`option java_multiple_files = true;`,
			`import "mcp/options/v1/options.proto";`,
			`message InvalidThemeRequest {}`,
			`message InvalidThemeResponse {}`,
			`service InvalidThemeAPI {`,
			`  rpc BadIcon(InvalidThemeRequest) returns (InvalidThemeResponse) {`,
			`    option (mcp.options.v1.method) = {`,
			`      icons: [{ src: "https://example.com/icon.svg" theme: "solarized" }]`,
			`    };`,
			`  }`,
			`}`,
			``,
		}, "\n"),
	}, "test/v1/invalid_theme.proto")
	invalidFile := invalidPlugin.FilesByPath["test/v1/invalid_theme.proto"]
	if invalidFile == nil {
		t.Fatal("invalid theme proto file not found in plugin")
	}
	shared, err := CollectFileModel(invalidFile, Options{Language: LanguageKotlin})
	if err != nil {
		t.Fatalf("CollectFileModel invalid theme: %v", err)
	}
	jvm, err := CollectJVMFileModel(invalidFile, shared)
	if err != nil {
		t.Fatalf("CollectJVMFileModel invalid theme: %v", err)
	}
	err = renderKotlinFile(invalidPlugin, jvm)
	if err == nil {
		t.Fatal("renderKotlinFile unexpectedly succeeded for invalid icon theme")
	}
	if !strings.Contains(err.Error(), `unsupported Kotlin icon theme "solarized"`) {
		t.Fatalf("renderKotlinFile error = %v, want invalid theme rejection", err)
	}
	if got := len(invalidPlugin.Response().GetFile()); got != 0 {
		t.Fatalf("invalid theme emitted %d files before failing, want 0", got)
	}
}

func renderBasicKotlinFixture(t *testing.T) string {
	t.Helper()

	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/example.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.example.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/example;examplev1";`,
			`option java_package = "com.example.contract";`,
			`option java_multiple_files = true;`,
			`import "mcp/options/v1/options.proto";`,
			`message CreateReportRequest { string title = 1; }`,
			`message CreateReportResponse { string report_id = 1; }`,
			`service ExampleAPI {`,
			`  option (mcp.options.v1.service) = { namespace: "example" };`,
			`  rpc CreateReport(CreateReportRequest) returns (CreateReportResponse) {`,
			`    option (mcp.options.v1.method) = { title: "Create report" };`,
			`  }`,
			`}`,
			``,
		}, "\n"),
	}, "test/v1/example.proto")

	return renderKotlinFileFromPlugin(t, plugin, "test/v1/example.proto")
}

func renderRepositoryExampleKotlinFixture(t *testing.T) string {
	t.Helper()

	plugin := newExampleProtogenPlugin(t)
	if err := Generate(plugin, Options{Language: LanguageKotlin}); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	return string(generatedFileContent(t, plugin, "internal/testproto/example/v1/example_mcp.kt"))
}

func renderKotlinFileFromPlugin(t *testing.T, plugin *protogen.Plugin, protoPath string) string {
	t.Helper()

	file := plugin.FilesByPath[protoPath]
	if file == nil {
		t.Fatalf("proto file %q not found in plugin", protoPath)
	}
	shared, err := CollectFileModel(file, Options{Language: LanguageKotlin})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	jvm, err := CollectJVMFileModel(file, shared)
	if err != nil {
		t.Fatalf("CollectJVMFileModel: %v", err)
	}
	if err := renderKotlinFile(plugin, jvm); err != nil {
		t.Fatalf("renderKotlinFile: %v", err)
	}

	return string(generatedFileContent(t, plugin, strings.TrimSuffix(protoPath, ".proto")+"_mcp.kt"))
}

func assertKotlinContains(t *testing.T, generated string, snippets ...string) {
	t.Helper()
	for _, snippet := range snippets {
		if !strings.Contains(generated, snippet) {
			t.Fatalf("generated Kotlin missing snippet %q\n%s", snippet, generated)
		}
	}
}

func assertKotlinOmits(t *testing.T, generated string, snippets ...string) {
	t.Helper()
	for _, snippet := range snippets {
		if strings.Contains(generated, snippet) {
			t.Fatalf("generated Kotlin must not contain snippet %q\n%s", snippet, generated)
		}
	}
}
