package codegen

import (
	"fmt"
	"strings"
	"testing"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
)

func goNamingProto(pkg, goPackage, declarations string) string {
	return fmt.Sprintf(`syntax = "proto3";
package %s;
option go_package = %q;
import "mcp/options/v1/options.proto";
option (mcp.options.v1.server) = { name: "naming-test" };
%s
`, pkg, goPackage, declarations)
}

func TestGoFileAPINamesSurviveDirectoryMoves(t *testing.T) {
	const declarations = `
message Guide { option (mcp.options.v1.prompt) = { name: "guide" }; }
message Status { option (mcp.options.v1.resource) = { uri: "status://current" }; }
`
	for _, protoPath := range []string{"old/location/server_config.proto", "new/deeper/location/server_config.proto"} {
		t.Run(protoPath, func(t *testing.T) {
			plugin := newTempProtogenPlugin(t, map[string]string{
				protoPath: goNamingProto("test.v1", "example.com/contracts/server;serverv1", declarations),
			}, protoPath)
			if err := Generate(plugin, Options{Language: LanguageGo}); err != nil {
				t.Fatalf("Generate: %v", err)
			}
			source := string(generatedFileContent(t, plugin, strings.TrimSuffix(protoPath, ".proto")+".mcp.go"))
			for _, want := range []string{
				"package serverv1",
				"type ServerConfigMCPHandlers struct",
				"type ServerConfigPromptHandler interface",
				"type ServerConfigResourceHandler interface",
				"func NewServerConfigMCPServer(",
				"func NewServerConfigMCPHTTPHandler(",
				"func RegisterServerConfigPrompts(",
				"func RegisterServerConfigResources(",
			} {
				if !strings.Contains(source, want) {
					t.Errorf("stable API missing %q", want)
				}
			}
			legacy := plugin.FilesByPath[protoPath].GoDescriptorIdent.GoName
			for _, suffix := range []string{"MCPHandlers", "PromptHandler", "ResourceHandler"} {
				want := "type " + legacy + suffix + " = ServerConfig" + suffix
				if !strings.Contains(source, want) {
					t.Errorf("compatibility type alias missing %q", want)
				}
			}
			for _, want := range []string{
				"// Deprecated: use NewServerConfigMCPServer.",
				"func New" + legacy + "MCPServer(",
				"return NewServerConfigMCPServer(ctx, handlers)",
				"func New" + legacy + "MCPHTTPHandler(",
				"return NewServerConfigMCPHTTPHandler(server, verifier)",
				"func Register" + legacy + "Prompts(",
				"return RegisterServerConfigPrompts(server, impl, opts...)",
				"func Register" + legacy + "Resources(",
				"return RegisterServerConfigResources(ctx, server, impl, opts...)",
			} {
				if !strings.Contains(source, want) {
					t.Errorf("compatibility wrapper missing %q", want)
				}
			}
		})
	}
}

func TestGoFileAPINameNormalization(t *testing.T) {
	for _, tc := range []struct{ basename, want string }{
		{"server_config", "ServerConfig"},
		{"server-config", "ServerConfig"},
		{"server.config", "ServerConfig"},
		{"HTTP_api", "HTTPApi"},
		{"123_service", "X123Service"},
		{"_service", "Service"},
		{"数字", "X数字"},
		{"___", "File"},
	} {
		t.Run(tc.basename, func(t *testing.T) {
			protoPath := "test/v1/" + tc.basename + ".proto"
			plugin := newTempProtogenPlugin(t, map[string]string{
				protoPath: goNamingProto("test.v1", "example.com/test;testv1", ""),
			}, protoPath)
			if got := goFileAPIName(plugin.FilesByPath[protoPath]); got != tc.want {
				t.Fatalf("goFileAPIName = %q, want %q", got, tc.want)
			}
			if err := Generate(plugin, Options{Language: LanguageGo}); err != nil {
				t.Fatalf("Generate: %v", err)
			}
			// GeneratedFile.Content formats/parses the source, proving names are
			// valid Go identifiers even for legal non-identifier proto basenames.
			generatedFileContent(t, plugin, strings.TrimSuffix(protoPath, ".proto")+".mcp.go")
		})
	}
}

func TestGoFileAPINameCollisionsFailBeforeRendering(t *testing.T) {
	for _, second := range []string{"other/v1/server_config.proto", "other/v1/server-config.proto", "other/v1/serverConfig.proto"} {
		t.Run(second, func(t *testing.T) {
			const first = "test/v1/server_config.proto"
			plugin := newTempProtogenPlugin(t, map[string]string{
				first:  goNamingProto("test.v1", "example.com/shared;sharedv1", ""),
				second: goNamingProto("other.v1", "example.com/shared;sharedv1", ""),
			}, first, second)
			err := Generate(plugin, Options{Language: LanguageGo})
			for _, want := range []string{"Go symbol collision", "ServerConfigMCPHandlers", "example.com/shared", first, second} {
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("Generate = %v, want error containing %q", err, want)
				}
			}
			if got := len(plugin.Response().File); got != 0 {
				t.Fatalf("rendered %d files before detecting collision", got)
			}
		})
	}
}

