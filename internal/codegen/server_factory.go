package codegen

import (
	"path"

	mcpoptionsv1 "github.com/easyp-tech/protoc-gen-mcp/mcp/options/v1"
)

// appsExtensionID and appsMIMEType mirror mcpruntime.AppExtensionID and
// mcpruntime.AppHTMLMIMEType.
const (
	appsExtensionID = "io.modelcontextprotocol/ui"
	appsMIMEType    = "text/html;profile=mcp-app"
)

// ServerFactoryModel describes the file-scoped server factory every target
// generates, following render_go_server.go.
type ServerFactoryModel struct {
	// Emit is set when the file has a server option or needs MCP Apps.
	Emit    bool
	Name    string
	Version string
	Apps    bool
	// HTTP is set when the file declares a server option; the generated HTTP
	// handler serves it, protected by OAuth when OAuth is non-nil.
	HTTP  bool
	OAuth *mcpoptionsv1.OAuthConfig
}

func newServerFactoryModel(model FileModel) ServerFactoryModel {
	apps := modelNeedsApps(model)
	factory := ServerFactoryModel{
		Emit:    model.ServerConfig != nil || apps,
		Name:    path.Base(model.ProtoPath),
		Version: "v0.0.1",
		Apps:    apps,
	}
	if model.ServerConfig != nil {
		factory.HTTP = true
		factory.OAuth = model.ServerConfig.GetOauth()
		if model.ServerConfig.GetName() != "" {
			factory.Name = model.ServerConfig.GetName()
		}
		if model.ServerConfig.GetVersion() != "" {
			factory.Version = model.ServerConfig.GetVersion()
		}
	}
	return factory
}
