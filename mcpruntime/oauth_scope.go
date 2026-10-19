package mcpruntime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// requireOAuthScopes is an authorization preflight, not a protocol dispatcher:
// the SDK still parses, validates and executes every accepted MCP message.
// Keep the tool callback's scope guard as defense in depth for other transports.
func requireOAuthScopes(server *mcp.Server, global []string, metadataURL string, next http.Handler) http.Handler {
	global = slices.Clone(global)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		required := slices.Clone(global)
		if r.Method == http.MethodPost && r.Body != nil {
			// Match the SDK's default limit, including chunked requests, rather
			// than introducing an unbounded read in front of its transport.
			body, err := io.ReadAll(io.LimitReader(r.Body, mcp.DefaultMaxRequestBodyBytes+1))
			r.Body.Close()
			if err != nil {
				http.Error(w, "cannot read MCP request", http.StatusBadRequest)
				return
			}
			if len(body) > mcp.DefaultMaxRequestBodyBytes {
				http.Error(w, "MCP request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			toolScopes, err := requestedToolScopes(server, body)
			if err != nil {
				http.Error(w, "invalid MCP request", http.StatusBadRequest)
				return
			}
			required = append(required, toolScopes...)
		}
		info := auth.TokenInfoFromContext(r.Context())
		if info == nil {
			http.Error(w, "authenticated token required", http.StatusUnauthorized)
			return
		}
		for _, scope := range required {
			if !slices.Contains(info.Scopes, scope) {
				slices.Sort(required)
				required = slices.Compact(required)
				w.Header().Set("WWW-Authenticate", fmt.Sprintf(
					"Bearer error=\"insufficient_scope\", scope=%q, resource_metadata=%q",
					strings.Join(required, " "), metadataURL))
				w.Header().Set("Cache-Control", "no-store")
				http.Error(w, "insufficient scope", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func requestedToolScopes(server *mcp.Server, body []byte) ([]string, error) {
	// Some SDK decoder paths accept the first value and ignore trailing JSON.
	// Never let a parse mismatch skip authorization then dispatch a valid prefix.
	if !json.Valid(body) {
		return nil, fmt.Errorf("expected one JSON value")
	}
	messages := []json.RawMessage{body}
	if trimmed := bytes.TrimSpace(body); len(trimmed) > 0 && trimmed[0] == '[' {
		// Legacy SDK protocols support batches. Check all calls before any can
		// execute; otherwise retrying a challenged batch could repeat effects.
		if err := json.Unmarshal(body, &messages); err != nil {
			return nil, err
		}
	}
	var scopes []string
	for _, raw := range messages {
		message, err := jsonrpc.DecodeMessage(raw)
		if err != nil {
			continue
		}
		request, ok := message.(*jsonrpc.Request)
		if !ok || request.Method != "tools/call" {
			continue
		}
		name, err := requestedToolName(request.Params)
		if err != nil {
			return nil, err
		}
		scopes = append(scopes, registeredSDKToolRequiredScopes(server, name)...)
	}
	return scopes, nil
}

func requestedToolName(params json.RawMessage) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(params))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return "", fmt.Errorf("tools/call params must be an object")
	}
	var name string
	seenName := false
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return "", err
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return "", err
		}
		// The SDK matches JSON fields case-sensitively. Unmarshalling a Go
		// struct with encoding/json would incorrectly treat "Name" as "name".
		if key == "name" {
			if seenName {
				return "", fmt.Errorf("ambiguous duplicate tool name")
			}
			seenName = true
			if err := json.Unmarshal(value, &name); err != nil {
				return "", err
			}
		}
	}
	return name, nil
}
