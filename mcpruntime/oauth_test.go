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
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

func TestOAuthProtectedResourceHandler(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "oauth-test", Version: "v0.0.1"}, nil)
	handler, err := NewOAuthResourceHandler(server, OAuthResourceServer{
		ResourceURL:          "https://example.com/mcp",
		AuthorizationServers: []string{"https://login.example.com"},
		Scopes:               []string{"read"},
		Verifier: func(ctx context.Context, token string, request *http.Request) (*auth.TokenInfo, error) {
			if token != "valid-token" {
				return nil, auth.ErrInvalidToken
			}
			return &auth.TokenInfo{
				Scopes:     []string{"read"},
				Expiration: time.Now().Add(time.Hour),
				UserID:     "test-user",
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
		{ResourceURL: "https://example.com/mcp", AuthorizationServers: []string{"https://login.example.com"}, Verifier: verifier, MCPPath: "/.well-known/oauth-protected-resource/mcp"},
	} {
		if _, err := NewOAuthResourceHandler(server, cfg); err == nil {
			t.Errorf("expected invalid OAuth config %+v to fail", cfg)
		}
	}
}

func TestOAuthProtectedResourceMetadataFollowsResourcePath(t *testing.T) {
	verifier := auth.TokenVerifier(func(context.Context, string, *http.Request) (*auth.TokenInfo, error) {
		return nil, auth.ErrInvalidToken
	})
	for _, tc := range []struct {
		name, resourceURL, metadataPath string
		missing                         []string
	}{
		{
			name:         "resource with path",
			resourceURL:  "https://example.com/mcp",
			metadataPath: ProtectedResourceMetadataPath + "/mcp",
			missing:      []string{ProtectedResourceMetadataPath + "/other", ProtectedResourceMetadataPath + "/mcp/extra"},
		},
		{
			name:         "resource with nested path and trailing slash",
			resourceURL:  "https://example.com/tenant/mcp/",
			metadataPath: ProtectedResourceMetadataPath + "/tenant/mcp",
		},
		{
			name:         "resource without path",
			resourceURL:  "https://example.com",
			metadataPath: ProtectedResourceMetadataPath,
			missing:      []string{ProtectedResourceMetadataPath + "/mcp"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := mcp.NewServer(&mcp.Implementation{Name: "oauth-test", Version: "v0.0.1"}, nil)
			handler, err := NewOAuthResourceHandler(server, OAuthResourceServer{
				ResourceURL:          tc.resourceURL,
				AuthorizationServers: []string{"https://login.example.com"},
				Verifier:             verifier,
			})
			if err != nil {
				t.Fatal(err)
			}
			endpoint := httptest.NewServer(handler)
			defer endpoint.Close()

			for _, path := range []string{tc.metadataPath, ProtectedResourceMetadataPath} {
				prm, err := oauthex.GetProtectedResourceMetadata(t.Context(), endpoint.URL+path, tc.resourceURL, endpoint.Client())
				if err != nil {
					t.Fatalf("%s: %v", path, err)
				}
				if len(prm.AuthorizationServers) != 1 || prm.AuthorizationServers[0] != "https://login.example.com" {
					t.Fatalf("%s: authorization servers %v", path, prm.AuthorizationServers)
				}
			}
			for _, path := range tc.missing {
				response, err := endpoint.Client().Get(endpoint.URL + path)
				if err != nil {
					t.Fatal(err)
				}
				response.Body.Close()
				if response.StatusCode != http.StatusNotFound {
					t.Fatalf("%s: status %d, want 404", path, response.StatusCode)
				}
			}

			unauthenticated := httptest.NewRecorder()
			handler.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader("{}")))
			want := `resource_metadata="https://example.com` + tc.metadataPath + `"`
			if got := unauthenticated.Header().Get("WWW-Authenticate"); !strings.Contains(got, want) {
				t.Fatalf("WWW-Authenticate %q does not point at %s", got, want)
			}
		})
	}
}
