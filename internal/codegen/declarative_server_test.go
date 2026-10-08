package codegen

import (
	"strings"
	"testing"
)

// These tests verify public protobuf annotation semantics without rewriting
// checked-in generated Go files or depending on a frontend toolchain.
func TestGoServerConfigAndEmbeddedAppGeneration(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/config.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/config;configv1";`,
			`import "mcp/options/v1/options.proto";`,
			`option (mcp.options.v1.server) = {
				name: "declarative-test" version: "v1.0.0"
				apps: { enabled: true }
				oauth: {
					resource_url: "https://mcp.example.com/mcp"
					authorization_servers: "https://auth.example.com"
					scopes: "reports:read" issuer: "https://auth.example.com"
					jwks_uri: "https://auth.example.com/jwks" audience: "https://mcp.example.com/mcp"
				}
			};`,
			`message View {
				option (mcp.options.v1.resource) = {
					uri: "ui://example/view" mime_type: "text/html;profile=mcp-app"
					source_file: "assets/app.html"
					app_ui: {
						csp: { connect_domains: "https://api.example.com" }
						permissions: { clipboard_write: true }
					}
				};
			}`,
			`message CheckRequest { string value = 1; }`,
			`message CheckResponse { bool allowed = 1; }`,
			`service VerifyAPI {
				rpc Check(CheckRequest) returns (CheckResponse) {
					option (mcp.options.v1.method) = {
						required_scopes: "reports:write"
						app_ui: { resource_uri: "ui://example/view" }
					};
				}
			}`,
		}, "\n"),
	}, "test/v1/config.proto")
	model, err := CollectFileModel(plugin.FilesByPath["test/v1/config.proto"], Options{Language:LanguageGo})
	if err != nil { t.Fatalf("collect declarative model: %v", err) }
	if model.ServerConfig == nil || !modelNeedsApps(model) {
		t.Fatalf("server config or Apps inference missing: %+v", model.ServerConfig)
	}
	if err := Generate(plugin, Options{Language:LanguageGo}); err != nil { t.Fatalf("generate: %v",err) }
	source := string(generatedFileContent(t,plugin,"test/v1/config.mcp.go"))
	for _, marker := range []string{
		"RequiredScopes:", "reports:write", "//go:embed assets/app.html",
		"RegisterEmbeddedResource", "AppResourceMetadata", "NewFile_test_v1_config_protoMCPServer",
		"NewFile_test_v1_config_protoMCPHTTPHandler", "NewConfiguredHTTPHandler",
		"JWKSURL:", "https://auth.example.com/jwks",
	} {
		if !strings.Contains(source, marker) { t.Errorf("generated server code missing %q",marker) }
	}
	if strings.Contains(source, "ReadView(ctx") {
		t.Fatal("embedded app should not require a handwritten read handler")
	}
}

func TestGoServerConfigAndSourceFileValidation(t *testing.T) {
	base := []string{
		`syntax = "proto3";`,
		`package test.v1;`,
		`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/config;configv1";`,
		`import "mcp/options/v1/options.proto";`,
	}
	tests := []struct { name, options, resource, errSubstr string }{
		{"bad embed path","",`message Page { option (mcp.options.v1.resource) = {
			uri: "ui://example/view" mime_type: "text/html;profile=mcp-app"
			source_file: "../secret.html"
		}; }`, "source_file"},
		{"glob source","",`message Page { option (mcp.options.v1.resource) = {
			uri: "ui://example/view" mime_type: "text/html;profile=mcp-app"
			source_file: "assets/*.html"
		}; }`, "source_file"},
		{"template source","",`message Page { option (mcp.options.v1.resource) = {
			uri_template: "ui://example/{id}" mime_type: "text/html;profile=mcp-app"
			source_file: "assets/app.html"
		}; }`, "static uri"},
		{"conflicting body","",`message Page { option (mcp.options.v1.resource) = {
			uri: "ui://example/view" mime_type: "text/html;profile=mcp-app"
			source_file: "assets/app.html" content_field: "html"
		}; string html = 1; }`, "mutually exclusive"},
		{"apps disabled",`option (mcp.options.v1.server) = { apps: { enabled: false } };`,
			`message Page { option (mcp.options.v1.resource) = {
				uri: "ui://example/view" mime_type: "text/html;profile=mcp-app"
				source_file: "assets/app.html"
			}; }`, "conflicts"},
		{"wrong MIME for app metadata","",`message Page { option (mcp.options.v1.resource) = {
			uri: "doc://example" mime_type: "application/json"
			app_ui: { permissions: { camera: true } }
		}; string text = 1; }`, "app_ui requires"},
		{"missing JWKS audience",`option (mcp.options.v1.server) = { oauth: {
			resource_url:"https://mcp.example.com/mcp"
			authorization_servers:"https://auth.example.com"
			jwks_uri:"https://auth.example.com/jwks"
			issuer:"https://auth.example.com"
		}};`, "", "issuer and audience"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := append(append([]string{}, base...), tt.options, tt.resource)
			plugin := newTempProtogenPlugin(t, map[string]string{
				"test/v1/config.proto":strings.Join(lines, "\n"),
			}, "test/v1/config.proto")
			_, err := CollectFileModel(plugin.FilesByPath["test/v1/config.proto"],Options{Language:LanguageGo})
			if err == nil || !strings.Contains(err.Error(), tt.errSubstr) {
				t.Fatalf("CollectFileModel error=%v, want %q",err,tt.errSubstr)
			}
		})
	}
}

func TestGoServerConfigOnlyFileGeneration(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/only_server.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/config;configv1";`,
			`import "mcp/options/v1/options.proto";`,
			`option (mcp.options.v1.server) = { name: "server-only" };`,
		},"\n"),
	}, "test/v1/only_server.proto")
	if err := Generate(plugin, Options{Language:LanguageGo}); err != nil {t.Fatalf("config-only generate: %v",err)}
	result := string(generatedFileContent(t,plugin,"test/v1/only_server.mcp.go"))
	for _, needle := range []string{"NewFile_test_v1_only_server_protoMCPServer","NewFile_test_v1_only_server_protoMCPHTTPHandler"} {
		if !strings.Contains(result,needle) {t.Fatalf("missing %q in generated server-only output",needle)}
	}
	for _, extra := range []string{`errors "errors"`, `RegisterSDKProtoTool`, `RegisterOption`} {
		if strings.Contains(result,extra) {t.Fatalf("unused helper %q in server-only output",extra)}
	}
}
