package examples_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// standaloneSwiftExampleCommand builds examples/9_swift_standalone with
// SwiftPM, as its README does, and returns the server command.
func standaloneSwiftExampleCommand(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	if _, err := exec.LookPath("swift"); err != nil {
		t.Skip("swift not found in PATH; skipping Swift example")
	}
	projectDir := filepath.Join(repoRoot(t), "examples", "9_swift_standalone")
	build := exec.Command("swift", "build", "--package-path", projectDir)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("swift build failed in %s: %v\n%s", projectDir, err, out)
	}
	binPath, err := exec.Command("swift", "build", "--package-path", projectDir, "--show-bin-path").Output()
	if err != nil {
		t.Fatalf("swift build --show-bin-path: %v", err)
	}
	cmd := exec.Command(filepath.Join(strings.TrimSpace(string(binPath)), "NotebookServer"), args...)
	cmd.Dir = projectDir
	return cmd
}

func TestStandaloneSwiftExampleOverStdio(t *testing.T) {
	client := newStdioClient(t, standaloneSwiftExampleCommand(t))
	client.initialize()
	runStandaloneNodeNotebookExample(t, client, "Ship Swift support", "swift")
}
