package mcpruntime

import (
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAppResourceMetadata_AllProperties(t *testing.T) {
	border := false
	meta := AppResourceMetadata(AppResourceConfig{
		CSP: AppResourceCSP{
			ConnectDomains:  []string{"https://api.example.com"},
			ResourceDomains: []string{"https://cdn.example.com"},
			FrameDomains:    []string{"https://video.example.com"},
			BaseURIDomains:  []string{"https://base.example.com"},
		},
		Permissions:   AppResourcePermissions{Camera: true, Microphone: true, Geolocation: true, ClipboardWrite: true},
		Domain:        "app.example.com",
		PrefersBorder: &border,
	})
	ui, ok := meta["ui"].(map[string]any)
	if !ok {
		t.Fatalf("missing UI metadata: %+v", meta)
	}
	csp := ui["csp"].(map[string]any)
	for key := range map[string]bool{"connectDomains": true, "resourceDomains": true, "frameDomains": true, "baseUriDomains": true} {
		if csp[key] == nil {
			t.Errorf("missing CSP %s", key)
		}
	}
	perms := ui["permissions"].(map[string]any)
	for _, key := range []string{"camera", "microphone", "geolocation", "clipboardWrite"} {
		if !reflect.DeepEqual(perms[key], map[string]any{}) {
			t.Errorf("missing permission %s", key)
		}
	}
	if ui["domain"] != "app.example.com" || ui["prefersBorder"] != false {
		t.Fatalf("UI options %+v", ui)
	}
	content := []*mcp.ResourceContents{{URI: "ui://example/app"}, nil}
	got := SetResourceMetadata(content, meta)
	if got[0].Meta["ui"] == nil {
		t.Fatal("content metadata missing")
	}
	if got[1] != nil {
		t.Fatal("nil resource content mutated")
	}
	if len(SetResourceMetadata(content, nil)) != len(content) {
		t.Fatal("metadata-less content changed")
	}
	if AppResourceMetadata(AppResourceConfig{}) != nil {
		t.Fatal("zero config should be omitted")
	}
}

func TestRegisterEmbeddedResource_TextBinaryAndErrors(t *testing.T) {
	assets := fstest.MapFS{
		"app.html": &fstest.MapFile{Data: []byte("<html>Hello App</html>")},
		"blob.bin": &fstest.MapFile{Data: []byte{0, 1, 255}},
	}
	type fixture struct {
		path, uri, mime string
		text            bool
	}
	for _, tc := range []fixture{
		{"app.html", "ui://demo/app", "text/html;profile=mcp-app", true},
		{"blob.bin", "data://demo/blob", "application/octet-stream", false},
	} {
		t.Run(tc.path, func(t *testing.T) {
			server := mcp.NewServer(&mcp.Implementation{Name: "embedded-test", Version: "v1"}, nil)
			meta := AppResourceMetadata(AppResourceConfig{Domain: "app.example.com"})
			err := RegisterEmbeddedResource(server, assets, tc.path, &mcp.Resource{
				URI: tc.uri, Name: "embedded", MIMEType: tc.mime, Meta: meta,
			})
			if err != nil {
				t.Fatalf("RegisterEmbeddedResource: %v", err)
			}
			session := testSDKServerClient(t, server)
			contents, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: tc.uri})
			if err != nil || len(contents.Contents) != 1 {
				t.Fatalf("resources/read: %+v, %v", contents, err)
			}
			got := contents.Contents[0]
			if tc.text && !strings.Contains(got.Text, "Hello") {
				t.Fatalf("wrong text: %+v", got)
			}
			if !tc.text && string(got.Blob) != string([]byte{0, 1, 255}) {
				t.Fatalf("wrong blob: %+v", got)
			}
			if got.Meta["ui"] == nil {
				t.Fatal("UI metadata not attached")
			}
		})
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "errors", Version: "v1"}, nil)
	for _, tc := range []struct {
		path, mime string
		server     *mcp.Server
		fsys       fs.FS
		res        *mcp.Resource
	}{
		{"app.html", "text/plain", nil, assets, &mcp.Resource{Name: "x"}},
		{"app.html", "text/plain", server, nil, &mcp.Resource{Name: "x"}},
		{"app.html", "text/plain", server, assets, nil},
		{"../unsafe.html", "text/plain", server, assets, &mcp.Resource{Name: "x"}},
		{"missing.txt", "text/plain", server, assets, &mcp.Resource{Name: "x"}},
		{"app.html", "not mime", server, assets, &mcp.Resource{Name: "x", MIMEType: "not mime"}},
	} {
		if err := RegisterEmbeddedResource(tc.server, tc.fsys, tc.path, tc.res); err == nil {
			t.Errorf("accepted invalid embedded resource %+v", tc)
		}
	}
}

