package codegen

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bufbuild/protocompile"
)

func TestGenerateSwiftGoldens(t *testing.T) {
	example := newExampleProtogenPlugin(t)
	if err := Generate(example, Options{Language: LanguageSwift}); err != nil {
		t.Fatalf("Generate example: %v", err)
	}
	assertSwiftGolden(t, generatedFileContent(t, example, "internal/testproto/example/v1/example.mcp.swift"), "example.mcp.swift.golden")

	prompts := newPromptsProtogenPlugin(t)
	if err := Generate(prompts, Options{Language: LanguageSwift}); err != nil {
		t.Fatalf("Generate prompts: %v", err)
	}
	assertSwiftGolden(t, generatedFileContent(t, prompts, "internal/testproto/prompts/v1/prompts.mcp.swift"), "prompts.mcp.swift.golden")

	resources := newResourcesProtogenPlugin(t)
	if err := Generate(resources, Options{Language: LanguageSwift}); err != nil {
		t.Fatalf("Generate resources: %v", err)
	}
	assertSwiftGolden(t, generatedFileContent(t, resources, "internal/testproto/resources/v1/resources.mcp.swift"), "resources.mcp.swift.golden")
}

func assertSwiftGolden(t *testing.T, got []byte, golden string) {
	t.Helper()
	want := readGoldenFile(t, repoRoot(t), "testdata/golden/"+golden)
	if !bytes.Equal(got, want) {
		t.Fatalf("fresh Swift renderer output differs from %s:\n%s", golden, diffBytes(want, got))
	}
}

func TestSwiftNamesFollowSwiftProtobuf(t *testing.T) {
	cases := map[string]string{
		"user_id":     "userID",
		"project_id":  "projectID",
		"name":        "name",
		"home_url":    "homeURL",
		"http_server": "httpServer",
	}
	for in, want := range cases {
		if got := swiftLowerCamel(in); got != want {
			t.Errorf("swiftLowerCamel(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"Type": "TypeMessage", "Foo": "Foo", "TypeMessage": "TypeMessageMessage", "_": "_Message"} {
		if got := swiftSanitizeTypeName(in); got != want {
			t.Errorf("swiftSanitizeTypeName(%q) = %q, want %q", in, got, want)
		}
	}
	if got := swiftMethodName("Return"); got != "`return`" {
		t.Errorf("swiftMethodName(Return) = %q, want backticked keyword", got)
	}
	if got := swiftStringLiteral("a\"b\\c\n\x01"); got != `"a\"b\\c\n\u{1}"` {
		t.Errorf("swiftStringLiteral = %s", got)
	}
	ids := swiftParamIdentifiers([]ResourceParamModel{{Name: "uri"}, {Name: "for"}, {Name: "user_id"}})
	if strings.Join(ids, ",") != "uri_,for_,userID" {
		t.Errorf("swiftParamIdentifiers = %v", ids)
	}
}

func TestGenerate_SwiftStreamingRPCFailsBeforeOutput(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{"stream/v1/stream.proto": strings.Join([]string{
		`syntax = "proto3";`,
		`package stream.v1;`,
		`option go_package = "example.com/stream/v1;streamv1";`,
		`service StreamAPI {`,
		`  rpc Watch(WatchRequest) returns (stream WatchResponse);`,
		`}`,
		`message WatchRequest {}`,
		`message WatchResponse {}`,
	}, "\n")}, "stream/v1/stream.proto")
	err := Generate(plugin, Options{Language: LanguageSwift})
	if err == nil || !strings.Contains(err.Error(), "streaming RPC is not supported") {
		t.Fatalf("Generate error = %v, want streaming rejection", err)
	}
	if files := plugin.Response().GetFile(); len(files) != 0 {
		t.Fatalf("Swift streaming RPC emitted %d files, want 0", len(files))
	}
}

func TestGenerate_SwiftRejectsRequiredTaskSupport(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{"tasks/v1/tasks.proto": strings.Join([]string{
		`syntax = "proto3";`,
		`package tasks.v1;`,
		`option go_package = "example.com/tasks/v1;tasksv1";`,
		`import "mcp/options/v1/options.proto";`,
		`service TaskAPI {`,
		`  rpc Run(RunRequest) returns (RunResponse) {`,
		`    option (mcp.options.v1.method) = { execution: { task_support: TASK_SUPPORT_REQUIRED } };`,
		`  }`,
		`}`,
		`message RunRequest {}`,
		`message RunResponse {}`,
	}, "\n")}, "tasks/v1/tasks.proto")
	err := Generate(plugin, Options{Language: LanguageSwift})
	if err == nil || !strings.Contains(err.Error(), "not supported by lang=swift") {
		t.Fatalf("Generate error = %v, want TASK_SUPPORT_REQUIRED rejection", err)
	}
}

func TestGenerate_SwiftServerOptionEmitsHTTPHandler(t *testing.T) {
	plugin := newCompiledProtogenPlugin(t, []string{"internal/testproto/config/v1/server_config.proto"}, &protocompile.SourceResolver{
		ImportPaths: []string{repoRoot(t)},
	})
	if err := Generate(plugin, Options{Language: LanguageSwift}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	got := string(generatedFileContent(t, plugin, "internal/testproto/config/v1/server_config.mcp.swift"))
	for _, want := range []string{
		"import ProtocGenMCPHTTP",
		`requiredScopes: ["reports:write"],`,
		"func newServerConfigMCPServer(_ handlers: ServerConfigMCPHandlers) async throws -> Server {",
		"func newServerConfigMCPHTTPHandler(_ handlers: ServerConfigMCPHandlers, verifier: (any TokenVerifier)? = nil) throws -> MCPHTTPHandler {",
		`resourceURL: "https://mcp.example.com/mcp",`,
		`jwksURL: "https://auth.example.com/keys.json",`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("generated Swift lacks %q", want)
		}
	}
}
