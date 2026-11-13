package stdiotest

import (
	"encoding/base64"
	"encoding/json"
	"os/exec"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// ResourceNotFoundCode is the JSON-RPC error code for unknown resources
// (Invalid Params, MCP SEP-2164).
const ResourceNotFoundCode = -32602

// legacyResourceNotFoundCode predates SEP-2164. The Kotlin SDK (0.11) still
// answers unknown resources with it from its own resources/read handler.
const legacyResourceNotFoundCode = -32002

// Resource mirrors an MCP resource or resource template listing entry.
type Resource struct {
	URI         string          `json:"uri,omitempty"`
	URITemplate string          `json:"uriTemplate,omitempty"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	MIMEType    string          `json:"mimeType,omitempty"`
	Annotations json.RawMessage `json:"annotations,omitempty"`
	Icons       json.RawMessage `json:"icons,omitempty"`
	Meta        json.RawMessage `json:"_meta,omitempty"`
}

// ResourceContents mirrors one entry of a resources/read result.
type ResourceContents struct {
	URI      string          `json:"uri"`
	MIMEType string          `json:"mimeType,omitempty"`
	Text     *string         `json:"text,omitempty"`
	Blob     *string         `json:"blob,omitempty"`
	Meta     json.RawMessage `json:"_meta,omitempty"`
}

// RunResourcesPromptsContract checks the resources and prompts generated from
// internal/testproto/resources/v1 and internal/testproto/prompts/v1. The server
// behind cmd must implement the fixture handlers the same way as
// internal/examplemcp.ResourcesHandler and PromptsHandler.
func RunResourcesPromptsContract(t *testing.T, cmd *exec.Cmd) {
	t.Helper()

	client := NewClient(t, cmd)
	client.Initialize()

	var initialized struct {
		Capabilities map[string]json.RawMessage `json:"capabilities"`
	}
	if err := json.Unmarshal(client.InitializeResult(), &initialized); err != nil {
		t.Fatalf("decode initialize result: %v", err)
	}
	for _, capability := range []string{"resources", "prompts"} {
		if _, ok := initialized.Capabilities[capability]; !ok {
			t.Errorf("initialize capabilities missing %q: %s", capability, client.InitializeResult())
		}
	}

	checkResourceList(t, client)
	checkTemplateList(t, client)
	checkResourceReads(t, client)
	checkPrompts(t, client)
}

func checkResourceList(t *testing.T, client *Client) {
	t.Helper()

	var listed struct {
		Resources []Resource `json:"resources"`
	}
	client.Result("resources/list", map[string]any{}, &listed)

	byURI := map[string]Resource{}
	for _, resource := range listed.Resources {
		byURI[resource.URI] = resource
	}
	for _, uri := range []string{
		"server://status",
		"users://ada/profile",
		"docs://example/intro",
		"skill://example/SKILL.md",
		"blob://example/attachment",
		"ui://example/report",
	} {
		if _, ok := byURI[uri]; !ok {
			t.Errorf("resources/list missing %q: %+v", uri, listed.Resources)
		}
	}

	status := byURI["server://status"]
	if status.Name != "server_status" || status.MIMEType != "application/json" ||
		status.Description != "Current server status and health information" {
		t.Errorf("server_status metadata = %+v", status)
	}
	var annotations struct {
		Audience []string `json:"audience"`
		Priority float64  `json:"priority"`
	}
	decode(t, "server_status annotations", status.Annotations, &annotations)
	if !slices.Equal(annotations.Audience, []string{"user", "assistant"}) || annotations.Priority != 0.8 {
		t.Errorf("server_status annotations = %s", status.Annotations)
	}
	var icons []map[string]any
	decode(t, "server_status icons", status.Icons, &icons)
	if len(icons) != 1 || icons[0]["src"] != "https://example.com/status.svg" {
		t.Errorf("server_status icons = %s", status.Icons)
	}

	checkAppMeta(t, "resources/list ui://example/report", byURI["ui://example/report"].Meta)
}

func checkTemplateList(t *testing.T, client *Client) {
	t.Helper()

	var listed struct {
		ResourceTemplates []Resource `json:"resourceTemplates"`
	}
	client.Result("resources/templates/list", map[string]any{}, &listed)

	var templates []string
	for _, template := range listed.ResourceTemplates {
		templates = append(templates, template.URITemplate)
	}
	slices.Sort(templates)
	want := []string{"docs://example/{name}", "projects://{project_id}/documents/{document_id}", "users://{user_id}/profile"}
	if !slices.Equal(templates, want) {
		t.Errorf("resources/templates/list = %v, want %v", templates, want)
	}
}

func checkResourceReads(t *testing.T, client *Client) {
	t.Helper()

	status := readOne(t, client, "server://status", "application/json")
	assertJSONText(t, status, map[string]any{"healthy": true, "uptimeSeconds": "123", "version": "v0.0.1"})

	profile := readOne(t, client, "users://ada%20l/profile", "application/json")
	assertJSONText(t, profile, map[string]any{
		"userId":      "ada l",
		"displayName": "Ada Lovelace",
		"email":       "ada l@example.com",
		"active":      true,
	})

	document := readOne(t, client, "projects://p1/documents/d1", "application/json")
	assertJSONText(t, document, map[string]any{"title": "doc d1 in p1", "body": "body", "author": "Ada"})

	plain := readOne(t, client, "docs://example/intro", "text/plain")
	if plain.Text == nil || *plain.Text != "document: intro" {
		t.Errorf("docs://example/intro text = %v, want raw %q", plain.Text, "document: intro")
	}

	skill := readOne(t, client, "skill://example/SKILL.md", "text/markdown")
	if skill.Text == nil || !strings.HasPrefix(*skill.Text, "# Example MCP Skill") || !strings.Contains(*skill.Text, "## Instructions") {
		t.Errorf("SKILL.md was not served as raw Markdown: %v", skill.Text)
	}

	binary := readOne(t, client, "blob://example/attachment", "application/octet-stream")
	if binary.Blob == nil {
		t.Errorf("blob://example/attachment has no blob: %+v", binary)
	} else if raw, err := base64.StdEncoding.DecodeString(*binary.Blob); err != nil || !slices.Equal(raw, []byte{0, 1, 2, 255}) {
		t.Errorf("blob://example/attachment blob = %q (%v), want base64 of 00 01 02 ff", *binary.Blob, err)
	}

	page := readOne(t, client, "ui://example/report", "text/html;profile=mcp-app")
	if page.Text == nil || !strings.HasPrefix(*page.Text, "<!doctype html>") || !strings.Contains(*page.Text, "Example MCP App") {
		t.Errorf("ui://example/report was not served as HTML: %v", page.Text)
	}
	checkAppMeta(t, "resources/read ui://example/report", page.Meta)

	for _, uri := range []string{"nothing://here", "users://ada/settings"} {
		resp := client.Call("resources/read", map[string]any{"uri": uri})
		if resp.Error == nil || (resp.Error.Code != ResourceNotFoundCode && resp.Error.Code != legacyResourceNotFoundCode) {
			t.Errorf("resources/read %s = %+v, want error code %d", uri, resp.Error, ResourceNotFoundCode)
		}
	}
	if resp := client.Call("resources/read", map[string]any{"uri": "users://ada%zz/profile"}); resp.Error == nil {
		t.Errorf("resources/read with a malformed escape unexpectedly succeeded: %s", resp.Result)
	}
}

func checkPrompts(t *testing.T, client *Client) {
	t.Helper()

	var listed struct {
		Prompts []struct {
			Name      string `json:"name"`
			Arguments []struct {
				Name     string `json:"name"`
				Required bool   `json:"required"`
			} `json:"arguments"`
		} `json:"prompts"`
	}
	client.Result("prompts/list", map[string]any{}, &listed)
	var names []string
	for _, prompt := range listed.Prompts {
		names = append(names, prompt.Name)
		if prompt.Name == "code_review" {
			required := map[string]bool{}
			for _, argument := range prompt.Arguments {
				required[argument.Name] = argument.Required
			}
			if !reflect.DeepEqual(required, map[string]bool{"code": true, "language": true, "focusArea": false}) {
				t.Errorf("code_review arguments = %+v", prompt.Arguments)
			}
		}
	}
	slices.Sort(names)
	if want := []string{"code_review", "explain_error", "summarize"}; !slices.Equal(names, want) {
		t.Errorf("prompts/list = %v, want %v", names, want)
	}

	var got struct {
		Messages []struct {
			Role    string `json:"role"`
			Content struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"messages"`
	}
	client.Result("prompts/get", map[string]any{
		"name":      "code_review",
		"arguments": map[string]string{"code": "print(1)", "language": "python"},
	}, &got)
	if len(got.Messages) != 1 || got.Messages[0].Role != "user" || got.Messages[0].Content.Type != "text" ||
		got.Messages[0].Content.Text != "Review this python code: print(1)" {
		t.Errorf("prompts/get code_review = %+v", got.Messages)
	}

	if resp := client.Call("prompts/get", map[string]any{
		"name":      "code_review",
		"arguments": map[string]string{"code": "print(1)"},
	}); resp.Error == nil {
		t.Errorf("prompts/get without a required argument unexpectedly succeeded: %s", resp.Result)
	}
	if resp := client.Call("prompts/get", map[string]any{"name": "missing_prompt"}); resp.Error == nil {
		t.Errorf("prompts/get of an unknown prompt unexpectedly succeeded: %s", resp.Result)
	}
}

