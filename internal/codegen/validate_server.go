package codegen

import (
	"fmt"
	"mime"
	"net/url"
	"strings"
)

// validateServerModel catches configuration errors at protobuf generation time.
// Authentication remains enforced by the generated Go runtime, not by comments.
func validateServerModel(model FileModel) error {
	hasApps := false
	for _, service := range model.Services {
		for _, method := range service.Methods {
			for _, scope := range method.RequiredScopes {
				if scope == "" || strings.TrimSpace(scope) != scope || strings.ContainsAny(scope, " \t\r\n") {
					return fmt.Errorf("method %s: invalid required scope %q", method.ProtoFullName, scope)
				}
			}
			if method.AppUI != nil {
				hasApps = true
				u, err := url.Parse(method.AppUI.GetResourceUri())
				if err != nil || u.Scheme != "ui" || u.Host == "" {
					return fmt.Errorf("method %s: invalid app_ui resource_uri %q", method.ProtoFullName, method.AppUI.GetResourceUri())
				}
				for _, v := range method.AppUI.GetVisibility() {
					if v != "model" && v != "app" {
						return fmt.Errorf("method %s: invalid app_ui visibility %q", method.ProtoFullName, v)
					}
				}
			}
		}
	}
	for _, resource := range model.Resources {
		base, parameters, err := mime.ParseMediaType(resource.MIMEType)
		if err != nil {
			return fmt.Errorf("resource %s: invalid mime_type: %w", resource.ProtoFullName, err)
		}
		if base == "text/html" && parameters["profile"] == "mcp-app" {
			hasApps = true
		}
		if resource.AppUI != nil {
			hasApps = true
			if base != "text/html" || parameters["profile"] != "mcp-app" {
				return fmt.Errorf("resource %s: app_ui requires mime_type text/html;profile=mcp-app", resource.ProtoFullName)
			}
		}
	}
	if config := model.ServerConfig; config != nil {
		if apps := config.GetApps(); apps != nil && apps.Enabled != nil && !apps.GetEnabled() && hasApps {
			return fmt.Errorf("file %s: apps.enabled=false conflicts with MCP Apps tools/resources", model.ProtoPath)
		}
		if oauth := config.GetOauth(); oauth != nil {
			if err := requireHTTPS("resource_url", oauth.GetResourceUrl()); err != nil {
				return err
			}
			if len(oauth.GetAuthorizationServers()) == 0 {
				return fmt.Errorf("file %s: OAuth requires authorization_servers", model.ProtoPath)
			}
			for _, authServer := range oauth.GetAuthorizationServers() {
				if err := requireHTTPS("authorization_server", authServer); err != nil {
					return err
				}
			}
			if oauth.GetJwksUri() != "" {
				if err := requireHTTPS("jwks_uri", oauth.GetJwksUri()); err != nil {
					return err
				}
				if oauth.GetIssuer() == "" || oauth.GetAudience() == "" {
					return fmt.Errorf("file %s: JWKS verification requires issuer and audience", model.ProtoPath)
				}
			}
			if oauth.GetMcpPath() != "" && (!strings.HasPrefix(oauth.GetMcpPath(), "/") || strings.Contains(oauth.GetMcpPath(), "?")) {
				return fmt.Errorf("file %s: invalid OAuth mcp_path %q", model.ProtoPath, oauth.GetMcpPath())
			}
		}
	}
	return nil
}

func requireHTTPS(field, value string) error {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
		return fmt.Errorf("server OAuth: %s must be an absolute HTTPS URL, got %q", field, value)
	}
	return nil
}

func modelNeedsApps(model FileModel) bool {
	if model.ServerConfig != nil && model.ServerConfig.GetApps() != nil && model.ServerConfig.GetApps().Enabled != nil {
		return model.ServerConfig.GetApps().GetEnabled()
	}
	for _, service := range model.Services {
		for _, method := range service.Methods {
			if method.AppUI != nil {
				return true
			}
		}
	}
	for _, resource := range model.Resources {
		if resource.AppUI != nil || strings.EqualFold(resource.MIMEType, "text/html;profile=mcp-app") {
			return true
		}
	}
	return false
}
