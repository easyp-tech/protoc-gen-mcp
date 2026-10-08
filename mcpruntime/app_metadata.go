package mcpruntime

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AppResourceCSP declares the standard MCP Apps sandbox Content Security Policy.
type AppResourceCSP struct {
	ConnectDomains  []string
	ResourceDomains []string
	FrameDomains    []string
	BaseURIDomains  []string
}

// AppResourcePermissions lists browser permissions requested by an MCP App.
type AppResourcePermissions struct {
	Camera, Microphone, Geolocation, ClipboardWrite bool
}

type AppResourceConfig struct {
	CSP           AppResourceCSP
	Permissions   AppResourcePermissions
	Domain        string
	PrefersBorder *bool
}

// AppResourceMetadata returns _meta.ui on both listed resources and read results.
func AppResourceMetadata(c AppResourceConfig) mcp.Meta {
	ui := map[string]any{}
	csp := map[string]any{}
	if len(c.CSP.ConnectDomains) > 0 { csp["connectDomains"] = c.CSP.ConnectDomains }
	if len(c.CSP.ResourceDomains) > 0 { csp["resourceDomains"] = c.CSP.ResourceDomains }
	if len(c.CSP.FrameDomains) > 0 { csp["frameDomains"] = c.CSP.FrameDomains }
	if len(c.CSP.BaseURIDomains) > 0 { csp["baseUriDomains"] = c.CSP.BaseURIDomains }
	if len(csp) > 0 { ui["csp"] = csp }
	permissions := map[string]any{}
	if c.Permissions.Camera { permissions["camera"] = map[string]any{} }
	if c.Permissions.Microphone { permissions["microphone"] = map[string]any{} }
	if c.Permissions.Geolocation { permissions["geolocation"] = map[string]any{} }
	if c.Permissions.ClipboardWrite { permissions["clipboardWrite"] = map[string]any{} }
	if len(permissions) > 0 { ui["permissions"] = permissions }
	if c.Domain != "" { ui["domain"] = c.Domain }
	if c.PrefersBorder != nil { ui["prefersBorder"] = *c.PrefersBorder }
	if len(ui) == 0 { return nil }
	return mcp.Meta{"ui": ui}
}

// SetResourceMetadata attaches the same MCP Apps metadata to resources/read
// content as is advertised on the resources/list entry.
func SetResourceMetadata(contents []*mcp.ResourceContents, meta mcp.Meta) []*mcp.ResourceContents {
	if len(meta) == 0 { return contents }
	for _, content := range contents {
		if content == nil { continue }
		content.Meta = make(mcp.Meta, len(meta))
		for k, v := range meta { content.Meta[k] = v }
	}
	return contents
}

// RegisterEmbeddedResource registers one Go-embedded static file without
// requiring an application-written resource handler.
func RegisterEmbeddedResource(server *mcp.Server, source fs.FS, sourcePath string, resource *mcp.Resource) error {
	if server == nil || resource == nil || source == nil {
		return errors.New("mcpruntime: embedded resource requires server, filesystem and descriptor")
	}
	if !fs.ValidPath(sourcePath) || sourcePath == "." {
		return fmt.Errorf("mcpruntime: invalid embedded resource path %q", sourcePath)
	}
	data, err := fs.ReadFile(source, sourcePath)
	if err != nil { return fmt.Errorf("mcpruntime: read embedded resource %q: %w", sourcePath, err) }
	base, _, err := mime.ParseMediaType(resource.MIMEType)
	if err != nil { return fmt.Errorf("mcpruntime: invalid embedded resource MIME %q: %w", resource.MIMEType, err) }
	text := strings.HasPrefix(base, "text/") || base == "application/javascript" || base == "application/xml" || strings.HasSuffix(base, "+xml") || strings.HasSuffix(base, "+json") || base == "application/json"
	server.AddResource(resource, func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		content := &mcp.ResourceContents{URI: req.Params.URI, MIMEType: resource.MIMEType}
		if text { content.Text = string(data) } else { content.Blob = append([]byte(nil), data...) }
		return &mcp.ReadResourceResult{Contents: SetResourceMetadata([]*mcp.ResourceContents{content}, resource.Meta)}, nil
	})
	return nil
}
