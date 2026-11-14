package typescript_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestRuntimeUnitTests builds @easyp-tech/protoc-gen-mcp-runtime and runs its
// node:test suite.
func TestRuntimeUnitTests(t *testing.T) {
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skipf("npm is required for the TypeScript runtime tests: %v", err)
	}
	if _, err := os.Stat(filepath.Join("node_modules", ".bin", "tsc")); err != nil {
		t.Fatalf("TypeScript runtime dependencies are missing; run npm ci in runtime/typescript: %v", err)
	}
	cmd := exec.Command("npm", "test")
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("TypeScript runtime tests failed: %v\n%s", err, out)
	}
}
