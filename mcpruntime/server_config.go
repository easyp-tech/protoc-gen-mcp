package mcpruntime

import (
	"errors"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewConfiguredServer enables MCP Apps when selected by protobuf options.
// The official SDK owns version negotiation, sessions, tools and transports.
func NewConfiguredServer(name, version string, apps bool) (*mcp.Server, error) {
	if name == "" { return nil, errors.New("mcpruntime: configured server needs a name") }
	if version == "" { version = "v0.0.1" }
	var options *mcp.ServerOptions
	if apps { options = &mcp.ServerOptions{Capabilities: AppCapabilities()} }
	return mcp.NewServer(&mcp.Implementation{Name: name, Version: version}, options), nil
}

// NewConfiguredHTTPHandler creates a protected or open stateless SDK endpoint.
// A custom verifier overrides JWKS. When JWKS is configured, nil verifier
// enables built-in JWT verification without requiring application auth code.
func NewConfiguredHTTPHandler(server *mcp.Server, oauth *OAuthResourceServer, verifier auth.TokenVerifier) (http.Handler, error) {
	if server == nil { return nil, errors.New("mcpruntime: server is nil") }
	if oauth == nil {
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true})
		mux := http.NewServeMux()
		mux.Handle("/mcp", http.NewCrossOriginProtection().Handler(handler))
		return mux, nil
	}
	cfg := *oauth
	cfg.Verifier = verifier
	if cfg.Verifier == nil {
		v, err := NewJWKSVerifier(JWKSVerifierConfig{JWKSURL: cfg.JWKSURL, Issuer: cfg.Issuer, Audience: cfg.Audience})
		if err != nil { return nil, err }
		cfg.Verifier = v
	}
	return NewOAuthResourceHandler(server, cfg)
}
