package codegen

import (
	"strings"
	"testing"
)

const goIconsTestProto = `syntax = "proto3";
package test.v1;
option go_package = "example.com/icons/testv1;testv1";
import "mcp/options/v1/options.proto";

service IconsAPI {
  rpc Ping(PingRequest) returns (PingResponse) {
    option (mcp.options.v1.method) = {
      icons: [{ src: "https://example.com/tool.svg" mime_type: "image/svg+xml" sizes: ["any"] theme: "%s" }]
    };
  }
}

message PingRequest {}
message PingResponse {}

message Status {
  option (mcp.options.v1.resource) = {
    uri: "status://current"
    icons: [{ src: "https://example.com/status.png" sizes: ["48x48", "96x96"] theme: "light" }]
  };
  bool healthy = 1;
}

message Item {
  option (mcp.options.v1.resource) = {
    uri_template: "items://{id}"
    icons: [{ src: "https://example.com/item.png" }]
  };
  string id = 1;
}
`

func generateGoIconsFixture(t *testing.T, theme string) (string, error) {
	t.Helper()
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/icons.proto": strings.Replace(goIconsTestProto, "%s", theme, 1),
	}, "test/v1/icons.proto")
	if err := Generate(plugin, Options{Language: LanguageGo}); err != nil {
		return "", err
	}
	return string(generatedFileContent(t, plugin, "test/v1/icons.mcp.go")), nil
}

func TestGoRendererEmitsIconSizesAndTheme(t *testing.T) {
	got, err := generateGoIconsFixture(t, "dark")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, want := range []string{
		`mcpruntime.Icon{URL: "https://example.com/tool.svg", MIMEType: "image/svg+xml", Sizes: []string{"any"}, Theme: "dark"}`,
		`mcp.Icon{Source: "https://example.com/status.png", MIMEType: "", Sizes: []string{"48x48", "96x96"}, Theme: "light"}`,
		`mcp.Icon{Source: "https://example.com/item.png", MIMEType: ""}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("generated Go is missing %s", want)
		}
	}
	if t.Failed() {
		t.Logf("generated Go:\n%s", got)
	}
}

func TestGoRendererRejectsUnknownIconTheme(t *testing.T) {
	_, err := generateGoIconsFixture(t, "solarized")
	if err == nil || !strings.Contains(err.Error(), `unsupported Go icon theme "solarized"`) {
		t.Fatalf("Generate error = %v, want unsupported Go icon theme", err)
	}
}
