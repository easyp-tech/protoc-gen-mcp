// Package httpcontract checks the OAuth-protected Streamable HTTP endpoint
// that every target generates for internal/testproto/config/v1, against the
// behaviour of the Go mcpruntime (oauth.go, oauth_scope.go).
package httpcontract

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"
)

// Tokens the server under test must accept through its demo verifier. Any
// other bearer token must be rejected as invalid.
const (
	ReaderToken = "reader" // scopes: reports:read
	WriterToken = "writer" // scopes: reports:read reports:write
)

// ListenPrefix starts the line a server prints on stdout once it accepts
// connections, followed by its base URL, for example
// "listening on http://127.0.0.1:43123".
const ListenPrefix = "listening on "

const (
	metadataURL  = "https://mcp.example.com/.well-known/oauth-protected-resource/mcp"
	toolName     = "secure_check_access"
	maxBodyBytes = 4 << 20
)

// Run starts cmd and checks the protected-resource metadata, the bearer
// challenge, the per-tool scope step-up and the request preflight.
func Run(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	base := start(t, cmd)
	client := &http.Client{Timeout: 30 * time.Second}

	t.Run("metadata", func(t *testing.T) {
		for _, path := range []string{"/.well-known/oauth-protected-resource/mcp", "/.well-known/oauth-protected-resource"} {
			resp := do(t, client, http.MethodGet, base+path, "", nil)
			body := readBody(t, resp)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET %s = %d %s", path, resp.StatusCode, body)
			}
			var metadata struct {
				Resource             string   `json:"resource"`
				AuthorizationServers []string `json:"authorization_servers"`
				ScopesSupported      []string `json:"scopes_supported"`
			}
			if err := json.Unmarshal(body, &metadata); err != nil {
				t.Fatalf("GET %s body %s: %v", path, body, err)
			}
			if metadata.Resource != "https://mcp.example.com/mcp" ||
				!slices.Equal(metadata.AuthorizationServers, []string{"https://auth.example.com"}) ||
				!slices.Equal(metadata.ScopesSupported, []string{"reports:read", "reports:write"}) {
				t.Fatalf("GET %s metadata = %+v", path, metadata)
			}
		}
		resp := do(t, client, http.MethodGet, base+"/.well-known/oauth-protected-resource/other", "", nil)
		readBody(t, resp)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("GET an unknown metadata path = %d, want 404", resp.StatusCode)
		}
	})

	t.Run("bearer challenge", func(t *testing.T) {
		for _, token := range []string{"", "not-a-token"} {
			resp := do(t, client, http.MethodPost, base+"/mcp", token, rpc(1, "tools/list", map[string]any{}))
			readBody(t, resp)
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("token %q: status %d, want 401", token, resp.StatusCode)
			}
			if challenge := resp.Header.Get("WWW-Authenticate"); !strings.Contains(challenge, `resource_metadata="`+metadataURL+`"`) {
				t.Fatalf("token %q: WWW-Authenticate %q lacks resource_metadata", token, challenge)
			}
		}
	})

	t.Run("initialize", func(t *testing.T) {
		result := call(t, client, base, ReaderToken, rpc(0, "initialize", map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "httpcontract", "version": "v0.0.0"},
		}))
		var initialize struct {
			ProtocolVersion string `json:"protocolVersion"`
			Capabilities    struct {
				Tools *struct{} `json:"tools"`
			} `json:"capabilities"`
			ServerInfo struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"serverInfo"`
		}
		decode(t, result, &initialize)
		if initialize.ProtocolVersion != "2025-06-18" || initialize.Capabilities.Tools == nil ||
			initialize.ServerInfo.Name != "secure-example" || initialize.ServerInfo.Version != "v2.0.0" {
			t.Fatalf("initialize = %s", result)
		}
	})

	t.Run("tools and scope step-up", func(t *testing.T) {
		list := call(t, client, base, ReaderToken, rpc(1, "tools/list", map[string]any{}))
		var tools struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		}
		decode(t, list, &tools)
		if len(tools.Tools) != 1 || tools.Tools[0].Name != toolName {
			t.Fatalf("tools/list = %+v, want %s", tools.Tools, toolName)
		}

		callBody := rpc(2, "tools/call", map[string]any{"name": toolName, "arguments": map[string]any{"label": "q3"}})
		resp := do(t, client, http.MethodPost, base+"/mcp", ReaderToken, callBody)
		body := readBody(t, resp)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("reader tools/call = %d %s, want 403", resp.StatusCode, body)
		}
		challenge := resp.Header.Get("WWW-Authenticate")
		for _, part := range []string{`error="insufficient_scope"`, `scope="reports:read reports:write"`, `resource_metadata="` + metadataURL + `"`} {
			if !strings.Contains(challenge, part) {
				t.Fatalf("403 WWW-Authenticate %q lacks %s", challenge, part)
			}
		}
		if got := resp.Header.Get("Cache-Control"); got != "no-store" {
			t.Fatalf("403 Cache-Control = %q, want no-store", got)
		}

		result := call(t, client, base, WriterToken, callBody)
		var called struct {
			IsError           bool           `json:"isError"`
			StructuredContent map[string]any `json:"structuredContent"`
		}
		decode(t, result, &called)
		if called.IsError || called.StructuredContent["allowed"] != true {
			t.Fatalf("writer tools/call = %s", result)
		}
	})

	t.Run("request preflight", func(t *testing.T) {
		cases := []struct {
			name string
			body []byte
			want int
		}{
			{"duplicate tool name", []byte(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"x","name":"` + toolName + `"}}`), http.StatusBadRequest},
			{"trailing JSON", append(rpc(4, "tools/list", map[string]any{}), []byte(" {}")...), http.StatusBadRequest},
			{"oversized body", append(rpc(5, "tools/list", map[string]any{}), bytes.Repeat([]byte(" "), maxBodyBytes)...), http.StatusRequestEntityTooLarge},
		}
		for _, tc := range cases {
			resp := do(t, client, http.MethodPost, base+"/mcp", WriterToken, tc.body)
			readBody(t, resp)
			if resp.StatusCode != tc.want {
				t.Fatalf("%s: status %d, want %d", tc.name, resp.StatusCode, tc.want)
			}
		}
		batch := []byte(`[` + string(rpc(6, "tools/list", map[string]any{})) + `,` +
			string(rpc(7, "tools/call", map[string]any{"name": toolName, "arguments": map[string]any{"label": "a"}})) + `]`)
		resp := do(t, client, http.MethodPost, base+"/mcp", ReaderToken, batch)
		readBody(t, resp)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("reader batch with a protected call: status %d, want 403", resp.StatusCode)
		}
	})
}

