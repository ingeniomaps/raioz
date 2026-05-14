//go:build !windows

package proxy

import (
	"fmt"
	"os"
	"syscall"
)

// flock(LOCK_EX) — advisory per-process exclusive lock. Blocks
// until the lock is granted.
func lockFileExclusive(f *os.File) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("flock LOCK_EX: %w", err)
	}
	return nil
}

func unlockFile(f *os.File) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
		return fmt.Errorf("flock LOCK_UN: %w", err)
	}
	return nil
}
