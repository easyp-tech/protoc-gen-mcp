package examples_test

import (
	"path/filepath"
	"testing"

	"github.com/easyp-tech/protoc-gen-mcp/internal/httpcontract"
)

var notebookShowcase = httpcontract.Showcase{
	MetadataPath:    "/.well-known/oauth-protected-resource/mcp",
	ScopesSupported: []string{"notebook:read", "notebook:write"},
	ScopedTool:      "notebook_DeleteNote",
	ScopedArgs:      map[string]any{"id": "note-1"},
}

func TestStandalonePythonExampleOverHTTP(t *testing.T) {
	cmd := standalonePythonExampleCommand(t, filepath.Join(repoRoot(t), "examples/5_python_standalone"))
	cmd.Args = append(cmd.Args, "--http", "--addr", "127.0.0.1:0", "--demo-auth")
	httpcontract.RunShowcase(t, cmd, notebookShowcase)
}

func TestStandaloneTypeScriptExampleOverHTTP(t *testing.T) {
	projectDir := filepath.Join(repoRoot(t), "examples/8_typescript_standalone")
	buildStandaloneNodeExample(t, projectDir)
	cmd := standaloneNodeExampleCommand(projectDir, "dist/server.js")
	cmd.Args = append(cmd.Args, "--http", "--addr", "127.0.0.1:0", "--demo-auth")
	httpcontract.RunShowcase(t, cmd, notebookShowcase)
}

var todoShowcase = httpcontract.Showcase{
	MetadataPath:    "/.well-known/oauth-protected-resource/mcp",
	ScopesSupported: []string{"todo:read", "todo:write"},
	ScopedTool:      "todo_DeleteTask",
	ScopedArgs:      map[string]any{"id": "task-1"},
}

func TestStandaloneJavaExampleOverHTTP(t *testing.T) {
	cmd := buildStandaloneJVMExample(t, "6_java_standalone", "todo-java-standalone")
	cmd.Args = append(cmd.Args, "--http", "--addr", "127.0.0.1:0", "--demo-auth")
	httpcontract.RunShowcase(t, cmd, todoShowcase)
}

func TestStandaloneKotlinExampleOverHTTP(t *testing.T) {
	cmd := buildStandaloneJVMExample(t, "7_kotlin_standalone", "todo-kotlin-standalone")
	cmd.Args = append(cmd.Args, "--http", "--addr", "127.0.0.1:0", "--demo-auth")
	httpcontract.RunShowcase(t, cmd, todoShowcase)
}

func TestStandaloneSwiftExampleOverHTTP(t *testing.T) {
	cmd := standaloneSwiftExampleCommand(t, "--http", "--addr", "127.0.0.1:0", "--demo-auth")
	httpcontract.RunShowcase(t, cmd, notebookShowcase)
}