func TestGoFileAPINamesUseResolvedGoPackageScope(t *testing.T) {
	const first, second = "test/v1/server.proto", "other/v1/server.proto"
	plugin := newTempProtogenPlugin(t, map[string]string{
		first:  goNamingProto("test.v1", "example.com/first;serverv1", ""),
		second: goNamingProto("other.v1", "example.com/second;serverv1", ""),
	}, first, second)
	if err := Generate(plugin, Options{Language: LanguageGo}); err != nil {
		t.Fatalf("distinct import paths with the same package name must not collide: %v", err)
	}
	for _, protoPath := range []string{first, second} {
		source := string(generatedFileContent(t, plugin, strings.TrimSuffix(protoPath, ".proto")+".mcp.go"))
		if !strings.Contains(source, "func NewServerMCPServer(") {
			t.Fatalf("%s missing basename API", protoPath)
		}
	}
}

func TestGoFileAPINamesHonorGoPackageMappings(t *testing.T) {
	const first, second = "test/v1/server.proto", "other/v1/server.proto"
	plugin := newTempProtogenPlugin(t, map[string]string{
		first:  goNamingProto("test.v1", "example.com/first;serverv1", ""),
		second: goNamingProto("other.v1", "example.com/second;serverv1", ""),
	}, first, second)
	plugin.Request.Parameter = proto.String("paths=source_relative,M" + first + "=example.com/shared;sharedv1,M" + second + "=example.com/shared;sharedv1")
	mapped, err := (protogen.Options{}).New(plugin.Request)
	if err != nil {
		t.Fatalf("mapped protogen request: %v", err)
	}
	err = Generate(mapped, Options{Language: LanguageGo})
	if err == nil || !strings.Contains(err.Error(), "Go symbol collision") || !strings.Contains(err.Error(), "example.com/shared") {
		t.Fatalf("Generate = %v, want collision in mapped Go package", err)
	}
}

func TestGoFileAPINamesRejectProtobufSymbolCollisions(t *testing.T) {
	for _, tc := range []struct{ name, declarations, symbol string }{
		{"handlers", "message ServerMCPHandlers {}", "ServerMCPHandlers"},
		{"constructor", "message NewServerMCPServer {}", "NewServerMCPServer"},
		{"HTTP constructor", "message NewServerMCPHTTPHandler {}", "NewServerMCPHTTPHandler"},
		{"enum", "enum ServerMCPHandlers { SERVER_MCP_HANDLERS_NONE = 0; }", "ServerMCPHandlers"},
		{"prompt", `message ServerPromptHandler {} message Guide { option (mcp.options.v1.prompt) = { name: "guide" }; }`, "ServerPromptHandler"},
		{"resource", `message ServerResourceHandler {} message Status { option (mcp.options.v1.resource) = { uri: "status://current" }; }`, "ServerResourceHandler"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const protoPath = "test/v1/server.proto"
			plugin := newTempProtogenPlugin(t, map[string]string{
				protoPath: goNamingProto("test.v1", "example.com/test;testv1", tc.declarations),
			}, protoPath)
			err := Generate(plugin, Options{Language: LanguageGo})
			if err == nil || !strings.Contains(err.Error(), "Go symbol collision") || !strings.Contains(err.Error(), tc.symbol) {
				t.Fatalf("Generate = %v, want collision on %s", err, tc.symbol)
			}
		})
	}
}

func TestGoFileAPINamesRejectImportedProtobufSymbolCollision(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/server.proto": goNamingProto("test.v1", "example.com/test;testv1", `import "test/v1/types.proto";`),
		"test/v1/types.proto":  `syntax = "proto3"; package test.v1; option go_package = "example.com/test;testv1"; message ServerMCPHandlers {}`,
	}, "test/v1/server.proto")
	err := Generate(plugin, Options{Language: LanguageGo})
	if err == nil || !strings.Contains(err.Error(), "ServerMCPHandlers") || !strings.Contains(err.Error(), "test/v1/types.proto") {
		t.Fatalf("Generate = %v, want collision with imported protobuf declaration", err)
	}
}

