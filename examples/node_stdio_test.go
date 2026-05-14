package examples_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStandaloneTypeScriptExampleOverStdio(t *testing.T) {
	root := repoRoot(t)
	projectDir := filepath.Join(root, "examples/8_typescript_standalone")
	buildStandaloneNodeExample(t, projectDir)

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "protoc-gen-mcp-standalone-typescript-example-test-client",
		Version: "v0.0.1",
	}, nil)

	session, err := client.Connect(ctx, &mcp.CommandTransport{
		Command: standaloneNodeExampleCommand(projectDir, "dist/server.js"),
	}, nil)
	if err != nil {
		t.Fatalf("client.Connect() over stdio failed: %v", err)
	}
	defer session.Close()

	runStandaloneNodeNotebookExample(t, session, "Ship TypeScript support", "typescript")
}

func TestStandaloneJavaScriptExampleOverStdio(t *testing.T) {
	root := repoRoot(t)
	projectDir := filepath.Join(root, "examples/9_javascript_standalone")
	buildStandaloneNodeExample(t, projectDir)

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "protoc-gen-mcp-standalone-javascript-example-test-client",
		Version: "v0.0.1",
	}, nil)

	session, err := client.Connect(ctx, &mcp.CommandTransport{
		Command: standaloneNodeExampleCommand(projectDir, "src/server.js"),
	}, nil)
	if err != nil {
		t.Fatalf("client.Connect() over stdio failed: %v", err)
	}
	defer session.Close()

	runStandaloneNodeNotebookExample(t, session, "Ship JavaScript consumption", "javascript")
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

func runStandaloneNodeNotebookExample(t *testing.T, session *mcp.ClientSession, title string, tag string) {
	t.Helper()

	ctx := context.Background()
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools() failed: %v", err)
	}
	findTool(t, tools.Tools, "notebook_CreateNote")
	findTool(t, tools.Tools, "notebook_SearchNotes")
	findTool(t, tools.Tools, "notebook_Health")

	createResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "notebook_CreateNote",
		Arguments: map[string]any{
			"title":   title,
			"body":    "Verify the generated Node MCP bindings are pleasant to use.",
			"tags":    []any{tag, "mcp"},
			"dueDate": "2026-05-30",
		},
	})
	if err != nil {
		t.Fatalf("CallTool(CreateNote) failed: %v", err)
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

	searchResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "notebook_SearchNotes",
		Arguments: map[string]any{
			"query": tag,
			"tags":  []any{"mcp"},
			"limit": 5,
		},
	})
	if err != nil {
		t.Fatalf("CallTool(SearchNotes) failed: %v", err)
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

	healthResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "notebook_Health",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool(Health) failed: %v", err)
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
