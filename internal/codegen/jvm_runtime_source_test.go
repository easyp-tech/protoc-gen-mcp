package codegen

import (
	"os"
	"path/filepath"
	"testing"
)

// withJVMRuntimeSource appends the runtime library a generated JVM sidecar
// links against, so contract tests can assert generated calls and the
// runtime behaviour they rely on in one place.
func withJVMRuntimeSource(t *testing.T, generated string, lang Language) string {
	t.Helper()
	path := filepath.Join(repoRoot(t), "runtime", "jvm", "runtime", "src", "main", "java", "tech", "easyp", "mcp", "runtime", "McpRuntime.java")
	if lang == LanguageKotlin {
		path = filepath.Join(repoRoot(t), "runtime", "jvm", "runtime-kotlin", "src", "main", "kotlin", "tech", "easyp", "mcp", "runtime", "kotlin", "McpRuntime.kt")
	}
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read JVM runtime source: %v", err)
	}
	return generated + "\n" + string(source)
}
