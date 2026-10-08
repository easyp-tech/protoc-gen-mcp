package codegen

import (
	"fmt"
	"strings"

	options "github.com/easyp-tech/protoc-gen-mcp/mcp/options/v1"
	"google.golang.org/protobuf/compiler/protogen"
)

func quotedStrings(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values { quoted = append(quoted, quote(value)) }
	return "[]string{" + strings.Join(quoted, ",") + "}"
}

// stringifyGoAppResourceMeta constructs SDK _meta.ui without losing typed
// protobuf policy: CSP allowlists and browser permissions live on the resource.
func stringifyGoAppResourceMeta(g *protogen.GeneratedFile, ui *options.AppResourceOptions) string {
	if ui == nil { return "nil" }
	pkg := protogen.GoImportPath("github.com/easyp-tech/protoc-gen-mcp/mcpruntime")
	metadata := g.QualifiedGoIdent(pkg.Ident("AppResourceMetadata"))
	config := g.QualifiedGoIdent(pkg.Ident("AppResourceConfig"))
	cspType := g.QualifiedGoIdent(pkg.Ident("AppResourceCSP"))
	permissionsType := g.QualifiedGoIdent(pkg.Ident("AppResourcePermissions"))
	var fields []string
	if csp := ui.GetCsp(); csp != nil {
		cspFields := []string{
			"ConnectDomains:" + quotedStrings(csp.GetConnectDomains()),
			"ResourceDomains:" + quotedStrings(csp.GetResourceDomains()),
			"FrameDomains:" + quotedStrings(csp.GetFrameDomains()),
			"BaseURIDomains:" + quotedStrings(csp.GetBaseUriDomains()),
		}
		fields = append(fields, "CSP:"+cspType+"{"+strings.Join(cspFields,",")+"}")
	}
	if perms := ui.GetPermissions(); perms != nil {
		fields = append(fields, fmt.Sprintf(
			"Permissions:%s{Camera:%t,Microphone:%t,Geolocation:%t,ClipboardWrite:%t}",
			permissionsType,perms.GetCamera(),perms.GetMicrophone(),perms.GetGeolocation(),perms.GetClipboardWrite()))
	}
	if ui.GetDomain() != "" { fields = append(fields, "Domain:"+quote(ui.GetDomain())) }
	if ui.PrefersBorder != nil {
		ptr := g.QualifiedGoIdent(protogen.GoImportPath("google.golang.org/protobuf/proto").Ident("Bool"))
		fields = append(fields, fmt.Sprintf("PrefersBorder:%s(%t)",ptr,ui.GetPrefersBorder()))
	}
	return metadata+"("+config+"{"+strings.Join(fields,",")+"})"
}
