package codegen

import (
	"fmt"
	"path"

	"google.golang.org/protobuf/compiler/protogen"
)

// renderGoServerFactory adds a file-scoped typed constructor. It composes
// generated registrations from this protobuf file and applies the server's
// declarative capabilities. Other protobuf files can register additional
// tools/resources on the returned SDK server without a second MCP runtime.
func renderGoServerFactory(g *protogen.GeneratedFile, info goRenderInfo, model FileModel) error {
	fileName := info.file.GoDescriptorIdent.GoName
	handlersName := fileName + "MCPHandlers"
	newName := "New" + fileName + "MCPServer"
	mcpImport := protogen.GoImportPath("github.com/modelcontextprotocol/go-sdk/mcp")
	runtimeImport := protogen.GoImportPath("github.com/easyp-tech/protoc-gen-mcp/mcpruntime")
	serverType := g.QualifiedGoIdent(mcpImport.Ident("Server"))
	contextType := g.QualifiedGoIdent(protogen.GoImportPath("context").Ident("Context"))

	g.P("// ", handlersName, " supplies business handlers for the generated file.")
	g.P("type ", handlersName, " struct {")
	for _, service := range model.Services {
		goName, err := info.serviceGoName(service)
		if err != nil { return err }
		g.P(goName, " ", goName, "ToolHandler")
	}
	if len(model.Prompts) > 0 { g.P("Prompts ", fileName, "PromptHandler") }
	if resourcesNeedImpl(model.Resources) {
		g.P("Resources ", fileName, "ResourceHandler")
	}
	g.P("}")
	g.P()

	name, version := path.Base(model.ProtoPath), "v0.0.1"
	if model.ServerConfig != nil {
		if model.ServerConfig.GetName() != "" { name = model.ServerConfig.GetName() }
		if model.ServerConfig.GetVersion() != "" { version = model.ServerConfig.GetVersion() }
	}
	g.P("// ", newName, " constructs and populates an official MCP SDK server.")
	g.P("func ", newName, "(ctx ", contextType, ", handlers ", handlersName, ") (*", serverType, ", error) {")
	g.P("server, err := ", g.QualifiedGoIdent(runtimeImport.Ident("NewConfiguredServer")), "(", quote(name), ",", quote(version), ",", fmt.Sprintf("%t",modelNeedsApps(model)), ")")
	g.P("if err != nil { return nil, err }")
	for _, service := range model.Services {
		goName, err := info.serviceGoName(service)
		if err != nil { return err }
		g.P("if err := Register", goName, "Tools(server, handlers.", goName, "); err != nil { return nil, err }")
	}
	if len(model.Prompts) > 0 {
		g.P("if err := Register", fileName, "Prompts(server, handlers.Prompts); err != nil { return nil, err }")
	}
	if len(model.Resources) > 0 {
		if resourcesNeedImpl(model.Resources) {
			g.P("if err := Register", fileName, "Resources(ctx, server, handlers.Resources); err != nil { return nil, err }")
		} else {
			g.P("if err := Register", fileName, "Resources(ctx, server, struct{}{}); err != nil { return nil, err }")
		}
	}
	g.P("return server, nil")
	g.P("}")
	g.P()

	// An external identity provider remains responsible for issuing tokens.
	// verifier=nil enables built-in RS256/JWKS access token verification.
	if model.ServerConfig != nil {
		httpHandler := g.QualifiedGoIdent(protogen.GoImportPath("net/http").Ident("Handler"))
		verifierType := g.QualifiedGoIdent(protogen.GoImportPath("github.com/modelcontextprotocol/go-sdk/auth").Ident("TokenVerifier"))
		g.P("// New", fileName, "MCPHTTPHandler applies protobuf OAuth policy to SDK Streamable HTTP.")
		g.P("func New", fileName, "MCPHTTPHandler(server *", serverType, ", verifier ", verifierType, ") (", httpHandler, ", error) {")
		if oauth := model.ServerConfig.GetOauth(); oauth != nil {
			configType := g.QualifiedGoIdent(runtimeImport.Ident("OAuthResourceServer"))
			g.P("return ", g.QualifiedGoIdent(runtimeImport.Ident("NewConfiguredHTTPHandler")), "(server, &", configType, "{")
			g.P("ResourceURL:",quote(oauth.GetResourceUrl()),",")
			g.P("AuthorizationServers: []string{")
			for _, url := range oauth.GetAuthorizationServers() { g.P(quote(url),",") }
			g.P("},")
			g.P("Scopes: []string{")
			for _, scope := range oauth.GetScopes() { g.P(quote(scope),",") }
			g.P("},")
			g.P("Issuer:",quote(oauth.GetIssuer()),",")
			g.P("Audience:",quote(oauth.GetAudience()),",")
			g.P("JWKSURL:",quote(oauth.GetJwksUri()),",")
			g.P("MCPPath:",quote(oauth.GetMcpPath()),",")
			g.P("}, verifier)")
		} else {
			g.P("return ", g.QualifiedGoIdent(runtimeImport.Ident("NewConfiguredHTTPHandler")), "(server, nil, verifier)")
		}
		g.P("}")
		g.P()
	}
	return nil
}

func resourcesNeedImpl(resources []ResourceModel) bool {
	for _, res := range resources {
		if res.SourceFile == "" { return true }
	}
	return false
}
