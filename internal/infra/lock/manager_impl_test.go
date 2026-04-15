package lock

import (
	"testing"

	"raioz/internal/domain/interfaces"
)

func TestNewLockManager(t *testing.T) {
	m := NewLockManager()
	if m == nil {
		t.Fatal("NewLockManager returned nil")
	}
}

func TestLockManagerImpl_AcquireAndRelease(t *testing.T) {
	m := NewLockManager()
	dir := t.TempDir()

	ws := &interfaces.Workspace{
		Root: dir,
	}

	lock, err := m.Acquire(ws)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if lock == nil {
		t.Fatal("Acquire returned nil lock")
	}

	if err := lock.Release(); err != nil {
		t.Errorf("Release failed: %v", err)
	}
}

func TestLockManagerImpl_Acquire_DoubleLockSameProcess(t *testing.T) {
	m := NewLockManager()
	dir := t.TempDir()

	ws := &interfaces.Workspace{Root: dir}

	lock1, err := m.Acquire(ws)
	if err != nil {
		t.Fatalf("first Acquire failed: %v", err)
	}
	defer lock1.Release()

	// Second acquire — behavior depends on the underlying lock implementation.
	// Some implementations allow it (reentrant), some reject. We just verify no panic.
	lock2, err := m.Acquire(ws)
	if err == nil && lock2 != nil {
		lock2.Release()
	}
}
