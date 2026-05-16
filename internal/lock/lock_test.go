package lock

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"raioz/internal/i18n"
	"raioz/internal/workspace"
)

func TestMain(m *testing.M) {
	i18n.Init("en")
	os.Exit(m.Run())
}

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

// Race arm: while replaceStaleLock evicts a dead-PID lock, a second
// raioz with a LIVE PID plants its own lock in the gap. The retry must
// surface the "concurrent acquire" message naming the live PID — that's
// the actionable next step the user needs.
func TestAcquire_ReplaceStaleLock_RacerWithLivePID(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{Root: tmpDir}
	lockPath := filepath.Join(tmpDir, lockFileName)

	deadPID := pickDeadPID(t)
	plantLock(t, lockPath, deadPID, time.Now())

	livePID := os.Getpid()
	setAfterStaleRemoveHook(func() {
		plantLock(t, lockPath, livePID, time.Now())
	})
	t.Cleanup(func() { setAfterStaleRemoveHook(nil) })

	_, err := Acquire(ws)
	if err == nil {
		t.Fatal("Acquire must fail when a live racer plants in the gap")
	}
	if !strings.Contains(err.Error(), "concurrent") {
		t.Errorf("expected concurrent-acquire message, got: %v", err)
	}
}

// Race arm: while replaceStaleLock evicts a dead-PID lock, a non-raioz
// process with a DEAD PID re-grabs the slot (e.g. PID reuse). The retry
// must fall through to the generic "after cleaning stale lock" error —
// not the concurrent-acquire one, because the new PID isn't actually
// alive.
func TestAcquire_ReplaceStaleLock_RacerWithDeadPID(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{Root: tmpDir}
	lockPath := filepath.Join(tmpDir, lockFileName)

	deadPID := pickDeadPID(t)
	plantLock(t, lockPath, deadPID, time.Now())

	racerDeadPID := pickDeadPID(t)
	setAfterStaleRemoveHook(func() {
		plantLock(t, lockPath, racerDeadPID, time.Now())
	})
	t.Cleanup(func() { setAfterStaleRemoveHook(nil) })

	_, err := Acquire(ws)
	if err == nil {
		t.Fatal("Acquire must fail when a dead-PID racer plants in the gap")
	}
	if strings.Contains(err.Error(), "concurrent") {
		t.Errorf("dead-PID racer must NOT report concurrent acquire, got: %v", err)
	}
	if !strings.Contains(err.Error(), "after cleaning stale lock") {
		t.Errorf("expected generic stale-lock error, got: %v", err)
	}
}

// plantLock writes a lock file with the given PID and mtime. Helper for
// race-arm tests that need to inject lock state mid-replaceStaleLock.
func plantLock(t *testing.T, path string, pid int, mtime time.Time) {
	t.Helper()
	content := fmt.Sprintf("pid=%d\ntimestamp=%s\n",
		pid, mtime.Format(time.RFC3339))
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("plant lock: %v", err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
}

// pickDeadPID returns a PID that's guaranteed dead at the moment of the
// call. PIDs above 1<<22 don't exist on Linux; we use a high value and
// double-check with isProcessRunning.
func pickDeadPID(t *testing.T) int {
	t.Helper()
	for _, candidate := range []int{0x7ffffffe, 0x7ffffffd, 0x7ffffffc} {
		if !isProcessRunning(candidate) {
			return candidate
		}
	}
	t.Skip("could not find a guaranteed-dead PID on this OS")
	return 0
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
