package lock

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"raioz/internal/workspace"
)

func TestAcquire(t *testing.T) {
	// Create temporary workspace
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{
		Root: tmpDir,
	}

	t.Run("successful acquire", func(t *testing.T) {
		lock, err := Acquire(ws)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if lock == nil {
			t.Fatal("Expected lock, got nil")
		}
		if lock.ws != ws {
			t.Errorf("Expected workspace %v, got %v", ws, lock.ws)
		}
		if lock.path != filepath.Join(ws.Root, lockFileName) {
			t.Errorf("Expected path %s, got %s", filepath.Join(ws.Root, lockFileName), lock.path)
		}

		// Verify lock file exists
		if _, err := os.Stat(lock.path); os.IsNotExist(err) {
			t.Error("Lock file should exist")
		}

		// Cleanup
		lock.Release()
	})

	t.Run("lock already exists", func(t *testing.T) {
		// Create first lock
		lock1, err := Acquire(ws)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		defer lock1.Release()

		// Try to acquire second lock (should fail)
		lock2, err := Acquire(ws)
		if err == nil {
			lock2.Release()
			t.Fatal("Expected error when lock already exists")
		}
		if lock2 != nil {
			t.Error("Expected nil lock when acquisition fails")
		}
		if err.Error() == "" {
			t.Error("Expected error message")
		}
	})
}

func TestRelease(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{
		Root: tmpDir,
	}

	t.Run("successful release", func(t *testing.T) {
		lock, err := Acquire(ws)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		lockPath := lock.path

		// Verify lock file exists
		if _, err := os.Stat(lockPath); os.IsNotExist(err) {
			t.Error("Lock file should exist before release")
		}

		// Release lock
		if err := lock.Release(); err != nil {
			t.Fatalf("Expected no error on release, got %v", err)
		}

		// Verify lock file is removed
		if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
			t.Error("Lock file should be removed after release")
		}
	})

	t.Run("release nil lock", func(t *testing.T) {
		lock := &Lock{}
		if err := lock.Release(); err != nil {
			t.Errorf("Expected no error releasing nil lock, got %v", err)
		}
	})

	t.Run("release with closed file", func(t *testing.T) {
		lock, err := Acquire(ws)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Close file manually
		lock.file.Close()
		lock.file = nil

		// Release should still work (removes file)
		if err := lock.Release(); err != nil {
			t.Errorf("Expected no error releasing with closed file, got %v", err)
		}
	})

	// Callers release the lock early to allow other workspace
	// projects to run `raioz up`, then a deferred Release() runs at process
	// exit. The second call must be a silent no-op.
	t.Run("double release is idempotent", func(t *testing.T) {
		lock, err := Acquire(ws)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if err := lock.Release(); err != nil {
			t.Fatalf("First Release: %v", err)
		}
		if err := lock.Release(); err != nil {
			t.Errorf("Second Release should be no-op, got %v", err)
		}
	})

	// After early release, a new Acquire on the same workspace
	// must succeed — that's the whole point of the change.
	t.Run("acquire succeeds after early release", func(t *testing.T) {
		lock1, err := Acquire(ws)
		if err != nil {
			t.Fatalf("First Acquire: %v", err)
		}
		if err := lock1.Release(); err != nil {
			t.Fatalf("Release: %v", err)
		}

		lock2, err := Acquire(ws)
		if err != nil {
			t.Fatalf("Second Acquire after release: %v", err)
		}
		if lock2 == nil {
			t.Fatal("Expected non-nil lock on re-acquire")
		}
		_ = lock2.Release()
	})

	t.Run("release on nil receiver is safe", func(t *testing.T) {
		var lock *Lock
		if err := lock.Release(); err != nil {
			t.Errorf("Release on nil *Lock should be no-op, got %v", err)
		}
	})
}

func TestLockFileContent(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{
		Root: tmpDir,
	}

	lock, err := Acquire(ws)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	defer lock.Release()

	// Read lock file content
	data, err := os.ReadFile(lock.path)
	if err != nil {
		t.Fatalf("Failed to read lock file: %v", err)
	}

	content := string(data)
	if content == "" {
		t.Error("Lock file should contain content")
	}

	// Verify it contains PID
	if len(content) < 4 {
		t.Error("Lock file content seems too short")
	}
}

// Lock with live PID but mtime past staleLockMaxAge must be swept
// (PID-reuse defense).
func TestAcquire_AgeBasedStaleEviction(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{Root: tmpDir}
	lockPath := filepath.Join(tmpDir, lockFileName)

	// PID = current process so isProcessRunning returns true;
	// mtime backdated so the age check fires.
	content := fmt.Sprintf("pid=%d\ntimestamp=%s\n",
		os.Getpid(), time.Now().Add(-48*time.Hour).Format(time.RFC3339))
	if err := os.WriteFile(lockPath, []byte(content), 0o600); err != nil {
		t.Fatalf("plant lock: %v", err)
	}
	stale := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(lockPath, stale, stale); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	lock, err := Acquire(ws)
	if err != nil {
		t.Fatalf("Acquire must sweep an aged lock even with live PID; got %v", err)
	}
	t.Cleanup(func() { _ = lock.Release() })
}

// Freshly-acquired lock must NOT be evicted by the age check.
func TestAcquire_RecentLockHeld(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{Root: tmpDir}

	first, err := Acquire(ws)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	t.Cleanup(func() { _ = first.Release() })

	if _, err := Acquire(ws); err == nil {
		t.Fatalf("second acquire should fail while the first is held")
	}
}
