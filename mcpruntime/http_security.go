package mcpruntime

import (
	"net/http"
	"net/url"
	"strings"
)

// supportedProtocolVersions lists protocol versions we accept in MCP-Protocol-Version.
var supportedProtocolVersions = map[string]struct{}{
	"2025-11-25": {},
	"2025-03-26": {},
	"2024-11-05": {},
}

const (
	headerMCPSessionID       = "MCP-Session-Id"
	headerMCPProtocolVersion = "MCP-Protocol-Version"
	headerLastEventID        = "Last-Event-ID"
)

// validateOrigin checks the Origin header against the allowlist.
// Spec: if Origin is present and invalid, respond 403.
// When Origin is absent, the request is allowed (non-browser clients).
func validateOrigin(r *http.Request, allowed []string, allowAll bool) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if allowAll {
		return true
	}
	if len(allowed) == 0 {
		// Default: allow only localhost origins when no list configured.
		return isLocalhostOrigin(origin)
	}
	for _, a := range allowed {
		if originsEqual(a, origin) {
			return true
		}
	}
	return false
}

func isLocalhostOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func originsEqual(a, b string) bool {
	return strings.EqualFold(strings.TrimRight(a, "/"), strings.TrimRight(b, "/"))
}

// resolveProtocolVersion implements MCP-Protocol-Version handling.
// Missing header → assume 2025-03-26 for backwards compatibility.
// Unsupported → error message for 400 response.
func resolveProtocolVersion(header string, session *Session) (string, error) {
	if header == "" {
		if session != nil {
			if v := session.ProtocolVersion(); v != "" {
				return v, nil
			}
		}
		return "2025-03-26", nil
	}
	if _, ok := supportedProtocolVersions[header]; !ok {
		return "", errUnsupportedProtocolVersion(header)
	}
	return header, nil
}

type protocolVersionError struct {
	version string
}

func errUnsupportedProtocolVersion(v string) error {
	return &protocolVersionError{version: v}
}

func (e *protocolVersionError) Error() string {
	return "unsupported MCP-Protocol-Version: " + e.version
}
