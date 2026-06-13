package examplemcp_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	ensureJVMExamplesInstalledOnce sync.Once
	ensureJVMExamplesInstalledErr  error
	ensureJVMExamplesInstalledSkip string
)

func TestJavaServerOverStdio(t *testing.T) {
	root := repoRoot(t)
	ensureJVMExamplesInstalled(t, root)

	cmd := installedJVMServerCommand(root, "java-server")
	runServerOverStdioContract(t, cmd)
}

func TestKotlinServerOverStdio(t *testing.T) {
	root := repoRoot(t)
	ensureJVMExamplesInstalled(t, root)

	cmd := installedJVMServerCommand(root, "kotlin-server")
	runServerOverStdioContract(t, cmd)
}

func TestJavaServerRejectsInvalidInputOverStdio(t *testing.T) {
	runJVMInvalidInputTest(t, "java-server")
}

func TestKotlinServerRejectsInvalidInputOverStdio(t *testing.T) {
	runJVMInvalidInputTest(t, "kotlin-server")
}

func TestJavaServerRejectsInvalidOutputOverStdio(t *testing.T) {
	runJVMInvalidOutputTest(t, "java-server")
}

func TestKotlinServerRejectsInvalidOutputOverStdio(t *testing.T) {
	runJVMInvalidOutputTest(t, "kotlin-server")
}

func ensureJVMExamplesInstalled(t *testing.T, root string) {
	t.Helper()

	ensureJVMExamplesInstalledOnce.Do(func() {
		javaScript := filepath.Join(root, "examples/jvm/java-server/build/install/java-server/bin/java-server")
		kotlinScript := filepath.Join(root, "examples/jvm/kotlin-server/build/install/kotlin-server/bin/kotlin-server")
		if fileExists(javaScript) && fileExists(kotlinScript) {
			if _, err := exec.LookPath("java"); err != nil {
				ensureJVMExamplesInstalledSkip = "java not found in PATH; skipping JVM stdio tests"
			}
			return
		}
		if _, err := exec.LookPath("gradle"); err != nil {
			ensureJVMExamplesInstalledSkip = "gradle not found in PATH; skipping JVM stdio tests"
			return
		}
		if _, err := exec.LookPath("javac"); err != nil {
			ensureJVMExamplesInstalledSkip = "javac not found in PATH; skipping JVM stdio tests"
			return
		}

		cmd := exec.Command(
			"gradle",
			"--no-daemon",
			"-p", filepath.Join(root, "examples/jvm"),
			":java-server:installDist",
			":kotlin-server:installDist",
		)
		cmd.Dir = root
		output, err := cmd.CombinedOutput()
		if err != nil {
			ensureJVMExamplesInstalledErr = fmt.Errorf("install JVM example scripts: %w\n%s", err, output)
		}
	})

	if ensureJVMExamplesInstalledSkip != "" {
		t.Skip(ensureJVMExamplesInstalledSkip)
	}
	if ensureJVMExamplesInstalledErr != nil {
		t.Fatal(ensureJVMExamplesInstalledErr)
	}
}

func installedJVMServerCommand(root, target string) *exec.Cmd {
	script := filepath.Join(root, "examples/jvm", target, "build/install", target, "bin", target)
	cmd := exec.Command(script)
	cmd.Dir = root
	return cmd
}

func runJVMInvalidInputTest(t *testing.T, target string) {
	t.Helper()

	root := repoRoot(t)
	ensureJVMExamplesInstalled(t, root)

	cmd := installedJVMServerCommand(root, target)
	client := newStdioClient(t, cmd)
	client.initialize()

	_, rpcErr := client.callTool("example_CreateReport", map[string]any{"count": 0})
	if rpcErr == nil {
		t.Fatal("CallTool(CreateReport) unexpectedly succeeded with invalid input")
	}

	lower := strings.ToLower(rpcErr.Message)
	if !strings.Contains(lower, "invalid") {
		t.Fatalf("CallTool(CreateReport) error = %v, want invalid-input failure", rpcErr.Message)
	}
	normalized := strings.ReplaceAll(lower, "_", "")
	if !strings.Contains(normalized, "examplecreatereport") {
		t.Fatalf("CallTool(CreateReport) error = %v, want tool name in failure", rpcErr.Message)
	}
}

func runJVMInvalidOutputTest(t *testing.T, target string) {
	t.Helper()

	root := repoRoot(t)
	ensureJVMExamplesInstalled(t, root)

	cmd := installedJVMServerCommand(root, target)
	cmd.Env = append(os.Environ(), "PROTOC_GEN_MCP_JVM_INVALID_OUTPUT=create_report")

	client := newStdioClient(t, cmd)
	client.initialize()

	_, rpcErr := client.callTool("example_CreateReport", map[string]any{
		"city":    "Paris",
		"count":   2,
		"details": map[string]any{"label": "today"},
	})
	if rpcErr == nil {
		t.Fatal("CallTool(CreateReport) unexpectedly succeeded with invalid output schema")
	}

	lower := strings.ToLower(rpcErr.Message)
	if !strings.Contains(lower, "validate output") && !strings.Contains(lower, "output") {
		t.Fatalf("CallTool(CreateReport) error = %v, want output validation failure", rpcErr.Message)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
