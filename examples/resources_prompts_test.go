package examples_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// assertNotebookResourcesAndPrompts checks the note resource template and the
// review_note prompt that the Python and TypeScript notebook examples serve.
func assertNotebookResourcesAndPrompts(t *testing.T, client *stdioClient, note map[string]any) {
	t.Helper()

	id, _ := note["id"].(string)
	uri := "notebook://notes/" + id
	var listed struct {
		Resources []struct {
			URI string `json:"uri"`
		} `json:"resources"`
	}
	decodeResult(t, client.call("resources/list", map[string]any{}), &listed)
	found := false
	for _, resource := range listed.Resources {
		found = found || resource.URI == uri
	}
	if !found {
		t.Fatalf("resources/list = %+v, want %s", listed.Resources, uri)
	}

	var read struct {
		Contents []struct {
			Text string `json:"text"`
		} `json:"contents"`
	}
	decodeResult(t, client.call("resources/read", map[string]any{"uri": uri}), &read)
	var body map[string]any
	if len(read.Contents) != 1 || json.Unmarshal([]byte(read.Contents[0].Text), &body) != nil || body["title"] != note["title"] {
		t.Fatalf("resources/read %s = %+v, want the note as ProtoJSON", uri, read.Contents)
	}

	var prompt struct {
		Messages []struct {
			Content struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"messages"`
	}
	decodeResult(t, client.call("prompts/get", map[string]any{
		"name":      "review_note",
		"arguments": map[string]string{"noteId": id},
	}), &prompt)
	title, _ := note["title"].(string)
	if len(prompt.Messages) != 1 || !strings.Contains(prompt.Messages[0].Content.Text, title) {
		t.Fatalf("prompts/get review_note = %+v, want the note title", prompt.Messages)
	}
}

// assertTodoResourcesAndPrompts checks the task resource template and the
// plan_day prompt that the Java and Kotlin todo examples serve.
func assertTodoResourcesAndPrompts(t *testing.T, client *stdioClient, task map[string]any) {
	t.Helper()

	id, _ := task["id"].(string)
	uri := "todo://tasks/" + id
	var templates struct {
		ResourceTemplates []struct {
			URITemplate string `json:"uriTemplate"`
		} `json:"resourceTemplates"`
	}
	decodeResult(t, client.call("resources/templates/list", map[string]any{}), &templates)
	if len(templates.ResourceTemplates) != 1 || templates.ResourceTemplates[0].URITemplate != "todo://tasks/{id}" {
		t.Fatalf("resources/templates/list = %+v", templates.ResourceTemplates)
	}

	var read struct {
		Contents []struct {
			Text string `json:"text"`
		} `json:"contents"`
	}
	decodeResult(t, client.call("resources/read", map[string]any{"uri": uri}), &read)
	var body map[string]any
	if len(read.Contents) != 1 || json.Unmarshal([]byte(read.Contents[0].Text), &body) != nil || body["title"] != task["title"] {
		t.Fatalf("resources/read %s = %+v, want the task as ProtoJSON", uri, read.Contents)
	}

	var prompt struct {
		Messages []struct {
			Content struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"messages"`
	}
	decodeResult(t, client.call("prompts/get", map[string]any{
		"name":      "plan_day",
		"arguments": map[string]string{"focus": "shipping"},
	}), &prompt)
	title, _ := task["title"].(string)
	if len(prompt.Messages) != 1 || !strings.Contains(prompt.Messages[0].Content.Text, title) {
		t.Fatalf("prompts/get plan_day = %+v, want the open task title", prompt.Messages)
	}
}

func decodeResult(t *testing.T, resp jsonrpcResponse, out any) {
	t.Helper()
	if resp.Error != nil {
		t.Fatalf("JSON-RPC error: code=%d msg=%s", resp.Error.Code, resp.Error.Message)
	}
	if err := json.Unmarshal(resp.Result, out); err != nil {
		t.Fatalf("decode result: %v (%s)", err, resp.Result)
	}
}
