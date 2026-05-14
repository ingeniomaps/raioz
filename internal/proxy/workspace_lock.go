package proxy

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"raioz/internal/naming"
)

// processProxyMu serializes acquireWorkspaceLock calls within the same
// raioz process. Advisory file locks are per-process on Unix, so two
// goroutines in the same process can both grab LOCK_EX on the same
// file without blocking — this mutex closes that gap. Cross-process
// serialization (the actual concurrency story documented in ADR-010)
// comes from the OS lock acquired via lockFileExclusive.
var processProxyMu sync.Mutex

func noopRelease() {}

// acquireWorkspaceLock blocks until this process holds an exclusive
// advisory lock on the workspace's proxy state (routes dir + Caddyfile),
// then returns a release function. Per-project mode is a no-op.
// release is idempotent. See ADR-010.
func (m *Manager) acquireWorkspaceLock() (release func(), err error) {
	if !m.isWorkspaceShared() {
		return noopRelease, nil
	}

	processProxyMu.Lock()
	// Any error path from here MUST unlock the process mutex.

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
	if err := lockFileExclusive(f); err != nil {
		_ = f.Close()
		processProxyMu.Unlock()
		return nil, fmt.Errorf("acquire proxy lock %q: %w", path, err)
	}

	var released sync.Once
	return func() {
		released.Do(func() {
			_ = unlockFile(f)
			_ = f.Close()
			processProxyMu.Unlock()
		})
	}, nil
}
