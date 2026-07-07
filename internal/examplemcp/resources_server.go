package examplemcp

import (
	"context"
	"fmt"

	"github.com/easyp-tech/protoc-gen-mcp/mcpruntime"

	promptsv1 "github.com/easyp-tech/protoc-gen-mcp/internal/testproto/prompts/v1"
	resourcesv1 "github.com/easyp-tech/protoc-gen-mcp/internal/testproto/resources/v1"
)

// NewResourcesServer returns a ready-to-run MCP server backed by the generated
// protobuf resources and prompts. It exists so that the generated resource
// registration code (resources.mcp.go) is actually compiled and exercised — the
// text-only golden test does not compile it.
func NewResourcesServer(ctx context.Context) (*mcpruntime.Server, error) {
	server := mcpruntime.NewServer("protoc-gen-mcp-resources-server", "v0.0.1")

	if err := resourcesv1.RegisterFile_internal_testproto_resources_v1_resources_protoResources(ctx, server, ResourcesHandler{}); err != nil {
		return nil, err
	}
	if err := promptsv1.RegisterFile_internal_testproto_prompts_v1_prompts_protoPrompts(server, PromptsHandler{}); err != nil {
		return nil, err
	}

	return server, nil
}

// ResourcesHandler implements the generated resource handler interface with
// deterministic responses for integration checks.
type ResourcesHandler struct{}

// ReadServerStatus returns a static server-status resource.
func (ResourcesHandler) ReadServerStatus(_ context.Context) (*resourcesv1.ServerStatus, error) {
	return &resourcesv1.ServerStatus{
		Healthy:       true,
		UptimeSeconds: 123,
		Version:       "v0.0.1",
	}, nil
}

// ListUserProfiles advertises the known user-profile instances.
func (ResourcesHandler) ListUserProfiles(_ context.Context) ([]mcpruntime.Resource, error) {
	return []mcpruntime.Resource{
		{Name: "user_profile", URI: "users://ada/profile", Description: "User profile information"},
	}, nil
}

// ReadUserProfile resolves a user-profile template resource by user id.
func (ResourcesHandler) ReadUserProfile(_ context.Context, userID string) (*resourcesv1.UserProfile, error) {
	return &resourcesv1.UserProfile{
		UserId:      userID,
		DisplayName: "Ada Lovelace",
		Email:       userID + "@example.com",
		Active:      true,
	}, nil
}

// ListDocuments advertises the known document instances.
func (ResourcesHandler) ListDocuments(_ context.Context) ([]mcpruntime.Resource, error) {
	return nil, nil
}

// ReadDocument resolves a document template resource by project and document id.
func (ResourcesHandler) ReadDocument(_ context.Context, projectID, documentID string) (*resourcesv1.Document, error) {
	return &resourcesv1.Document{
		Title:  fmt.Sprintf("doc %s in %s", documentID, projectID),
		Body:   "body",
		Author: "Ada",
	}, nil
}

// PromptsHandler implements the generated prompt handler interface with
// deterministic responses for integration checks.
type PromptsHandler struct{}

// CodeReview renders a code-review prompt from its arguments.
func (PromptsHandler) CodeReview(_ context.Context, req *promptsv1.CodeReview) ([]mcpruntime.PromptMessage, error) {
	return []mcpruntime.PromptMessage{{
		Role:    mcpruntime.RoleUser,
		Content: &mcpruntime.TextContent{Type: "text", Text: fmt.Sprintf("Review this %s code: %s", req.GetLanguage(), req.GetCode())},
	}}, nil
}

// Summarize renders a summarization prompt from its arguments.
func (PromptsHandler) Summarize(_ context.Context, req *promptsv1.Summarize) ([]mcpruntime.PromptMessage, error) {
	return []mcpruntime.PromptMessage{{
		Role:    mcpruntime.RoleUser,
		Content: &mcpruntime.TextContent{Type: "text", Text: "Summarize: " + req.GetContent()},
	}}, nil
}

// ExplainError renders an error-explanation prompt from its arguments.
func (PromptsHandler) ExplainError(_ context.Context, req *promptsv1.ExplainError) ([]mcpruntime.PromptMessage, error) {
	return []mcpruntime.PromptMessage{{
		Role:    mcpruntime.RoleUser,
		Content: &mcpruntime.TextContent{Type: "text", Text: "Explain: " + req.GetErrorMessage()},
	}}, nil
}
