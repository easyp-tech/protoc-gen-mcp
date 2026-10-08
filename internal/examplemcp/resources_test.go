package examplemcp_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/easyp-tech/protoc-gen-mcp/internal/examplemcp"
	examplev1 "github.com/easyp-tech/protoc-gen-mcp/internal/testproto/example/v1"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newResourcesClient(t *testing.T) *mcp.ClientSession {
	t.Helper()
	server, err := examplemcp.NewResourcesServer(context.Background())
	if err != nil {
		t.Fatalf("NewResourcesServer: %v", err)
	}
	return connectResourcesClient(t, server)
}

func connectResourcesClient(t *testing.T, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "resource-test", Version: "v0.0.1"}, nil)
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })
	return clientSession
}

func TestResourcesPromptsRoundTrip(t *testing.T) {
	session := newResourcesClient(t)
	ctx := context.Background()

	initialized := session.InitializeResult()
	if initialized == nil || initialized.Capabilities == nil {
		t.Fatal("server did not advertise capabilities")
	}
	if _, ok := initialized.Capabilities.Extensions["io.modelcontextprotocol/ui"]; !ok {
		t.Fatalf("protobuf-configured server omitted MCP Apps extension: %+v", initialized.Capabilities)
	}

	resources, err := session.ListResources(ctx, nil)
	if err != nil {
		t.Fatalf("resources/list: %v", err)
	}
	if len(resources.Resources) == 0 {
		t.Fatal("resources/list returned no resources")
	}
	var foundStatic bool
	for _, resource := range resources.Resources {
		foundStatic = foundStatic || resource.URI == "server://status"
	}
	if !foundStatic {
		t.Fatalf("resources/list missing server://status: %+v", resources.Resources)
	}

	templates, err := session.ListResourceTemplates(ctx, nil)
	if err != nil {
		t.Fatalf("resources/templates/list: %v", err)
	}
	for _, expected := range []string{"users://{user_id}/profile", "projects://{project_id}/documents/{document_id}"} {
		var found bool
		for _, template := range templates.ResourceTemplates {
			found = found || template.URITemplate == expected
		}
		if !found {
			t.Errorf("templates/list missing %q", expected)
		}
	}

	static, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "server://status"})
	if err != nil {
		t.Fatalf("static resources/read: %v", err)
	}
	if len(static.Contents) != 1 || !strings.Contains(static.Contents[0].Text, `"healthy":true`) {
		t.Fatalf("unexpected static resource: %+v", static.Contents)
	}

	profile, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "users://ada/profile"})
	if err != nil {
		t.Fatalf("templated resources/read: %v", err)
	}
	if len(profile.Contents) != 1 || !strings.Contains(profile.Contents[0].Text, `"userId":"ada"`) {
		t.Fatalf("unexpected templated resource: %+v", profile.Contents)
	}

	skill, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "skill://example/SKILL.md"})
	if err != nil {
		t.Fatalf("skill resources/read: %v", err)
	}
	if len(skill.Contents) != 1 || skill.Contents[0].MIMEType != "text/markdown" ||
		!strings.Contains(skill.Contents[0].Text, "## Instructions") ||
		!strings.HasPrefix(skill.Contents[0].Text, "# Example MCP Skill") {
		t.Fatalf("SKILL.md was not delivered as raw Markdown: %+v", skill.Contents)
	}

	plain, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "docs://example/intro"})
	if err != nil {
		t.Fatalf("plain text resources/read: %v", err)
	}
	if len(plain.Contents) != 1 || plain.Contents[0].MIMEType != "text/plain" ||
		plain.Contents[0].Text != "document: intro" {
		t.Fatalf("plain text was not served unescaped: %+v", plain.Contents)
	}

	binary, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "blob://example/attachment"})
	if err != nil {
		t.Fatalf("binary resources/read: %v", err)
	}
	if len(binary.Contents) != 1 || binary.Contents[0].MIMEType != "application/octet-stream" ||
		!bytes.Equal(binary.Contents[0].Blob, []byte{0, 1, 2, 255}) {
		t.Fatalf("binary resource was not served as blob: %+v", binary.Contents)
	}

	html, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "ui://example/report"})
	if err != nil {
		t.Fatalf("UI resources/read: %v", err)
	}
	if len(html.Contents) != 1 || html.Contents[0].MIMEType != "text/html;profile=mcp-app" ||
		!strings.HasPrefix(html.Contents[0].Text, "<!doctype html>") {
		t.Fatalf("MCP Apps HTML resource was not served as HTML: %+v", html.Contents)
	}
	if !strings.Contains(html.Contents[0].Text, "Example MCP App") {
		t.Fatalf("embedded frontend asset missing from generated resource: %s", html.Contents[0].Text)
	}
	appMeta, ok := html.Contents[0].Meta["ui"].(map[string]any)
	if !ok {
		t.Fatalf("missing content _meta.ui: %+v", html.Contents[0].Meta)
	}
	csp, ok := appMeta["csp"].(map[string]any)
	if !ok {
		t.Fatalf("missing CSP metadata: %+v", appMeta)
	}
	connect, ok := csp["connectDomains"].([]any)
	if !ok || len(connect) != 1 || connect[0] != "https://api.example.com" {
		t.Fatalf("CSP connectDomains mismatch: %+v", csp)
	}
	permissions, ok := appMeta["permissions"].(map[string]any)
	if !ok || permissions["clipboardWrite"] == nil || appMeta["prefersBorder"] != true {
		t.Fatalf("permissions/border mismatch: %+v", appMeta)
	}
	listed := false
	for _, resource := range resources.Resources {
		if resource.URI == "ui://example/report" {
			_, listed = resource.Meta["ui"]
		}
	}
	if !listed {
		t.Fatal("UI resource metadata missing from resources/list")
	}

	prompts, err := session.ListPrompts(ctx, nil)
	if err != nil {
		t.Fatalf("prompts/list: %v", err)
	}
	for _, expected := range []string{"code_review", "summarize", "explain_error"} {
		var found bool
		for _, prompt := range prompts.Prompts {
			found = found || prompt.Name == expected
		}
		if !found {
			t.Errorf("prompts/list missing %q", expected)
		}
	}
	result, err := session.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      "code_review",
		Arguments: map[string]string{"code": "print(1)", "language": "python"},
	})
	if err != nil {
		t.Fatalf("prompts/get: %v", err)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("prompts/get messages = %d, want 1", len(result.Messages))
	}
	txt, ok := result.Messages[0].Content.(*mcp.TextContent)
	if !ok || !strings.Contains(txt.Text, "Review this python code: print(1)") {
		t.Fatalf("unexpected prompt content: %+v", result.Messages[0].Content)
	}
}