func TestConfiguredHTTPServer_InvalidAndOpenRoutes(t *testing.T) {
	if _, err := NewConfiguredServer("", "v1", false); err == nil {
		t.Fatal("server without name accepted")
	}
	s, err := NewConfiguredServer("configured", "", true)
	if err != nil {
		t.Fatal(err)
	}
	session := testSDKServerClient(t, s)
	initialized := session.InitializeResult()
	if initialized == nil || initialized.Capabilities == nil || initialized.Capabilities.Extensions[AppExtensionID] == nil {
		t.Fatalf("apps capability missing: %+v", initialized)
	}
	if _, err := NewConfiguredHTTPHandler(nil, nil, nil); err == nil {
		t.Fatal("nil server accepted")
	}
	h, err := NewConfiguredHTTPHandler(s, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// A public MCP endpoint must never return OAuth 401 without OAuth config.
	result := httptest.NewRecorder()
	h.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/mcp", nil))
	if result.Code == http.StatusUnauthorized {
		t.Fatal("unprotected endpoint requires OAuth unexpectedly")
	}
	if _, err := NewConfiguredHTTPHandler(s, &OAuthResourceServer{ResourceURL: "https://mcp.example.com/mcp"}, nil); err == nil {
		t.Fatal("unconfigured JWKS verifier accepted")
	}
	custom := auth.TokenVerifier(func(context.Context, string, *http.Request) (*auth.TokenInfo, error) {
		return &auth.TokenInfo{Scopes: []string{"read"}, Expiration: time.Now().Add(time.Hour)}, nil
	})
	h, err = NewConfiguredHTTPHandler(s, &OAuthResourceServer{
		ResourceURL:          "https://mcp.example.com/mcp",
		AuthorizationServers: []string{"https://auth.example.com"},
		Scopes:               []string{"read"},
	}, custom)
	if err != nil {
		t.Fatal(err)
	}
	unauth := httptest.NewRecorder()
	h.ServeHTTP(unauth, httptest.NewRequest(http.MethodGet, "/mcp", nil))
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous HTTP status=%d", unauth.Code)
	}
}

func TestJWKSVerifier_InvalidRemoteDocuments(t *testing.T) {
	for _, body := range []string{
		"not-json",
		`{"keys":[]}`,
		`{"keys":[{"kid":"one","kty":"RSA","alg":"HS256"}]}`,
		`{"keys":[{"kid":"one","kty":"RSA","n":"bad","e":"AQAB"}]}`,
		strings.Repeat("x", 1<<20+1),
	} {
		remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
		cache := &cachedJWKS{client: remote.Client(), config: JWKSVerifierConfig{JWKSURL: remote.URL}}
		if err := cache.load(context.Background()); err == nil {
			t.Errorf("invalid JWKS body was accepted: %s", body[:min(len(body), 80)])
		}
		remote.Close()
	}
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "unavailable", http.StatusBadGateway) }))
	defer remote.Close()
	cache := &cachedJWKS{client: remote.Client(), config: JWKSVerifierConfig{JWKSURL: remote.URL}}
	if err := cache.load(context.Background()); err == nil {
		t.Fatal("HTTP 502 JWKS accepted")
	}
}
