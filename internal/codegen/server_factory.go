package codegen

import "path"

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
		if model.ServerConfig.GetName() != "" {
			factory.Name = model.ServerConfig.GetName()
		}
		if model.ServerConfig.GetVersion() != "" {
			factory.Version = model.ServerConfig.GetVersion()
		}
	}
	return factory
}
