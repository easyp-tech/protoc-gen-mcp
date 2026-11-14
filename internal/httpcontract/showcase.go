package httpcontract

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"
)

// Showcase describes an example server started with --http --demo-auth: it
// accepts the tokens demo-read and demo-write, and ScopedTool needs a scope
// only demo-write holds.
type Showcase struct {
	MetadataPath    string
	ScopesSupported []string
	ScopedTool      string
	ScopedArgs      map[string]any
}

// RunShowcase checks the OAuth walkthrough every example README describes:
// discovery, the bearer challenge, and the scope step-up on ScopedTool.
func RunShowcase(t *testing.T, cmd *exec.Cmd, showcase Showcase) {
	t.Helper()
	base := start(t, cmd)
	client := &http.Client{Timeout: 30 * time.Second}

	resp := do(t, client, http.MethodGet, base+showcase.MetadataPath, "", nil)
	body := readBody(t, resp)
	var metadata struct {
		ScopesSupported []string `json:"scopes_supported"`
	}
	if resp.StatusCode != http.StatusOK || json.Unmarshal(body, &metadata) != nil || !slices.Equal(metadata.ScopesSupported, showcase.ScopesSupported) {
		t.Fatalf("GET %s = %d %s, want scopes_supported %v", showcase.MetadataPath, resp.StatusCode, body, showcase.ScopesSupported)
	}

	resp = do(t, client, http.MethodPost, base+"/mcp", "", rpc(1, "tools/list", map[string]any{}))
	readBody(t, resp)
	if resp.StatusCode != http.StatusUnauthorized || !strings.Contains(resp.Header.Get("WWW-Authenticate"), "resource_metadata=") {
		t.Fatalf("tools/list without a token = %d %q, want 401 with resource_metadata", resp.StatusCode, resp.Header.Get("WWW-Authenticate"))
	}

	call(t, client, base, "demo-read", rpc(2, "tools/list", map[string]any{}))
	scoped := rpc(3, "tools/call", map[string]any{"name": showcase.ScopedTool, "arguments": showcase.ScopedArgs})
	resp = do(t, client, http.MethodPost, base+"/mcp", "demo-read", scoped)
	body = readBody(t, resp)
	if resp.StatusCode != http.StatusForbidden || !strings.Contains(resp.Header.Get("WWW-Authenticate"), `error="insufficient_scope"`) {
		t.Fatalf("demo-read %s = %d %s, want 403 insufficient_scope", showcase.ScopedTool, resp.StatusCode, body)
	}

	result := call(t, client, base, "demo-write", scoped)
	var outcome struct {
		IsError bool `json:"isError"`
	}
	decode(t, result, &outcome)
	if outcome.IsError {
		t.Fatalf("demo-write %s = %s, want success", showcase.ScopedTool, result)
	}
}
