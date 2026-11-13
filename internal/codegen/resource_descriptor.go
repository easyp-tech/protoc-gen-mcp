package codegen

import (
	"encoding/json"
	"fmt"

	mcpoptionsv1 "github.com/easyp-tech/protoc-gen-mcp/mcp/options/v1"
)

// resourceContentKind says how a non-Go runtime turns a handler result into
// MCP resource contents. It mirrors mcpruntime.MarshalResourceContent and
// mcpruntime.RegisterEmbeddedResource.
type resourceContentKind int

const (
	// resourceContentProtoJSON serializes the whole message as ProtoJSON text.
	resourceContentProtoJSON resourceContentKind = iota
	// resourceContentTextField serves a singular string field as raw text.
	resourceContentTextField
	// resourceContentBlobField serves a singular bytes field as a base64 blob.
	resourceContentBlobField
	// resourceContentSourceFile serves the str or bytes the handler returns
	// for a source_file resource, which only Go can embed at build time.
	resourceContentSourceFile
)

func resourceContent(resource ResourceModel) resourceContentKind {
	switch {
	case resource.SourceFile != "":
		return resourceContentSourceFile
	case resource.ContentField == "":
		return resourceContentProtoJSON
	case resource.ContentBytes:
		return resourceContentBlobField
	default:
		return resourceContentTextField
	}
}

// resourceDescriptorJSON renders the resources/list (or resources/templates/list)
// entry for a resource without its name, which runtimes prefix with the
// registration namespace. Keys match the Go SDK wire format.
func resourceDescriptorJSON(resource ResourceModel) (string, error) {
	descriptor := map[string]any{}
	if resource.IsTemplate {
		descriptor["uriTemplate"] = resource.URITemplate
	} else {
		descriptor["uri"] = resource.URI
	}
	if resource.Description != "" {
		descriptor["description"] = resource.Description
	}
	if resource.MIMEType != "" {
		descriptor["mimeType"] = resource.MIMEType
	}
	if annotations := resourceAnnotationsMap(resource.Annotations); annotations != nil {
		descriptor["annotations"] = annotations
	}
	if len(resource.Icons) > 0 {
		icons := make([]map[string]any, 0, len(resource.Icons))
		for _, icon := range resource.Icons {
			if theme := icon.GetTheme(); theme != "" && theme != "light" && theme != "dark" {
				return "", fmt.Errorf("resource %s: unsupported icon theme %q", resource.ProtoFullName, theme)
			}
			icons = append(icons, iconMap(icon))
		}
		descriptor["icons"] = icons
	}
	if meta := appResourceMetaMap(resource.AppUI); meta != nil {
		descriptor["_meta"] = meta
	}
	encoded, err := json.Marshal(descriptor)
	if err != nil {
		return "", fmt.Errorf("resource %s: encode descriptor: %w", resource.ProtoFullName, err)
	}
	return string(encoded), nil
}

// resourceMetaJSON renders the _meta attached to resources/read contents, or
// "" when the resource has none.
func resourceMetaJSON(resource ResourceModel) string {
	meta := appResourceMetaMap(resource.AppUI)
	if meta == nil {
		return ""
	}
	encoded, _ := json.Marshal(meta)
	return string(encoded)
}

func resourceAnnotationsMap(annotations *mcpoptionsv1.ResourceAnnotations) map[string]any {
	if annotations == nil {
		return nil
	}
	result := map[string]any{}
	var audience []string
	for _, role := range annotations.GetAudience() {
		switch role {
		case mcpoptionsv1.ResourceAudience_RESOURCE_AUDIENCE_USER:
			audience = append(audience, "user")
		case mcpoptionsv1.ResourceAudience_RESOURCE_AUDIENCE_ASSISTANT:
			audience = append(audience, "assistant")
		}
	}
	if len(audience) > 0 {
		result["audience"] = audience
	}
	if annotations.Priority != nil {
		result["priority"] = annotations.GetPriority()
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func iconMap(icon *mcpoptionsv1.Icon) map[string]any {
	result := map[string]any{"src": icon.GetSrc()}
	if icon.GetMimeType() != "" {
		result["mimeType"] = icon.GetMimeType()
	}
	if len(icon.GetSizes()) > 0 {
		result["sizes"] = icon.GetSizes()
	}
	if icon.GetTheme() != "" {
		result["theme"] = icon.GetTheme()
	}
	return result
}

// appResourceMetaMap mirrors mcpruntime.AppResourceMetadata.
func appResourceMetaMap(ui *mcpoptionsv1.AppResourceOptions) map[string]any {
	if ui == nil {
		return nil
	}
	app := map[string]any{}
	if csp := ui.GetCsp(); csp != nil {
		policy := map[string]any{}
		for key, values := range map[string][]string{
			"connectDomains":  csp.GetConnectDomains(),
			"resourceDomains": csp.GetResourceDomains(),
			"frameDomains":    csp.GetFrameDomains(),
			"baseUriDomains":  csp.GetBaseUriDomains(),
		} {
			if len(values) > 0 {
				policy[key] = values
			}
		}
		if len(policy) > 0 {
			app["csp"] = policy
		}
	}
	if perms := ui.GetPermissions(); perms != nil {
		permissions := map[string]any{}
		for key, enabled := range map[string]bool{
			"camera":         perms.GetCamera(),
			"microphone":     perms.GetMicrophone(),
			"geolocation":    perms.GetGeolocation(),
			"clipboardWrite": perms.GetClipboardWrite(),
		} {
			if enabled {
				permissions[key] = map[string]any{}
			}
		}
		if len(permissions) > 0 {
			app["permissions"] = permissions
		}
	}
	if ui.GetDomain() != "" {
		app["domain"] = ui.GetDomain()
	}
	if ui.PrefersBorder != nil {
		app["prefersBorder"] = ui.GetPrefersBorder()
	}
	if len(app) == 0 {
		return nil
	}
	return map[string]any{"ui": app}
}
