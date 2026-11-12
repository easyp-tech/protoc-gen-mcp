package examplemcp_test

import (
	"context"
	"testing"

	"github.com/easyp-tech/protoc-gen-mcp/internal/examplemcp"
	examplev1 "github.com/easyp-tech/protoc-gen-mcp/internal/testproto/example/v1"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// This test also runs with checked-in .golden bytes overlaid on the fixture
// packages, against both the repository pin and the latest published Go SDK.
func TestGeneratedAnnotationsSDKCompatibility(t *testing.T) {
	ctx := context.Background()
	server, err := examplemcp.NewResourcesServer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := examplev1.RegisterExampleAPITools(server, examplemcp.Handler{}); err != nil {
		t.Fatal(err)
	}
	session := connectResourcesClient(t, server)
	resources, err := session.ListResources(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var status *mcp.Resource
	for _, resource := range resources.Resources {
		if resource.URI == "server://status" {
			status = resource
		}
	}
	if status == nil || status.Annotations == nil {
		t.Fatalf("generated resource annotations missing: %+v", status)
	}
	if status.Annotations.Priority != 0.8 || len(status.Annotations.Audience) != 2 ||
		status.Annotations.Audience[0] != "user" || status.Annotations.Audience[1] != "assistant" {
		t.Fatalf("generated resource annotations changed: %+v", status.Annotations)
	}

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var health *mcp.Tool
	for _, tool := range tools.Tools {
		if tool.Name == "example_Health" {
			health = tool
		}
	}
	if health == nil || health.Annotations == nil || !health.Annotations.ReadOnlyHint ||
		!health.Annotations.IdempotentHint {
		t.Fatalf("generated tool annotations changed: %+v", health)
	}

	prompts, err := session.ListPrompts(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, prompt := range prompts.Prompts {
		if prompt.Name != "code_review" {
			continue
		}
		if len(prompt.Icons) != 1 || prompt.Icons[0].Source != "https://example.com/review.svg" {
			t.Fatalf("generated prompt icons changed: %+v", prompt.Icons)
		}
		if len(prompt.Arguments) != 3 || !prompt.Arguments[0].Required || prompt.Arguments[2].Required {
			t.Fatalf("generated prompt arguments changed: %+v", prompt.Arguments)
		}
		return
	}
	t.Fatal("generated code_review prompt missing")
}