// TestGeneratedAppUIFromProto verifies that an annotated unary RPC advertises
// the MCP Apps UI resource without losing its generated JSON Schema.
func TestGeneratedAppUIFromProto(t *testing.T) {
	server, err := examplemcp.NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "ui-test", Version: "v0.0.1"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "example_CreateReport" {
			continue
		}
		ui, ok := tool.Meta["ui"].(map[string]any)
		if !ok || ui["resourceUri"] != "ui://example/report" {
			t.Fatalf("generated tool UI metadata = %+v", tool.Meta)
		}
		if tool.InputSchema == nil {
			t.Fatal("generated tool lost input schema")
		}
		return
	}
	t.Fatal("generated example_CreateReport tool was not registered")
}

// TestAppUIUsesProtobufResource verifies tool _meta.ui references an HTML
// resource declared and registered through generated protobuf resource options.
func TestAppUIUsesProtobufResource(t *testing.T) {
	ctx := context.Background()
	server, err := examplemcp.NewResourcesServer(ctx)
	if err != nil {
		t.Fatalf("NewResourcesServer: %v", err)
	}
	if err := examplev1.RegisterExampleAPITools(server, examplemcp.Handler{}); err != nil {
		t.Fatalf("register example tools: %v", err)
	}
	session := connectResourcesClient(t, server)
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	var resourceURI string
	for _, tool := range tools.Tools {
		if tool.Name != "example_CreateReport" {
			continue
		}
		ui, ok := tool.Meta["ui"].(map[string]any)
		if !ok {
			t.Fatalf("tool UI metadata missing: %+v", tool.Meta)
		}
		resourceURI, _ = ui["resourceUri"].(string)
	}
	if resourceURI != "ui://example/report" {
		t.Fatalf("generated tool resourceURI=%q", resourceURI)
	}
	html, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: resourceURI})
	if err != nil || len(html.Contents) != 1 || html.Contents[0].MIMEType != "text/html;profile=mcp-app" {
		t.Fatalf("tool UI does not resolve to generated HTML resource: %+v, err=%v", html, err)
	}
}

func TestPromptsGetMissingRequiredArg(t *testing.T) {
	session := newResourcesClient(t)
	_, err := session.GetPrompt(context.Background(), &mcp.GetPromptParams{
		Name:      "code_review",
		Arguments: map[string]string{"code": "print(1)"},
	})
	if err == nil {
		t.Fatal("expected missing required argument to fail")
	}
}
