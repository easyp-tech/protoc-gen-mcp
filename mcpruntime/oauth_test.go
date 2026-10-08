package mcpruntime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestOAuthProtectedResourceHandler(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "oauth-test", Version: "v0.0.1"}, nil)
	handler, err := NewOAuthResourceHandler(server, OAuthResourceServer{
		ResourceURL: "https://example.com/mcp",
		AuthorizationServers: []string{"https://login.example.com"},
		Scopes: []string{"read"},
		Verifier: func(ctx context.Context, token string, request *http.Request) (*auth.TokenInfo, error) {
			if token != "valid-token" {
				return nil, auth.ErrInvalidToken
			}
			return &auth.TokenInfo{
				Scopes: []string{"read"},
				Expiration: time.Now().Add(time.Hour),
				UserID: "test-user",
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewOAuthResourceHandler: %v", err)
	}

	metadata := httptest.NewRecorder()
	handler.ServeHTTP(metadata, httptest.NewRequest(http.MethodGet, ProtectedResourceMetadataPath, nil))
	if metadata.Code != http.StatusOK || !strings.Contains(metadata.Body.String(), "login.example.com") {
		t.Fatalf("resource metadata response: status=%d body=%s", metadata.Code, metadata.Body.String())
	}
	unauthenticated := httptest.NewRecorder()
	handler.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "/mcp", nil))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d, want 401", unauthenticated.Code)
	}
	if got := unauthenticated.Header().Get("WWW-Authenticate"); !strings.Contains(got, "resource_metadata=") {
		t.Fatalf("WWW-Authenticate missing resource metadata: %q", got)
	}

	authenticated := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	authenticated.Header.Set("Authorization", "Bearer valid-token")
	recorded := httptest.NewRecorder()
	handler.ServeHTTP(recorded, authenticated)
	if recorded.Code == http.StatusUnauthorized || recorded.Code == http.StatusForbidden {
		t.Fatalf("authenticated request was rejected: status=%d body=%s", recorded.Code, recorded.Body.String())
	}
}

func TestOAuthConfigurationValidation(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "oauth-test", Version: "v0.0.1"}, nil)
	verifier := auth.TokenVerifier(func(context.Context, string, *http.Request) (*auth.TokenInfo, error) {
		return nil, auth.ErrInvalidToken
	})
	for _, cfg := range []OAuthResourceServer{
		{ResourceURL: "http://example.com/mcp", AuthorizationServers: []string{"https://login.example.com"}, Verifier: verifier},
		{ResourceURL: "https://example.com/mcp", AuthorizationServers: nil, Verifier: verifier},
		{ResourceURL: "https://example.com/mcp", AuthorizationServers: []string{"http://insecure.local"}, Verifier: verifier},
		{ResourceURL: "https://example.com/mcp", AuthorizationServers: []string{"https://login.example.com"}, Verifier: verifier, MCPPath: "/"},
	} {
		if _, err := NewOAuthResourceHandler(server, cfg); err == nil {
			t.Errorf("expected invalid OAuth config %+v to fail", cfg)
		}
	}
}
