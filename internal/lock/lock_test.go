package lock

import (
	"os"
	"path/filepath"
	"testing"

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
