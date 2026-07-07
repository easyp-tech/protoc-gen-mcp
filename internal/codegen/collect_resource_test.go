package codegen

import (
	"strings"
	"testing"
)

// collectResourceFixture builds a plugin from a single resources.proto whose
// ServerStatus message carries the given resource option body.
func collectResourceFixture(t *testing.T, resourceOption string) (FileModel, error) {
	t.Helper()
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/resources.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/resources;resourcesv1";`,
			`import "mcp/options/v1/options.proto";`,
			`message ServerStatus {`,
			`  option (mcp.options.v1.resource) = {` + resourceOption + `};`,
			`  bool healthy = 1;`,
			`}`,
			"",
		}, "\n"),
	}, "test/v1/resources.proto")
	file := plugin.FilesByPath["test/v1/resources.proto"]
	if file == nil {
		t.Fatal("resources proto file not found in plugin")
	}
	model, err := CollectFileModel(file, Options{Language: LanguageGo})
	return model, err
}

func TestCollectResources_UriAndUriTemplateMutuallyExclusive(t *testing.T) {
	_, err := collectResourceFixture(t, ` name: "s" uri: "server://status" uri_template: "server://{id}" `)
	if err == nil {
		t.Fatal("expected error when both uri and uri_template are set")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("error = %q, want it to mention 'mutually exclusive'", err)
	}
}

func TestCollectResources_NeitherUriNorTemplate(t *testing.T) {
	_, err := collectResourceFixture(t, ` name: "s" `)
	if err == nil {
		t.Fatal("expected error when neither uri nor uri_template is set")
	}
	if !strings.Contains(err.Error(), "either uri or uri_template must be set") {
		t.Fatalf("error = %q, want it to mention required uri/uri_template", err)
	}
}

func TestCollectResources_TemplateWithoutParams(t *testing.T) {
	_, err := collectResourceFixture(t, ` name: "s" uri_template: "server://status" `)
	if err == nil {
		t.Fatal("expected error when uri_template has no parameters")
	}
	if !strings.Contains(err.Error(), "no parameters") {
		t.Fatalf("error = %q, want it to mention missing parameters", err)
	}
}

func TestCollectResources_TemplateWithInvalidParam(t *testing.T) {
	_, err := collectResourceFixture(t, ` name: "s" uri_template: "server://{123bad}" `)
	if err == nil {
		t.Fatal("expected error when uri_template parameter identifier is invalid")
	}
	if !strings.Contains(err.Error(), "invalid parameter") {
		t.Fatalf("error = %q, want it to mention invalid parameter", err)
	}
}

func TestCollectResources_StaticAndTemplateRecognized(t *testing.T) {
	plugin := newTempProtogenPlugin(t, map[string]string{
		"test/v1/resources.proto": strings.Join([]string{
			`syntax = "proto3";`,
			`package test.v1;`,
			`option go_package = "github.com/easyp-tech/protoc-gen-mcp/internal/codegen/testdata/resources;resourcesv1";`,
			`import "mcp/options/v1/options.proto";`,
			`message ServerStatus {`,
			`  option (mcp.options.v1.resource) = { name: "server_status" uri: "server://status" };`,
			`  bool healthy = 1;`,
			`}`,
			`message UserProfile {`,
			`  option (mcp.options.v1.resource) = { name: "user_profile" uri_template: "users://{user_id}/profile" };`,
			`  string user_id = 1;`,
			`}`,
			`message NotAResource {`,
			`  string data = 1;`,
			`}`,
			"",
		}, "\n"),
	}, "test/v1/resources.proto")
	file := plugin.FilesByPath["test/v1/resources.proto"]
	if file == nil {
		t.Fatal("resources proto file not found in plugin")
	}
	model, err := CollectFileModel(file, Options{Language: LanguageGo})
	if err != nil {
		t.Fatalf("CollectFileModel: %v", err)
	}
	if len(model.Resources) != 2 {
		t.Fatalf("resource count = %d, want 2", len(model.Resources))
	}
	if model.Resources[0].IsTemplate {
		t.Fatalf("Resources[0] (ServerStatus) should be static")
	}
	if !model.Resources[1].IsTemplate {
		t.Fatalf("Resources[1] (UserProfile) should be a template")
	}
	if got := model.Resources[1].Params; len(got) != 1 || got[0].Name != "user_id" {
		t.Fatalf("template params = %+v, want [user_id]", got)
	}
}