func readOne(t *testing.T, client *Client, uri, mimeType string) ResourceContents {
	t.Helper()
	var read struct {
		Contents []ResourceContents `json:"contents"`
	}
	client.Result("resources/read", map[string]any{"uri": uri}, &read)
	if len(read.Contents) != 1 {
		t.Fatalf("resources/read %s returned %d contents, want 1", uri, len(read.Contents))
	}
	contents := read.Contents[0]
	if contents.URI != uri || contents.MIMEType != mimeType {
		t.Errorf("resources/read %s contents uri=%q mimeType=%q, want uri=%q mimeType=%q", uri, contents.URI, contents.MIMEType, uri, mimeType)
	}
	return contents
}

func assertJSONText(t *testing.T, contents ResourceContents, want map[string]any) {
	t.Helper()
	if contents.Text == nil {
		t.Errorf("resources/read %s has no text: %+v", contents.URI, contents)
		return
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(*contents.Text), &got); err != nil {
		t.Errorf("resources/read %s text is not JSON: %v (%q)", contents.URI, err, *contents.Text)
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("resources/read %s = %v, want %v", contents.URI, got, want)
	}
}

func checkAppMeta(t *testing.T, where string, raw json.RawMessage) {
	t.Helper()
	var meta struct {
		UI struct {
			CSP struct {
				ConnectDomains  []string `json:"connectDomains"`
				ResourceDomains []string `json:"resourceDomains"`
			} `json:"csp"`
			Permissions struct {
				ClipboardWrite any `json:"clipboardWrite"`
			} `json:"permissions"`
			PrefersBorder bool `json:"prefersBorder"`
		} `json:"ui"`
	}
	decode(t, where+" _meta", raw, &meta)
	if !slices.Equal(meta.UI.CSP.ConnectDomains, []string{"https://api.example.com"}) ||
		!slices.Equal(meta.UI.CSP.ResourceDomains, []string{"https://cdn.example.com"}) ||
		meta.UI.Permissions.ClipboardWrite == nil || !meta.UI.PrefersBorder {
		t.Errorf("%s _meta.ui = %s", where, raw)
	}
}

func decode(t *testing.T, what string, raw json.RawMessage, out any) {
	t.Helper()
	if len(raw) == 0 {
		t.Errorf("%s is missing", what)
		return
	}
	if err := json.Unmarshal(raw, out); err != nil {
		t.Errorf("decode %s: %v (%s)", what, err, raw)
	}
}
