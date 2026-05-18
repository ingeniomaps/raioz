//go:build windows

package fsutil

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// FileLockExclusive acquires a cross-process advisory exclusive lock
// on f via LockFileEx + LOCKFILE_EXCLUSIVE_LOCK. The lock covers the
// entire file (offset 0, length max-uint32 hi+lo). Blocks until
// granted — matches Unix flock(LOCK_EX) semantics.
func FileLockExclusive(f *os.File) error {
	ol := new(windows.Overlapped)
	err := windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK,
		0,
		^uint32(0), ^uint32(0),
		ol,
	)
	if err != nil {
		return fmt.Errorf("LockFileEx: %w", err)
	}
	return nil
}

// FileUnlock releases the lock acquired via FileLockExclusive.
func FileUnlock(f *os.File) error {
	ol := new(windows.Overlapped)
	err := windows.UnlockFileEx(
		windows.Handle(f.Fd()),
		0,
		^uint32(0), ^uint32(0),
		ol,
	)
	if err != nil {
		return fmt.Errorf("UnlockFileEx: %w", err)
	}
	return nil
}
