//go:build !windows

package lock

import (
	"os"
	"syscall"
)

func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 doesn't send anything; it just reports whether the
	// PID is reachable (alive and same UID).
	return process.Signal(syscall.Signal(0)) == nil
}
