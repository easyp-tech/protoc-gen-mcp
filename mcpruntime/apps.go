package mcpruntime

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// AppExtensionID is the standard MCP Apps UI extension identifier.
	AppExtensionID = "io.modelcontextprotocol/ui"
	// AppHTMLMIMEType is the canonical MCP Apps HTML resource content type.
	AppHTMLMIMEType = "text/html;profile=mcp-app"
)

// AppUI links a tool to an MCP Apps resource. Visibility is either "model"
// or "app" (or both); when omitted the host's default visibility applies.
type AppUI struct {
	ResourceURI string
	Visibility  []string
}

func (a AppUI) metadata() map[string]any {
	ui := map[string]any{"resourceUri": a.ResourceURI}
	if len(a.Visibility) > 0 {
		ui["visibility"] = append([]string(nil), a.Visibility...)
	}
	return ui
}

func validateAppUI(a AppUI) error {
	if err := validateAppURI(a.ResourceURI); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, v := range a.Visibility {
		if v != "model" && v != "app" {
			return fmt.Errorf("unsupported visibility %q (expected model or app)", v)
		}
		if seen[v] {
			return fmt.Errorf("duplicate visibility %q", v)
		}
		seen[v] = true
	}
	return nil
}

func validateAppURI(raw string) error {
	if strings.TrimSpace(raw) != raw || raw == "" {
		return errors.New("app resource URI must be non-empty and unpadded")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid app resource URI: %w", err)
	}
	if u.Scheme != "ui" || u.Host == "" || u.Fragment != "" {
		return fmt.Errorf("app resource URI %q must use ui://<host> without fragments", raw)
	}
	return nil
}

// WithAppUI associates a generated tool (method segment or fully qualified
// name) with a UI resource. A UI-less client can still call the same tool and
// consume its normal structuredContent/text result.
func WithAppUI(toolName, resourceURI string, visibility ...string) RegisterOption {
	return func(opts *RegisterOptions) {
		if opts.AppUI == nil {
			opts.AppUI = make(map[string]AppUI)
		}
		opts.AppUI[toolName] = AppUI{ResourceURI: resourceURI, Visibility: append([]string(nil), visibility...)}
	}
}

func (o RegisterOptions) appUIFor(methodName, fullName string) (AppUI, bool) {
	if a, ok := o.AppUI[fullName]; ok {
		return a, true
	}
	a, ok := o.AppUI[methodName]
	return a, ok
}

// AppResource registers an HTML UI resource served directly by the SDK.
// HTML is supplied by the application; the generator does not build frontend
// bundles or expose HTML as a protobuf JSON resource.
type AppResource struct {
	URI         string
	Name        string
	Title       string
	Description string
	HTML        string
}

// RegisterAppResource registers a static MCP Apps UI resource for resources/read.
func RegisterAppResource(server *mcp.Server, resource AppResource) error {
	if server == nil {
		return errors.New("mcpruntime: server is nil")
	}
	if err := validateAppURI(resource.URI); err != nil {
		return err
	}
	if strings.TrimSpace(resource.Name) == "" {
		return errors.New("mcpruntime: app resource name is required")
	}
	if !utf8.ValidString(resource.HTML) {
		return errors.New("mcpruntime: app HTML must be UTF-8")
	}
	if strings.TrimSpace(resource.HTML) == "" {
		return errors.New("mcpruntime: app HTML must not be empty")
	}
	server.AddResource(&mcp.Resource{
		URI:         resource.URI,
		Name:        resource.Name,
		Title:       resource.Title,
		Description: resource.Description,
		MIMEType:    AppHTMLMIMEType,
	}, func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI:      req.Params.URI,
			MIMEType: AppHTMLMIMEType,
			Text:     resource.HTML,
		}}}, nil
	})
	return nil
}

// AppCapabilities advertises MCP Apps on servers that serve UI resources.
// The caller can supply additional capabilities on the returned struct.
func AppCapabilities() *mcp.ServerCapabilities {
	caps := &mcp.ServerCapabilities{}
	caps.AddExtension(AppExtensionID, map[string]any{
		"mimeTypes": []string{AppHTMLMIMEType},
	})
	return caps
}
