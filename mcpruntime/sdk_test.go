package mcpruntime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/types/known/emptypb"
)

func testSDKServerClient(t *testing.T, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatalf("connect server: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-sdk-test", Version: "v0.0.1"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestSDKProtoToolAndAppsRoundTrip(t *testing.T) {
	server := mcp.NewServer(
		&mcp.Implementation{Name: "sdk-test", Version: "v0.0.1"},
		&mcp.ServerOptions{Capabilities: AppCapabilities()},
	)
	spec := ToolSpec[*emptypb.Empty, *emptypb.Empty]{
		Name: "hello",
		Namespace: "demo",
		InputSchemaJSON:  `{"type":"object"}`,
		OutputSchemaJSON: `{"type":"object"}`,
		NewRequest: func() *emptypb.Empty { return &emptypb.Empty{} },
		NewResponse: func() *emptypb.Empty { return &emptypb.Empty{} },
		Handler: func(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
			return &emptypb.Empty{}, nil
		},
	}
	if err := RegisterSDKProtoTool(server, spec, WithAppUI("hello", "ui://demo/hello", "model", "app")); err != nil {
		t.Fatalf("register SDK proto tool: %v", err)
	}
	if err := RegisterAppResource(server, AppResource{
		URI: "ui://demo/hello", Name: "Hello", HTML: "<html><body>Hello</body></html>",
	}); err != nil {
		t.Fatalf("register app resource: %v", err)
	}
	if err := RegisterSDKProtoTool(server, spec); err == nil {
		t.Fatal("duplicate registration unexpectedly succeeded")
	}

	session := testSDKServerClient(t, server)
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "demo_hello" {
		t.Fatalf("tools = %+v, want demo_hello", tools.Tools)
	}
	meta, err := json.Marshal(tools.Tools[0].Meta)
	if err != nil || !strings.Contains(string(meta), "ui://demo/hello") {
		t.Fatalf("tool UI _meta missing: %s, %v", meta, err)
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "demo_hello", Arguments: map[string]any{}})
	if err != nil || result.IsError {
		t.Fatalf("tools/call: result=%+v error=%v", result, err)
	}
	resource, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "ui://demo/hello"})
	if err != nil {
		t.Fatalf("resources/read: %v", err)
	}
	if len(resource.Contents) != 1 || resource.Contents[0].MIMEType != AppHTMLMIMEType ||
		!strings.Contains(resource.Contents[0].Text, "Hello") {
		t.Fatalf("unexpected UI contents: %+v", resource.Contents)
	}
}

func TestSDKProtoToolInputValidationAndApplicationError(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "sdk-validation", Version: "v0.0.1"}, nil)
	err := RegisterSDKProtoTool(server, ToolSpec[*emptypb.Empty, *emptypb.Empty]{
		Name: "failure",
		InputSchemaJSON: `{"type":"object","additionalProperties":false}`,
		OutputSchemaJSON: `{"type":"object"}`,
		NewRequest: func() *emptypb.Empty { return &emptypb.Empty{} },
		NewResponse: func() *emptypb.Empty { return &emptypb.Empty{} },
		Handler: func(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
			return nil, errors.New("business error")
		},
	})
	if err != nil {
		t.Fatalf("register SDK proto tool: %v", err)
	}
	session := testSDKServerClient(t, server)

	invalid, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "failure", Arguments: map[string]any{"unknown": 1},
	})
	if err != nil || !invalid.IsError {
		t.Fatalf("invalid input: result=%+v error=%v", invalid, err)
	}
	failed, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "failure", Arguments: map[string]any{},
	})
	if err != nil || !failed.IsError {
		t.Fatalf("application failure: result=%+v error=%v", failed, err)
	}
}

func TestAppValidation(t *testing.T) {
	for _, uri := range []string{"", "https://demo.example/app", "ui://", "ui://demo/app#fragment"} {
		if err := validateAppURI(uri); err == nil {
			t.Errorf("expected app URI %q to be rejected", uri)
		}
	}
	if err := validateAppUI(AppUI{ResourceURI: "ui://demo/app", Visibility: []string{"tool"}}); err == nil {
		t.Fatal("invalid visibility accepted")
	}
	if err := validateAppUI(AppUI{ResourceURI: "ui://demo/app", Visibility: []string{"model", "model"}}); err == nil {
		t.Fatal("duplicate visibility accepted")
	}
	caps := AppCapabilities()
	if _, ok := caps.Extensions[AppExtensionID]; !ok {
		t.Fatal("app extension not advertised")
	}
}
