package codegen

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/compiler/protogen"
)

func TestJavaContract_PublicAPIAndRegistrationShape(t *testing.T) {
	generated := renderBasicJavaFixture(t)

	wantSnippets := []string{
		"public final class ExampleMcp {",
		"public interface ExampleAPIToolHandler {",
		"CreateReportResponse createReport(McpAsyncServerExchange ctx, CreateReportRequest request) throws Exception;",
		"public static void registerExampleAPITools(",
		"McpServerTransportProvider transportProvider,",
	}
	assertJavaContains(t, generated, wantSnippets...)

	notWantSnippets := []string{
		"addTool(",
		"DynamicMessage",
	}
	assertJavaOmits(t, generated, notWantSnippets...)
}

func TestJavaContract_OnePublicSidecarClassAndImports(t *testing.T) {
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

	generated := renderJavaFileFromPlugin(t, plugin, "test/v1/service.proto")
	wantSnippets := []string{
		"package com.example.service;",
		"import com.example.shared.SharedTypes;",
		"import com.example.sharedmulti.MultiRequest;",
		"import com.example.sharedmulti.MultiResponse;",
		"import com.example.defaultouter.SharedDefaultOuter;",
		"public final class ServiceMcp {",
		"SharedTypes.SharedRequest",
		"MultiRequest",
		"SharedDefaultOuter.DefaultRequest",
	}
	assertJavaContains(t, generated, wantSnippets...)
}

func TestJavaContract_LowLevelServerContractShape(t *testing.T) {
	generated := renderBasicJavaFixture(t)

	wantSnippets := []string{
		"private static final class RegisteredTool {",
		"private static final class ServerToolRegistry {",
		"transportProvider.setSessionFactory(",
		"new McpServerSession(",
		"requestHandlers.put(McpSchema.METHOD_TOOLS_LIST",
		"requestHandlers.put(McpSchema.METHOD_TOOLS_CALL",
	}
	assertJavaContains(t, generated, wantSnippets...)

	notWantSnippets := []string{
		".tools(",
		".toolCall(",
		"addTool(",
	}
	assertJavaOmits(t, generated, notWantSnippets...)
}

func TestJavaContract_RawSchemaProtoJSONAndValidation(t *testing.T) {
	generated := renderBasicJavaFixture(t)

	wantSnippets := []string{
		"private static final JsonFormat.TypeRegistry PROTO_TYPE_REGISTRY = buildTypeRegistry();",
		"private static JsonFormat.TypeRegistry buildTypeRegistry() {",
		"registerFileTypes(builder, seenFiles, Example.getDescriptor());",
		"builder.add(descriptor);",
		"private static Map<String, Object> loadSchema(String rawSchemaJson) {",
		"private static void validateJson(Map<String, Object> schema, Object payload) {",
		"private static Message parseProtoJson(String argumentsJson, Message.Builder builder) {",
		"private static String marshalProtoJson(Message responseMessage) {",
		"private static Mono<Object> dispatchToolCall(",
		"validateJson(tool.inputSchema, arguments);",
		"parseProtoJson(JSON_MAPPER.writeValueAsString(arguments), tool.requestBuilder.get())",
		"marshalProtoJson(responseMessage);",
		"validateJson(tool.outputSchema, structuredContent);",
		"JsonFormat.parser().usingTypeRegistry(PROTO_TYPE_REGISTRY).merge(",
		"usingTypeRegistry(PROTO_TYPE_REGISTRY)",
		"alwaysPrintFieldsWithNoPresence()",
		"omittingInsignificantWhitespace()",
		"McpSchema.ErrorCodes.INVALID_PARAMS",
		".isError(true)",
	}
	assertJavaContains(t, generated, wantSnippets...)
}

func TestJavaContract_MetadataProjection(t *testing.T) {
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

	generated := renderJavaFileFromPlugin(t, plugin, "test/v1/metadata.proto")
	wantSnippets := []string{
		"private static Map<String, Object> listRegisteredTools(ServerToolRegistry registry) {",
		"private static Map<String, Object> toolAsProtocolMap(RegisteredTool tool) {",
		"new LinkedHashMap<>()",
		"toolAnnotationsMapOrNull(",
		"toolExecutionMapOrNull(",
		"iconThemeOrError(",
		`"icons"`,
		`"execution"`,
		`"taskSupport"`,
		`"light"`,
		`"dark"`,
	}
	assertJavaContains(t, generated, wantSnippets...)
	assertJavaOmits(t, generated, "Tool.builder(")
}

func TestJavaContract_MetadataProjectionRejectsUnknownIconTheme(t *testing.T) {
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
	shared, err := CollectFileModel(invalidFile, Options{Language: LanguageJava})
	if err != nil {
		t.Fatalf("CollectFileModel invalid theme: %v", err)
	}
	jvm, err := CollectJVMFileModel(invalidFile, shared)
	if err != nil {
		t.Fatalf("CollectJVMFileModel invalid theme: %v", err)
	}
	err = renderJavaFile(invalidPlugin, jvm)
	if err == nil {
		t.Fatal("renderJavaFile unexpectedly succeeded for invalid icon theme")
	}
	if !strings.Contains(err.Error(), `unsupported Java icon theme "solarized"`) {
		t.Fatalf("renderJavaFile error = %v, want invalid theme rejection", err)
	}
	if got := len(invalidPlugin.Response().GetFile()); got != 0 {
		t.Fatalf("invalid theme emitted %d files before failing, want 0", got)
	}
}

func renderBasicJavaFixture(t *testing.T) string {
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

	return renderJavaFileFromPlugin(t, plugin, "test/v1/example.proto")
}

func renderJavaFileFromPlugin(t *testing.T, plugin *protogen.Plugin, protoPath string) string {
	t.Helper()

	file := plugin.FilesByPath[protoPath]
	if file == nil {
		t.Fatalf("proto file %q not found in plugin", protoPath)
	}
	shared, err := CollectFileModel(file, Options{Language: LanguageJava})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	jvm, err := CollectJVMFileModel(file, shared)
	if err != nil {
		t.Fatalf("CollectJVMFileModel: %v", err)
	}
	if err := renderJavaFile(plugin, jvm); err != nil {
		t.Fatalf("renderJavaFile: %v", err)
	}

	return string(generatedFileContent(t, plugin, javaSidecarOutputPath(jvmGeneratedFilenamePrefixForProtoPath(protoPath))))
}

func assertJavaContains(t *testing.T, generated string, snippets ...string) {
	t.Helper()
	for _, snippet := range snippets {
		if !strings.Contains(generated, snippet) {
			t.Fatalf("generated Java missing snippet %q\n%s", snippet, generated)
		}
	}
}

func assertJavaOmits(t *testing.T, generated string, snippets ...string) {
	t.Helper()
	for _, snippet := range snippets {
		if strings.Contains(generated, snippet) {
			t.Fatalf("generated Java unexpectedly contains snippet %q\n%s", snippet, generated)
		}
	}
}