func start(t *testing.T, cmd *exec.Cmd) string {
	t.Helper()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	lines := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if line := scanner.Text(); strings.HasPrefix(line, ListenPrefix) {
				lines <- strings.TrimPrefix(line, ListenPrefix)
				_, _ = io.Copy(io.Discard, stdout)
				return
			}
		}
		close(lines)
	}()
	select {
	case base, ok := <-lines:
		if !ok {
			t.Fatalf("server exited before listening; stderr:\n%s", stderr.String())
		}
		return strings.TrimRight(base, "/")
	case <-time.After(90 * time.Second):
		t.Fatalf("server did not start listening; stderr:\n%s", stderr.String())
	}
	return ""
}

func rpc(id int, method string, params any) []byte {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	return body
}

func do(t *testing.T, client *http.Client, method, url, token string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return body
}

// call posts one JSON-RPC request and returns its result, accepting a JSON
// or a server-sent-events response.
func call(t *testing.T, client *http.Client, base, token string, body []byte) json.RawMessage {
	t.Helper()
	resp := do(t, client, http.MethodPost, base+"/mcp", token, body)
	raw := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /mcp %s = %d %s", body, resp.StatusCode, raw)
	}
	payload := raw
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		payload = nil
		for _, line := range strings.Split(string(raw), "\n") {
			if data, ok := strings.CutPrefix(strings.TrimRight(line, "\r"), "data:"); ok {
				payload = []byte(strings.TrimSpace(data))
			}
		}
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("decode JSON-RPC response %s: %v", raw, err)
	}
	if envelope.Error != nil {
		t.Fatalf("JSON-RPC error %d: %s", envelope.Error.Code, envelope.Error.Message)
	}
	return envelope.Result
}

func decode(t *testing.T, raw json.RawMessage, out any) {
	t.Helper()
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
}

// DemoScopes returns the scopes a demo token grants, or nil for an invalid
// token; servers under test implement the same table.
func DemoScopes(token string) []string {
	switch token {
	case ReaderToken:
		return []string{"reports:read"}
	case WriterToken:
		return []string{"reports:read", "reports:write"}
	default:
		return nil
	}
}

// Listening formats the line servers print once they accept connections.
func Listening(base string) string {
	return fmt.Sprintf("%s%s", ListenPrefix, base)
}
