package swift_test

import (
	"os/exec"
	"testing"
)

// TestRuntimeUnitTests runs the XCTest suite of the Swift runtime, declared
// by the Package.swift at the repository root.
func TestRuntimeUnitTests(t *testing.T) {
	if _, err := exec.LookPath("swift"); err != nil {
		t.Skipf("swift is required for the Swift runtime tests: %v", err)
	}
	cmd := exec.Command("swift", "test", "--package-path", "../..")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Swift runtime tests failed: %v\n%s", err, out)
	}
}
