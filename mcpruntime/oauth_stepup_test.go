package mcpruntime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/types/known/emptypb"
)

const stepUpChallenge = `Bearer error="insufficient_scope", scope="mcp:access reports:write", resource_metadata="https://mcp.example.com/.well-known/oauth-protected-resource/mcp"`

func newStepUpTestHandler(t *testing.T, calls *atomic.Int32) http.Handler {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "step-up", Version: "v1"}, nil)
	for _, name := range []string{"read", "write"} {
		spec := registryTestTool(name)
		spec.RequiredScopes = []string{"reports:" + name}
		spec.Handler = func(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
			calls.Add(1)
			return &emptypb.Empty{}, nil
		}
		if err := RegisterSDKProtoTool(server, spec); err != nil {
			t.Fatal(err)
		}
	}
	global := []string{"mcp:access", "mcp:access"}
	handler, err := NewOAuthResourceHandler(server, OAuthResourceServer{
		ResourceURL: "https://mcp.example.com/mcp", AuthorizationServers: []string{"https://auth.example.com"},
		Scopes: global,
		Verifier: func(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
			if token == "invalid" {
				return nil, auth.ErrInvalidToken
			}
			info := &auth.TokenInfo{Scopes: []string{"mcp:access", "reports:read"}, Expiration: time.Now().Add(time.Hour)}
			if token == "writer" {
				info.Scopes = append(info.Scopes, "reports:write")
			}
			if token == "no-global" {
				info.Scopes = []string{"reports:write"}
			}
			if token == "expired" {
				info.Expiration = time.Now().Add(-time.Hour)
			}
			return info, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	global[0] = "changed-by-caller" // The HTTP gate must keep its own snapshot.
	return handler
}

func TestOAuthScopeChallengesBeforeDispatch(t *testing.T) {
	var calls atomic.Int32
	handler := newStepUpTestHandler(t, &calls)
	for _, tc := range []struct {
		name, method, token, body, challenge string
		status                               int
	}{
		{"tool", "POST", "reader", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write","arguments":{}}}`, stepUpChallenge, 403},
		{"global-and-tool", "POST", "no-global", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write"}}`, stepUpChallenge, 403},
		{"legacy-batch", "POST", "reader", `[{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read"}},{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"write"}}]`, strings.Replace(stepUpChallenge, "mcp:access reports:write", "mcp:access reports:read reports:write", 1), 403},
		{"case-sensitive-name", "POST", "reader", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write","Name":"read"}}`, stepUpChallenge, 403},
		{"case-sensitive-other-field", "POST", "reader", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write","Arguments":12}}`, stepUpChallenge, 403},
		{"case-sensitive-batch", "POST", "reader", `[{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read"}},{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"write","Name":"read"}}]`, strings.Replace(stepUpChallenge, "mcp:access reports:write", "mcp:access reports:read reports:write", 1), 403},
		{"trailing-json", "POST", "reader", `[{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read"}},{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"write"}}] {}`, "", 400},
		{"duplicate-name-null", "POST", "reader", `[{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read"}},{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"write","name":null}}]`, "", 400},
		{"duplicate-name", "POST", "reader", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write","name":"read"}}`, "", 400},
		{"get-global", "GET", "no-global", "", strings.Replace(stepUpChallenge, "mcp:access reports:write", "mcp:access", 1), 403},
		{"delete-global", "DELETE", "no-global", "", strings.Replace(stepUpChallenge, "mcp:access reports:write", "mcp:access", 1), 403},
		{"expired-before-scope", "POST", "expired", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write"}}`, "", 401},
		{"invalid-before-scope", "POST", "invalid", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write"}}`, "", 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/mcp", strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer "+tc.token)
			req.Header.Set("MCP-Protocol-Version", "2025-03-26")
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			if recorder.Code != tc.status {
				t.Fatalf("status=%d, want %d: %s", recorder.Code, tc.status, recorder.Body)
			}
			if tc.challenge != "" && recorder.Header().Get("WWW-Authenticate") != tc.challenge {
				t.Errorf("challenge=%q, want %q", recorder.Header().Get("WWW-Authenticate"), tc.challenge)
			}
			if tc.status == 403 && recorder.Header().Get("Cache-Control") != "no-store" {
				t.Error("scope challenge must not be cached")
			}
			if calls.Load() != 0 {
				t.Fatal("business handler ran before authorization")
			}
		})
	}
}

func TestOAuthScopePreflightBoundsBodyAndAuthenticatesFirst(t *testing.T) {
	var calls atomic.Int32
	handler := newStepUpTestHandler(t, &calls)
	for _, chunked := range []bool{false, true} {
		req := httptest.NewRequest("POST", "/mcp", strings.NewReader(strings.Repeat(" ", mcp.DefaultMaxRequestBodyBytes+1)))
		req.Header.Set("Authorization", "Bearer writer")
		if chunked {
			req.ContentLength = -1
			req.TransferEncoding = []string{"chunked"}
		}
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("chunked=%t: oversized body status=%d", chunked, recorder.Code)
		}
	}
	for _, token := range []string{"", "expired", "writer"} {
		body := &unreadableOAuthBody{}
		req := httptest.NewRequest("POST", "/mcp", nil)
		req.Body = body
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		if token == "writer" {
			if recorder.Code != 400 || !body.read || !body.closed {
				t.Fatalf("read failure: status=%d, body=%+v", recorder.Code, body)
			}
		} else if recorder.Code != 401 || body.read {
			t.Fatalf("token %q: unauthenticated body read=%t, status=%d", token, body.read, recorder.Code)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid request reached business handler")
	}
}

type unreadableOAuthBody struct{ read, closed bool }

func (b *unreadableOAuthBody) Read([]byte) (int, error) {
	b.read = true
	return 0, errors.New("read failed")
}
func (b *unreadableOAuthBody) Close() error { b.closed = true; return nil }

type stepUpOAuthHandler struct {
	authorized atomic.Bool
	challenges atomic.Int32
	calls      *atomic.Int32
}

func (h *stepUpOAuthHandler) TokenSource(context.Context) (oauth2.TokenSource, error) {
	token := "reader"
	if h.authorized.Load() {
		token = "writer"
	}
	return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}), nil
}

func (h *stepUpOAuthHandler) Authorize(_ context.Context, _ *http.Request, response *http.Response) error {
	defer response.Body.Close()
	h.challenges.Add(1)
	if response.StatusCode != http.StatusForbidden || response.Header.Get("WWW-Authenticate") != stepUpChallenge {
		return fmt.Errorf("unexpected step-up: status=%d challenge=%q", response.StatusCode, response.Header.Get("WWW-Authenticate"))
	}
	if h.calls.Load() != 1 { // Only the earlier read call may have executed.
		return fmt.Errorf("write ran before step-up: calls=%d", h.calls.Load())
	}
	h.authorized.Store(true)
	return nil
}

func TestOAuthClientRetriesToolAfterScopeStepUp(t *testing.T) {
	var calls atomic.Int32
	endpoint := httptest.NewServer(newStepUpTestHandler(t, &calls))
	defer endpoint.Close()
	oauth := &stepUpOAuthHandler{calls: &calls}
	client := mcp.NewClient(&mcp.Implementation{Name: "step-up-client", Version: "v1"}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: endpoint.URL + "/mcp", OAuthHandler: oauth, DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if _, err := session.ListTools(ctx, nil); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"read", "write"} {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name})
		if err != nil || result.IsError {
			t.Fatalf("%s: result=%+v, err=%v", name, result, err)
		}
	}
	if calls.Load() != 2 || oauth.challenges.Load() != 1 {
		t.Fatalf("handler calls=%d, challenges=%d, want 2 and 1", calls.Load(), oauth.challenges.Load())
	}
}
