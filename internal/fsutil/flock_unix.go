//go:build !windows

package fsutil

import (
	"fmt"
	"os"
	"syscall"
)

// FileLockExclusive acquires an advisory exclusive lock on f
// (flock(LOCK_EX) on Unix, LockFileEx with LOCKFILE_EXCLUSIVE_LOCK on
// Windows). Blocks until the lock is granted. Caller must hold f open
// while the lock is held; closing f implicitly releases.
func FileLockExclusive(f *os.File) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("flock LOCK_EX: %w", err)
	}
	return nil
}

// FileUnlock releases the advisory lock acquired via FileLockExclusive.
func FileUnlock(f *os.File) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
		return fmt.Errorf("flock LOCK_UN: %w", err)
	}
	return nil
}
