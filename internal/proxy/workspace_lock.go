package proxy

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"raioz/internal/naming"
)

// processProxyMu serializes acquireWorkspaceLock calls within the same
// raioz process. syscall.Flock is per-process on Linux, so two goroutines
// in the same process can both grab LOCK_EX on the same file without
// blocking — this mutex closes that gap. Cross-process serialization
// (the actual concurrency story documented in ADR-010) comes from the
// flock itself.
var processProxyMu sync.Mutex

// noopRelease is the value returned by acquireWorkspaceLock in per-project
// mode (no workspace declared). The shared Caddyfile and routes dir only
// exist when workspace-shared mode is active; there is nothing to
// serialize against.
func noopRelease() {}

// acquireWorkspaceLock blocks until this process holds an exclusive
// advisory lock on the workspace's proxy state (routes dir + Caddyfile),
// then returns a release function. Per-project mode is a no-op.
//
// The lock is intentionally separate from the up-time workspace lock
// (internal/app/upcase/lock.go): the up lock guards a project's
// lifecycle and is intentionally bypassed inside sibling spawns to
// avoid A→B→A deadlocks; the proxy lock guards the *shared* Caddyfile
// and must be honored everywhere, including sibling spawns, because
// the artifact it protects is workspace-scoped, not project-scoped.
//
// release is idempotent. Callers should `defer release()` immediately
// after a successful acquire.
//
// See ADR-010.
func (m *Manager) acquireWorkspaceLock() (release func(), err error) {
	if !m.isWorkspaceShared() {
		return noopRelease, nil
	}

	processProxyMu.Lock()
	// From here on any error path MUST unlock the process mutex.

	dir := naming.WorkspaceProxyDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		processProxyMu.Unlock()
		return nil, fmt.Errorf("create workspace proxy dir: %w", err)
	}
	path := filepath.Join(dir, ".proxy.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		processProxyMu.Unlock()
		return nil, fmt.Errorf("open proxy lock %q: %w", path, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		processProxyMu.Unlock()
		return nil, fmt.Errorf("acquire proxy lock %q: %w", path, err)
	}

	var released sync.Once
	return func() {
		released.Do(func() {
			_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
			_ = f.Close()
			processProxyMu.Unlock()
		})
	}, nil
}