func TestGoFileAPINamesRejectImportedMCPSymbolCollisions(t *testing.T) {
	for _, tc := range []struct {
		name, targetDeclarations, importedDeclarations, symbol string
		importedServer                                         bool
	}{
		{"server factory", "", "", "ServerMCPHandlers", true},
		{"prompt", `message Guide { option (mcp.options.v1.prompt) = { name: "guide" }; }`, `message ImportedGuide { option (mcp.options.v1.prompt) = { name: "imported_guide" }; }`, "ServerPromptHandler", false},
		{"resource", `message Status { option (mcp.options.v1.resource) = { uri: "status://current" }; }`, `message ImportedStatus { option (mcp.options.v1.resource) = { uri: "status://imported" }; }`, "ServerResourceHandler", false},
		{"implicit Apps factory", "", `message App { option (mcp.options.v1.resource) = { uri: "ui://test/app" mime_type: "text/html;profile=mcp-app" source_file: "assets/app.html" }; }`, "ServerMCPHandlers", false},
		{"implicit tool Apps factory", "", `message Request {} message Response {} service AppAPI { rpc Call(Request) returns (Response) { option (mcp.options.v1.method) = { app_ui: { resource_uri: "ui://test/app" } }; } }`, "ServerMCPHandlers", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			imported := goNamingProto("two", "example.com/shared;shared", tc.importedDeclarations)
			if !tc.importedServer {
				imported = strings.ReplaceAll(imported, `option (mcp.options.v1.server) = { name: "naming-test" };`, "")
			}
			plugin := newTempProtogenPlugin(t, map[string]string{
				"one/server.proto": goNamingProto("one", "example.com/shared;shared", `import "two/server.proto";`+tc.targetDeclarations),
				"two/server.proto": imported,
			}, "one/server.proto")
			err := Generate(plugin, Options{Language: LanguageGo})
			for _, want := range []string{"Go symbol collision", tc.symbol, "one/server.proto", "two/server.proto"} {
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("Generate = %v, want error containing %q", err, want)
				}
			}
			if got := len(plugin.Response().File); got != 0 {
				t.Fatalf("rendered %d files before detecting imported sidecar collision", got)
			}
		})
	}
}

func TestGoFileAPINamesDoNotGenerateOrValidateIrrelevantImports(t *testing.T) {
	for _, tc := range []struct{ name, goPackage, declarations string }{
		{"different Go package", "example.com/other;other", `option (mcp.options.v1.server) = { name: "imported" };`},
		{"same basename without MCP", "example.com/shared;shared", "message Plain {}"},
		{"unsupported imported schema", "example.com/shared;shared", `import "google/protobuf/api.proto"; message ImportedRequest { google.protobuf.Api api = 1; } message ImportedResponse {} service ImportedAPI { rpc Call(ImportedRequest) returns (ImportedResponse); }`},
		{"unsupported imported RPC", "example.com/shared;shared", `message ImportedRequest {} message ImportedResponse {} service SharedAPI { rpc Call(stream ImportedRequest) returns (ImportedResponse); }`},
		{"hidden imported method", "example.com/shared;shared", `message ImportedRequest {} message ImportedResponse {} service SharedAPI { rpc Call(ImportedRequest) returns (ImportedResponse) { option (mcp.options.v1.method) = { hidden: true }; } }`},
		{"unrelated invalid imported resource", "example.com/shared;shared", `message Broken { option (mcp.options.v1.resource) = { mime_type: "text/html; invalid" }; }`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plugin := newTempProtogenPlugin(t, map[string]string{
				"one/server.proto": goNamingProto("one", "example.com/shared;shared", `import "two/server.proto"; message Request {} message Response {} service SharedAPI { rpc Call(Request) returns (Response); }`),
				"two/server.proto": fmt.Sprintf(`syntax = "proto3"; package two; option go_package = %q; import "mcp/options/v1/options.proto"; %s`, tc.goPackage, tc.declarations),
			}, "one/server.proto")
			if err := Generate(plugin, Options{Language: LanguageGo}); err != nil {
				t.Fatalf("irrelevant imported features must not block generation: %v", err)
			}
			files := plugin.Response().File
			if len(files) != 1 || files[0].GetName() != "one/server.mcp.go" {
				t.Fatalf("generated imports or omitted target: %v", files)
			}
		})
	}
}

func TestGoFileAPINamesRejectImportedMCPAndPlainTargetCollision(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"one/types.proto":  `syntax = "proto3"; package one; option go_package = "example.com/shared;shared"; import "two/server.proto"; message ServerMCPHandlers {}`,
		"two/server.proto": goNamingProto("two", "example.com/shared;shared", ""),
	}, "one/types.proto")
	err := Generate(plugin, Options{Language: LanguageGo})
	if err == nil || !strings.Contains(err.Error(), "Go symbol collision") || !strings.Contains(err.Error(), "ServerMCPHandlers") {
		t.Fatalf("Generate = %v, want imported MCP collision with plain generated protobuf type", err)
	}
}
