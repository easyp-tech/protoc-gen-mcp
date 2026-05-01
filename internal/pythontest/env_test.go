package pythontest

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWithSetupLockRemovesStaleLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "python.lock")
	if err := os.WriteFile(lockPath, []byte("stale\n"), 0o644); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}
	staleTime := time.Now().Add(-staleSetupLock - time.Minute)
	if err := os.Chtimes(lockPath, staleTime, staleTime); err != nil {
		t.Fatalf("age stale lock: %v", err)
	}

	called := false
	if err := withSetupLock(lockPath, func() error {
		called = true
		return nil
	}); err != nil {
		t.Fatalf("withSetupLock: %v", err)
	}
	if !called {
		t.Fatal("withSetupLock did not call callback")
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("lock still exists after callback: %v", err)
	}
}
