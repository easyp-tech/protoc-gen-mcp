package python_test

import (
	"testing"

	"github.com/easyp-tech/protoc-gen-mcp/internal/pythontest"
)

// TestRuntimeUnitTests runs the protoc-gen-mcp-runtime unittest suite in the
// isolated Python test environment.
func TestRuntimeUnitTests(t *testing.T) {
	cmd := pythontest.Command(t, "-m", "unittest", "discover", "-s", "tests", "-t", ".")
	cmd.Dir = pythontest.RuntimeDir()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc-gen-mcp-runtime unit tests failed: %v\n%s", err, out)
	}
}
