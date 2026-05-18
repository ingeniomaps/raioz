package upcase

import (
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/fsutil"
	"raioz/internal/naming"
)

// acquirePortsLock takes a global advisory lock on
// `naming.PortsLockFile()` so concurrent `raioz up` invocations don't
// race on host-port allocation. Returns a release func the caller must
// defer; release is safe to call when the lock wasn't taken (it
// closes/unlocks nil-safely).
//
// Failure modes are non-fatal: if the lock dir is unwritable we log
// nothing and return a no-op release so `raioz up` proceeds — port
// collisions then degrade to the pre-fix behaviour (Docker errors out
// noisily). Better to let the user proceed than to block `up` because
// of a state-dir permission issue.
func acquirePortsLock() (release func(), err error) {
	noop := func() {}
	path := naming.PortsLockFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return noop, nil // degrade silently — see doc comment
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return noop, nil
	}
	if err := fsutil.FileLockExclusive(f); err != nil {
		_ = f.Close()
		return noop, fmt.Errorf("acquire ports lock %s: %w", path, err)
	}
	return func() {
		_ = fsutil.FileUnlock(f)
		_ = f.Close()
	}, nil
}
