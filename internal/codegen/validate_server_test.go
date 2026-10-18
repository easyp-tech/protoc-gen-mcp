package codegen

import (
	"fmt"
	"strings"
	"testing"

	mcpoptionsv1 "github.com/easyp-tech/protoc-gen-mcp/mcp/options/v1"
	"google.golang.org/protobuf/proto"
)

func TestModelNeedsAppsParsesResourceMIMEType(t *testing.T) {
	for _, tc := range []struct {
		name, mimeType string
		want           bool
	}{
		{"canonical", "text/html;profile=mcp-app", true},
		{"parameter spacing", "text/html; profile=mcp-app", true},
		{"quoted profile", `text/html; profile="mcp-app"`, true},
		{"case insensitive media type", "TEXT/HTML; PROFILE=mcp-app", true},
		{"extra parameter first", "text/html; charset=utf-8; profile=mcp-app", true},
		{"extra parameter last", "text/html; profile=mcp-app; charset=utf-8", true},
		{"plain HTML", "text/html", false},
		{"different profile", "text/html;profile=other", false},
		{"case sensitive profile value", "text/html;profile=MCP-APP", false},
		{"different media type", "text/plain;profile=mcp-app", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := FileModel{Resources: []ResourceModel{{MIMEType: tc.mimeType}}}
			if err := validateServerModel(model); err != nil {
				t.Fatalf("valid MIME type rejected: %v", err)
			}
			if got := modelNeedsApps(model); got != tc.want {
				t.Fatalf("modelNeedsApps(%q) = %t, want %t", tc.mimeType, got, tc.want)
			}
			// Validation and automatic capability detection must agree on which
			// resources conflict with an explicitly disabled Apps configuration.
			model.ServerConfig = &mcpoptionsv1.ServerConfig{
				Apps: &mcpoptionsv1.AppsConfig{Enabled: proto.Bool(false)},
			}
			err := validateServerModel(model)
			if (err != nil) != tc.want {
				t.Fatalf("disabled Apps validation error = %v, want conflict = %t", err, tc.want)
			}
		})
	}
}

func TestGoAppResourceMIMETypeInfersServerFactory(t *testing.T) {
	for _, mimeType := range []string{
		"text/html; profile=mcp-app",
		`text/html; charset=utf-8; profile="mcp-app"`,
	} {
		t.Run(mimeType, func(t *testing.T) {
			plugin := newTempProtogenPlugin(t, map[string]string{
				"test/v1/app.proto": fmt.Sprintf(`syntax = "proto3";
package test.v1;
option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/app;appv1";
import "mcp/options/v1/options.proto";
message App {
  option (mcp.options.v1.resource) = {
    uri: "ui://example/app"
    mime_type: %q
    source_file: "assets/app.html"
  };
}`, mimeType),
			}, "test/v1/app.proto")
			if err := Generate(plugin, Options{Language: LanguageGo}); err != nil {
				t.Fatalf("generate: %v", err)
			}
			source := string(generatedFileContent(t, plugin, "test/v1/app.mcp.go"))
			for _, marker := range []string{
				"func NewFile_test_v1_app_protoMCPServer(",
				`NewConfiguredServer("app.proto", "v0.0.1", true)`,
			} {
				if !strings.Contains(source, marker) {
					t.Errorf("generated App server is missing %q", marker)
				}
			}
		})
	}
}

func TestValidateServerOAuthRejectsInvalidRuntimeConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name, issuer, path, wantError string
	}{
		{"default path", "https://auth.example.com", "", ""},
		{"custom path", "https://auth.example.com", "/custom/mcp", ""},
		{"root path", "https://auth.example.com", "/", "invalid OAuth mcp_path"},
		{"metadata path", "https://auth.example.com", "/.well-known/oauth-protected-resource", "invalid OAuth mcp_path"},
		{"relative path", "https://auth.example.com", "mcp", "invalid OAuth mcp_path"},
		{"query path", "https://auth.example.com", "/mcp?x=1", "invalid OAuth mcp_path"},
		{"HTTP issuer", "http://auth.example.com", "", "issuer must be an absolute HTTPS URL"},
		{"relative issuer", "auth.example.com", "", "issuer must be an absolute HTTPS URL"},
		{"issuer with user info", "https://user@auth.example.com", "", "issuer must be an absolute HTTPS URL"},
		{"issuer with fragment", "https://auth.example.com/#fragment", "", "issuer must be an absolute HTTPS URL"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := FileModel{ServerConfig: &mcpoptionsv1.ServerConfig{
				Oauth: &mcpoptionsv1.OAuthConfig{
					ResourceUrl:          "https://mcp.example.com/mcp",
					AuthorizationServers: []string{"https://auth.example.com"},
					JwksUri:              "https://auth.example.com/jwks",
					Audience:             "https://mcp.example.com/mcp",
					Issuer:               tc.issuer,
					McpPath:              tc.path,
				},
			}}
			err := validateServerModel(model)
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("valid configuration rejected: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("validation error = %v, want %q", err, tc.wantError)
			}
		})
	}
}
