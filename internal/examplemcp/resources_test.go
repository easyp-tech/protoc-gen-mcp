package examplemcp_test

import (
	"context"
	"strings"
	"testing"

	"github.com/easyp-tech/protoc-gen-mcp/internal/examplemcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newResourcesClient(t *testing.T) *mcp.ClientSession {
	t.Helper()
	server, err := examplemcp.NewResourcesServer(context.Background())
	if err != nil {
		t.Fatalf("NewResourcesServer: %v", err)
	}
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
		Name: "code_review",
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

func TestPromptsGetMissingRequiredArg(t *testing.T) {
	session := newResourcesClient(t)
	_, err := session.GetPrompt(context.Background(), &mcp.GetPromptParams{
		Name: "code_review",
		Arguments: map[string]string{"code": "print(1)"},
	})
	if err == nil {
		t.Fatal("expected missing required argument to fail")
	}
}
