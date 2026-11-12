package codegen

import "google.golang.org/protobuf/compiler/protogen"

// Keep the original descriptor-path APIs source-compatible while steering new
// consumers toward the relocation-stable basename APIs. Functions are wrappers,
// rather than mutable package variables, and handlers are true type aliases.
func renderGoLegacyFileAliases(g *protogen.GeneratedFile, info goRenderInfo, model FileModel) {
	name, legacy := goFileAPIName(info.file), info.file.GoDescriptorIdent.GoName
	if name == legacy {
		return
	}
	mcpImport := protogen.GoImportPath("github.com/modelcontextprotocol/go-sdk/mcp")
	runtimeImport := protogen.GoImportPath("github.com/easyp-tech/protoc-gen-mcp/mcpruntime")
	alias := func(suffix string) {
		g.P("// Deprecated: use ", name, suffix, ".")
		g.P("type ", legacy, suffix, " = ", name, suffix)
		g.P()
	}
	if len(model.Prompts) > 0 {
		alias("PromptHandler")
		g.P("// Deprecated: use Register", name, "Prompts.")
		g.P("func Register", legacy, "Prompts(server *", mcpImport.Ident("Server"), ", impl ", name, "PromptHandler, opts ...", runtimeImport.Ident("RegisterOption"), ") error {")
		g.P("return Register", name, "Prompts(server, impl, opts...)")
		g.P("}")
		g.P()
	}
	if len(model.Resources) > 0 {
		alias("ResourceHandler")
		g.P("// Deprecated: use Register", name, "Resources.")
		g.P("func Register", legacy, "Resources(ctx ", protogen.GoImportPath("context").Ident("Context"), ", server *", mcpImport.Ident("Server"), ", impl ", name, "ResourceHandler, opts ...", runtimeImport.Ident("RegisterOption"), ") error {")
		g.P("return Register", name, "Resources(ctx, server, impl, opts...)")
		g.P("}")
		g.P()
	}
	if model.ServerConfig != nil || modelNeedsApps(model) {
		alias("MCPHandlers")
		g.P("// Deprecated: use New", name, "MCPServer.")
		g.P("func New", legacy, "MCPServer(ctx ", protogen.GoImportPath("context").Ident("Context"), ", handlers ", name, "MCPHandlers) (*", mcpImport.Ident("Server"), ", error) {")
		g.P("return New", name, "MCPServer(ctx, handlers)")
		g.P("}")
		g.P()
	}
	if model.ServerConfig != nil {
		g.P("// Deprecated: use New", name, "MCPHTTPHandler.")
		g.P("func New", legacy, "MCPHTTPHandler(server *", mcpImport.Ident("Server"), ", verifier ", protogen.GoImportPath("github.com/modelcontextprotocol/go-sdk/auth").Ident("TokenVerifier"), ") (", protogen.GoImportPath("net/http").Ident("Handler"), ", error) {")
		g.P("return New", name, "MCPHTTPHandler(server, verifier)")
		g.P("}")
		g.P()
	}
}
