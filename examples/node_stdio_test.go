package examples_test

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestStandaloneTypeScriptExampleOverStdio(t *testing.T) {
	root := repoRoot(t)
	projectDir := filepath.Join(root, "examples/8_typescript_standalone")
	buildStandaloneNodeExample(t, projectDir)

	cmd := standaloneNodeExampleCommand(projectDir, "dist/server.js")
	client := newStdioClient(t, cmd)
	client.initialize()

	runStandaloneNodeNotebookExample(t, client, "Ship TypeScript support", "typescript")
}

func TestStandaloneJavaScriptExampleOverStdio(t *testing.T) {
	root := repoRoot(t)
	projectDir := filepath.Join(root, "examples/9_javascript_standalone")
	buildStandaloneNodeExample(t, projectDir)

	cmd := standaloneNodeExampleCommand(projectDir, "src/server.js")
	client := newStdioClient(t, cmd)
	client.initialize()

	runStandaloneNodeNotebookExample(t, client, "Ship JavaScript consumption", "javascript")
}

func buildStandaloneNodeExample(t *testing.T, projectDir string) {
	t.Helper()

	cmd := exec.Command("make", "build")
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make build failed in %s:\n%s", projectDir, string(output))
	}
}

func standaloneNodeExampleCommand(projectDir string, script string) *exec.Cmd {
	cmd := exec.Command("node", script)
	cmd.Dir = projectDir
	return cmd
}

func runStandaloneNodeNotebookExample(t *testing.T, client *stdioClient, title string, tag string) {
	t.Helper()

	tools := client.listTools()
	findTool(t, tools.Tools, "notebook_CreateNote")
	findTool(t, tools.Tools, "notebook_SearchNotes")
	findTool(t, tools.Tools, "notebook_Health")

	createResult, rpcErr := client.callTool("notebook_CreateNote", map[string]any{
		"title":   title,
		"body":    "Verify the generated Node MCP bindings are pleasant to use.",
		"tags":    []any{tag, "mcp"},
		"dueDate": "2026-05-30",
	})
	if rpcErr != nil {
		t.Fatalf("CallTool(CreateNote) failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}
	if createResult.IsError {
		t.Fatalf("CreateNote returned tool error: %+v", createResult)
	}
	assertTextStructuredContentMatch(t, "notebook_CreateNote", createResult)
	createStructured := decodeMap(t, createResult.StructuredContent)
	note, ok := createStructured["note"].(map[string]any)
	if !ok {
		t.Fatalf("created note has type %T, want map[string]any", createStructured["note"])
	}
	if got := note["title"]; got != title {
		t.Fatalf("created note.title = %v, want %s", got, title)
	}

	searchResult, rpcErr := client.callTool("notebook_SearchNotes", map[string]any{
		"query": tag,
		"tags":  []any{"mcp"},
		"limit": 5,
	})
	if rpcErr != nil {
		t.Fatalf("CallTool(SearchNotes) failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}
	if searchResult.IsError {
		t.Fatalf("SearchNotes returned tool error: %+v", searchResult)
	}
	assertTextStructuredContentMatch(t, "notebook_SearchNotes", searchResult)
	searchStructured := decodeMap(t, searchResult.StructuredContent)
	notes, ok := searchStructured["notes"].([]any)
	if !ok || len(notes) != 1 {
		t.Fatalf("notes = %T %v, want one matching note", searchStructured["notes"], searchStructured["notes"])
	}

	healthResult, rpcErr := client.callTool("notebook_Health", map[string]any{})
	if rpcErr != nil {
		t.Fatalf("CallTool(Health) failed: code=%d msg=%s", rpcErr.Code, rpcErr.Message)
	}
	if healthResult.IsError {
		t.Fatalf("Health returned tool error: %+v", healthResult)
	}
	assertTextStructuredContentMatch(t, "notebook_Health", healthResult)
	healthStructured := decodeMap(t, healthResult.StructuredContent)
	if got := healthStructured["noteCount"]; got != 1.0 {
		t.Fatalf("health.noteCount = %v, want 1", got)
	}
}
