package proxy

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"raioz/internal/naming"
)

// TestWorkspaceLock_NoopInPerProjectMode verifies that the lock is a true
// no-op when the manager has no workspace declared. There is no shared
// proxy state to serialize against in that mode.
func TestWorkspaceLock_NoopInPerProjectMode(t *testing.T) {
	m := NewManager("")
	m.SetProjectName("solo") // no SetWorkspace

	release, err := m.acquireWorkspaceLock()
	if err != nil {
		t.Fatalf("per-project lock must not error: %v", err)
	}
	if release == nil {
		t.Fatal("release must not be nil even for no-op")
	}
	release()
	release() // double release is safe
}

// TestWorkspaceLock_CreatesLockFile verifies that acquiring the lock
// produces an on-disk lock file in the workspace proxy dir. Subsequent
// raioz processes use the same path to coordinate, so the file's
// presence and location are part of the cross-process contract.
func TestWorkspaceLock_CreatesLockFile(t *testing.T) {
	m := makeSharedManager(t, "wsLock", "alpha")

	release, err := m.acquireWorkspaceLock()
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer release()

	want := filepath.Join(naming.WorkspaceProxyDir(), ".proxy.lock")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected lock file at %s, got %v", want, err)
	}
}

// TestWorkspaceLock_SerializesWithinProcess verifies that two goroutines
// acquiring the same workspace's lock do not overlap. Linux flock is
// per-process — the processProxyMu in the package backs this guarantee.
// Without that mutex, concurrent same-process holders would silently
// stomp on each other's Caddyfile generation.
func TestWorkspaceLock_SerializesWithinProcess(t *testing.T) {
	m := makeSharedManager(t, "wsLockSerial", "alpha")

	var active atomic.Int32
	var maxActive atomic.Int32

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			release, err := m.acquireWorkspaceLock()
			if err != nil {
				t.Errorf("acquire: %v", err)
				return
			}
			n := active.Add(1)
			for {
				cur := maxActive.Load()
				if n <= cur || maxActive.CompareAndSwap(cur, n) {
					break
				}
			}
			// Spend enough time inside the critical section that any
			// concurrency violation would be observable.
			time.Sleep(5 * time.Millisecond)
			active.Add(-1)
			release()
		})
	}
	wg.Wait()

	if maxActive.Load() != 1 {
		t.Errorf("expected at most 1 concurrent holder, observed %d",
			maxActive.Load())
	}
}

// TestWorkspaceLock_ReleaseAllowsReacquire confirms that a released lock
// can be taken again. A leaky release would manifest as a deadlock here.
func TestWorkspaceLock_ReleaseAllowsReacquire(t *testing.T) {
	m := makeSharedManager(t, "wsLockReuse", "alpha")

	for i := range 3 {
		release, err := m.acquireWorkspaceLock()
		if err != nil {
			t.Fatalf("iteration %d acquire: %v", i, err)
		}
		release()
	}
}
