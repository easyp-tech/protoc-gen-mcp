package mcpruntime

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

const ProtectedResourceMetadataPath = "/.well-known/oauth-protected-resource"

// OAuthResourceServer configures an MCP OAuth protected resource. Authentication
// is handled by an external authorization server. Verifier must validate that
// the bearer token was issued for this resource (issuer, audience, signature).
type OAuthResourceServer struct {
	ResourceURL               string
	AuthorizationServers      []string
	Scopes                    []string
	Issuer, Audience, JWKSURL string
	Verifier                  auth.TokenVerifier
	MCPPath                   string
}

// NewOAuthResourceHandler mounts the SDK's protected-resource discovery and
// bearer-token middleware around a stateless MCP endpoint. The metadata route
// intentionally stays public, while all MCP methods require bearer auth.
func NewOAuthResourceHandler(server *mcp.Server, config OAuthResourceServer) (http.Handler, error) {
	if server == nil || config.Verifier == nil {
		return nil, errors.New("mcpruntime: SDK server and token verifier are required")
	}
	resourceURL, err := url.Parse(config.ResourceURL)
	if err != nil || resourceURL.Scheme != "https" || resourceURL.Host == "" {
		return nil, fmt.Errorf("mcpruntime: invalid HTTPS resource URL %q", config.ResourceURL)
	}
	if len(config.AuthorizationServers) == 0 {
		return nil, errors.New("mcpruntime: at least one authorization server is required")
	}
	for _, address := range config.AuthorizationServers {
		parsed, err := url.Parse(address)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return nil, fmt.Errorf("mcpruntime: invalid HTTPS authorization server %q", address)
		}
	}
	path := config.MCPPath
	if path == "" {
		path = "/mcp"
	}
	if path[0] != '/' || path == ProtectedResourceMetadataPath || path == "/" {
		return nil, fmt.Errorf("mcpruntime: invalid MCP path %q", path)
	}

	metadata := &oauthex.ProtectedResourceMetadata{
		Resource:             config.ResourceURL,
		AuthorizationServers: append([]string(nil), config.AuthorizationServers...),
		ScopesSupported:      append([]string(nil), config.Scopes...),
	}
	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true},
	)
	metadataURL := resourceURL.Scheme + "://" + resourceURL.Host + ProtectedResourceMetadataPath
	// Let the SDK validate bearer tokens and expiration before inspecting the
	// body. Scope checks run at the HTTP boundary so clients can authorize and
	// retry an operation without the business handler running first.
	protected := auth.RequireBearerToken(config.Verifier, &auth.RequireBearerTokenOptions{
		ResourceMetadataURL: metadataURL,
	})(requireOAuthScopes(server, config.Scopes, metadataURL, handler))

	mux := http.NewServeMux()
	mux.Handle(ProtectedResourceMetadataPath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Keep discovery current when other generated files register tools after
		// the HTTP handler is built. Tool scopes must not become global gates.
		current := *metadata
		current.ScopesSupported = append(slices.Clone(metadata.ScopesSupported), registeredSDKToolScopes(server)...)
		slices.Sort(current.ScopesSupported)
		current.ScopesSupported = slices.Compact(current.ScopesSupported)
		auth.ProtectedResourceMetadataHandler(&current).ServeHTTP(w, r)
	}))
	mux.Handle(path, protected)
	return http.NewCrossOriginProtection().Handler(mux), nil
}
