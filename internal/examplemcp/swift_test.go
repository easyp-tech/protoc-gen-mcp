package examplemcp_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/easyp-tech/protoc-gen-mcp/internal/httpcontract"
	"github.com/easyp-tech/protoc-gen-mcp/internal/stdiotest"
)

var (
	swiftFixtureOnce   sync.Once
	swiftFixtureBinary string
	swiftFixtureSkip   string
	swiftFixtureErr    error
)

// swiftFixtureCommand builds the SwiftPM harness in internal/examplemcp/swift
// once and returns its server binary, run from the repository root.
func swiftFixtureCommand(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	root := repoRoot(t)
	swiftFixtureOnce.Do(func() {
		if _, err := exec.LookPath("swift"); err != nil {
			swiftFixtureSkip = "swift not found in PATH; skipping Swift fixture tests"
			return
		}
		packageDir := filepath.Join(root, "internal/examplemcp/swift")
		build := exec.Command("swift", "build", "--package-path", packageDir)
		if output, err := build.CombinedOutput(); err != nil {
			swiftFixtureErr = fmt.Errorf("swift build: %w\n%s", err, output)
			return
		}
		binPath, err := exec.Command("swift", "build", "--package-path", packageDir, "--show-bin-path").Output()
		if err != nil {
			swiftFixtureErr = fmt.Errorf("swift build --show-bin-path: %w", err)
			return
		}
		swiftFixtureBinary = filepath.Join(strings.TrimSpace(string(binPath)), "SwiftFixtureServer")
	})
	if swiftFixtureSkip != "" {
		t.Skip(swiftFixtureSkip)
	}
	if swiftFixtureErr != nil {
		t.Fatal(swiftFixtureErr)
	}
	cmd := exec.Command(swiftFixtureBinary, args...)
	cmd.Dir = root
	return cmd
}

func TestSwiftServerOverStdio(t *testing.T) {
	runServerOverStdioContract(t, swiftFixtureCommand(t))
}

func TestSwiftServerRejectsInvalidInputOverStdio(t *testing.T) {
	assertRejectsInvalidInput(t, swiftFixtureCommand(t))
}

func TestSwiftServerRejectsInvalidOutputOverStdio(t *testing.T) {
	cmd := swiftFixtureCommand(t)
	cmd.Env = append(os.Environ(), "PROTOC_GEN_MCP_INVALID_OUTPUT=create_report")
	assertRejectsInvalidOutput(t, cmd)
}

func TestSwiftResourcesOverStdio(t *testing.T) {
	stdiotest.RunResourcesPromptsContract(t, swiftFixtureCommand(t, "resources"), stdiotest.ContractOptions{})
}

func TestSwiftHTTPOAuthContract(t *testing.T) {
	httpcontract.Run(t, swiftFixtureCommand(t, "http"))
}
