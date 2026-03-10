package codegen_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEasypGenerateExampleGolden(t *testing.T) {
	root := repoRoot(t)

	output := runEasyp(t, root, filepath.Join(root, "easyp.test.yaml"), "generate", "-p", "internal/testproto", "-r", ".")
	if strings.TrimSpace(output) == "" {
		t.Fatal("easyp generate returned empty output")
	}

	gotPath := filepath.Join(root, "internal/testproto/example/v1/example.mcp.go")
	wantPath := filepath.Join(root, "testdata/golden/example.mcp.go.golden")

	got, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("read generated file %q: %v", gotPath, err)
	}

	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read golden file %q: %v", wantPath, err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("generated file %s does not match golden snapshot %s", gotPath, wantPath)
	}
}

func TestGeneratedSchemasUseInterpretedStringLiterals(t *testing.T) {
	root := repoRoot(t)

	runEasyp(t, root, filepath.Join(root, "easyp.test.yaml"), "generate", "-p", "internal/testproto", "-r", ".")

	gotPath := filepath.Join(root, "internal/testproto/example/v1/example.mcp.go")
	got, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("read generated file %q: %v", gotPath, err)
	}

	generated := string(got)
	if strings.Contains(generated, "InputSchemaJSON = `") || strings.Contains(generated, "OutputSchemaJSON = `") {
		t.Fatalf("generated schema constants must not use raw string literals:\n%s", generated)
	}
	if !strings.Contains(generated, "`StringValue`") {
		t.Fatalf("generated schema descriptions should preserve backticks from proto comments:\n%s", generated)
	}
}

func TestEasypGenerateUnsupportedFails(t *testing.T) {
	root := repoRoot(t)
	configPath := filepath.Join(t.TempDir(), "easyp.unsupported.yaml")

	config := strings.Join([]string{
		"generate:",
		"  inputs:",
		"    - directory:",
		"        path: testdata/unsupported",
		"        root: \".\"",
		"  plugins:",
		"    - command: [\"go\", \"run\", \"./cmd/protoc-gen-mcp-go\"]",
		"      out: .",
		"      opts:",
		"        paths: source_relative",
		"",
	}, "\n")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	cmd := exec.Command("easyp", "--cfg", configPath, "generate", "-p", "testdata/unsupported", "-r", ".")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("easyp generate unexpectedly succeeded:\n%s", output)
	}

	if !strings.Contains(string(output), `well-known type "google.protobuf.Type" is not supported`) {
		t.Fatalf("unexpected easyp failure output:\n%s", output)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file path")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func runEasyp(t *testing.T, dir string, config string, args ...string) string {
	t.Helper()

	if _, err := exec.LookPath("easyp"); err != nil {
		t.Fatalf("easyp not found in PATH: %v", err)
	}

	cmdArgs := append([]string{"--cfg", config}, args...)
	cmd := exec.Command("easyp", cmdArgs...)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("easyp %v failed: %v\n%s", args, err, output)
	}

	return string(output)
}
