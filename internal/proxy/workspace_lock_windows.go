//go:build windows

package proxy

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// LockFileEx with LOCKFILE_EXCLUSIVE_LOCK is the Windows analogue of
// flock(LOCK_EX): cross-process advisory lock on the file handle.
// The lock covers the entire file (offset 0, length max-uint32 hi+lo).
// Without LOCKFILE_FAIL_IMMEDIATELY the call blocks until granted,
// matching the Unix Flock(LOCK_EX) semantics.
func lockFileExclusive(f *os.File) error {
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

func unlockFile(f *os.File) error {
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
