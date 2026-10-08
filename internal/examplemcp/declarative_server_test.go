package examplemcp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	configv1 "github.com/easyp-tech/protoc-gen-mcp/internal/testproto/config/v1"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type secureAPIHandler struct{}

func (secureAPIHandler) CheckAccess(_ context.Context, req *configv1.CheckAccessRequest) (*configv1.CheckAccessResponse, error) {
	return &configv1.CheckAccessResponse{Allowed: req.GetLabel() != ""}, nil
}

func TestGeneratedMCPServerAndOAuthHTTPFactory(t *testing.T) {
	ctx := context.Background()
	server, err := configv1.NewFile_internal_testproto_config_v1_server_config_protoMCPServer(ctx,
		configv1.File_internal_testproto_config_v1_server_config_protoMCPHandlers{
			SecureAPI: secureAPIHandler{},
		})
	if err != nil {
		t.Fatalf("generated server: %v", err)
	}
	transportServer, transportClient := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, transportServer, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "config-test", Version: "v0.0.1"}, nil)
	cs, err := client.Connect(ctx, transportClient, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 1 || tools.Tools[0].Name != "secure_check_access" {
		t.Fatalf("scoped tool listing: %+v, err=%v", tools, err)
	}
	// An in-memory unauthenticated call cannot bypass generated scope policy.
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "secure_check_access", Arguments: map[string]any{"label": "x"}})
	if err != nil || !result.IsError || len(result.Content) == 0 {
		t.Fatalf("missing scope was not rejected: result=%+v err=%v", result, err)
	}
	if content, ok := result.Content[0].(*mcp.TextContent); !ok || !strings.Contains(content.Text, "permission denied") {
		t.Fatalf("missing scope error is not explicit: %+v", result.Content)
	}

	handler, err := configv1.NewFile_internal_testproto_config_v1_server_config_protoMCPHTTPHandler(server,
		func(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
			if token != "valid-token" && token != "read-only" {
				return nil, auth.ErrInvalidToken
			}
			scopes := []string{"reports:read"}
			if token == "valid-token" {
				scopes = append(scopes, "reports:write")
			}
			return &auth.TokenInfo{UserID: "test", Scopes: scopes, Expiration: time.Now().Add(time.Hour)}, nil
		})
	if err != nil {
		t.Fatalf("configured OAuth HTTP handler: %v", err)
	}
	meta := httptest.NewRecorder()
	handler.ServeHTTP(meta, httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil))
	if meta.Code != 200 {
		t.Fatalf("discovery status %d: %s", meta.Code, meta.Body.String())
	}
	var metadata map[string]any
	if err := json.Unmarshal(meta.Body.Bytes(), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["resource"] != "https://mcp.example.com/mcp" {
		t.Fatalf("wrong resource: %+v", metadata)
	}
	unauth := httptest.NewRecorder()
	handler.ServeHTTP(unauth, httptest.NewRequest(http.MethodGet, "/mcp", nil))
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous request status=%d", unauth.Code)
	}
	valid := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	valid.Header.Set("Authorization", "Bearer valid-token")
	ok := httptest.NewRecorder()
	handler.ServeHTTP(ok, valid)
	if ok.Code == http.StatusUnauthorized || ok.Code == http.StatusForbidden {
		t.Fatalf("valid scoped token rejected: %d: %s", ok.Code, ok.Body.String())
	}
	if _, err := configv1.NewFile_internal_testproto_config_v1_server_config_protoMCPHTTPHandler(server, nil); err != nil {
		t.Fatalf("generated JWKS verifier setup: %v", err)
	}
	endpoint := httptest.NewServer(handler)
	defer endpoint.Close()
	for _, tc := range []struct {
		token   string
		allowed bool
	}{
		{"read-only", false},
		{"valid-token", true},
	} {
		t.Run(tc.token, func(t *testing.T) {
			c := mcp.NewClient(&mcp.Implementation{Name: "scoped-http-test", Version: "v0.0.1"}, nil)
			transport := &mcp.StreamableClientTransport{
				Endpoint:             endpoint.URL + "/mcp",
				HTTPClient:           &http.Client{Timeout: 5 * time.Second, Transport: bearerTransport{token: tc.token, base: http.DefaultTransport}},
				DisableStandaloneSSE: true,
			}
			session, err := c.Connect(ctx, transport, nil)
			if err != nil {
				t.Fatalf("HTTP MCP initialize: %v", err)
			}
			defer session.Close()
			response, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name: "secure_check_access", Arguments: map[string]any{"label": "ok"},
			})
			if err != nil {
				t.Fatalf("HTTP tool call: %v", err)
			}
			if response.IsError == tc.allowed {
				t.Fatalf("tool result isError=%t, expected allowed=%t", response.IsError, tc.allowed)
			}
		})
	}
}

type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	copied := req.Clone(req.Context())
	copied.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(copied)
}
