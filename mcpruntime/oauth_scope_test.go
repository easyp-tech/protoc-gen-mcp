package mcpruntime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

func TestOAuthDiscoveryIncludesToolScopesWithoutRequiringThemGlobally(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "scoped", Version: "v1"}, nil)
	for _, name := range []string{"read", "write"} {
		spec := registryTestTool(name)
		spec.RequiredScopes = []string{"reports:" + name}
		if err := RegisterSDKProtoTool(server, spec); err != nil {
			t.Fatal(err)
		}
	}
	handler, err := NewOAuthResourceHandler(server, OAuthResourceServer{
		ResourceURL:          "https://mcp.example.com/mcp",
		AuthorizationServers: []string{"https://auth.example.com"},
		Scopes:               []string{"reports:read"},
		Verifier: func(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
			scopes := []string{"reports:read"}
			if token == "writer" {
				scopes = append(scopes, "reports:write")
			}
			return &auth.TokenInfo{Scopes: scopes, Expiration: time.Now().Add(time.Hour)}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	readScopes := func() []string {
		t.Helper()
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, ProtectedResourceMetadataPath, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("public metadata status = %d", recorder.Code)
		}
		var metadata oauthex.ProtectedResourceMetadata
		if err := json.Unmarshal(recorder.Body.Bytes(), &metadata); err != nil {
			t.Fatal(err)
		}
		return metadata.ScopesSupported
	}
	if got, want := readScopes(), []string{"reports:read", "reports:write"}; !slices.Equal(got, want) {
		t.Fatalf("advertised scopes = %v, want %v", got, want)
	}

	// Additional generated services can be registered after the handler exists.
	// Their tool scopes must appear once, without changing global requirements.
	export := registryTestTool("export")
	export.RequiredScopes = []string{"reports:read", "reports:export", "reports:export"}
	if err := RegisterSDKProtoTool(server, export); err != nil {
		t.Fatal(err)
	}
	export.RequiredScopes[1] = "mutated-after-registration"
	if got, want := readScopes(), []string{"reports:export", "reports:read", "reports:write"}; !slices.Equal(got, want) {
		t.Fatalf("updated advertised scopes = %v, want %v", got, want)
	}

	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	for _, tc := range []struct {
		token, tool string
		wantError   bool
	}{
		{"reader", "read", false},
		{"reader", "write", true},
		{"writer", "write", false},
	} {
		t.Run(tc.token+"/"+tc.tool, func(t *testing.T) {
			client := mcp.NewClient(&mcp.Implementation{Name: "scope-test", Version: "v1"}, nil)
			httpClient := &http.Client{Transport: oauthScopeTestTransport{
				base: http.DefaultTransport, token: tc.token,
			}}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
				Endpoint: httpServer.URL + "/mcp", HTTPClient: httpClient,
			}, nil)
			if err != nil {
				t.Fatalf("connect with server-wide scopes only: %v", err)
			}
			defer session.Close()
			if _, err := session.ListTools(ctx, nil); err != nil {
				t.Fatalf("tools/list with server-wide scopes: %v", err)
			}
			result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tc.tool})
			if err != nil {
				t.Fatalf("tools/call: %v", err)
			}
			if result.IsError != tc.wantError {
				t.Fatalf("tools/call isError = %t, want %t: %+v", result.IsError, tc.wantError, result)
			}
		})
	}
}

type oauthScopeTestTransport struct {
	base  http.RoundTripper
	token string
}

func (t oauthScopeTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(req)
}
