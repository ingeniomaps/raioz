package lock

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"raioz/internal/i18n"
	"raioz/internal/workspace"
)

const lockFileName = ".raioz.lock"

// staleLockMaxAge evicts a lock whose PID number is alive but was
// reused by a non-raioz process (PID wraparound, common in containers
// with low pid_max). 24h is generous — even `raioz dashboard` watch
// mode rarely survives that long.
const staleLockMaxAge = 24 * time.Hour

type Lock struct {
	ws   *workspace.Workspace
	path string
	file *os.File
}

// isProcessRunning checks if a process with the given PID is still running
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists (doesn't actually send a signal)
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// isLockExpired returns true when the lock file is older than
// staleLockMaxAge. See the constant for the PID-reuse rationale.
func isLockExpired(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) > staleLockMaxAge
}

// readLockPID reads the PID from an existing lock file
func readLockPID(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open lock file %q: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "pid=") {
			pidStr := strings.TrimPrefix(line, "pid=")
			pid, err := strconv.Atoi(pidStr)
			if err != nil {
				return 0, fmt.Errorf("invalid PID in lock file: %w", err)
			}
			return pid, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("failed to read lock file: %w", err)
	}

	return 0, fmt.Errorf("PID not found in lock file")
}

// afterStaleRemoveHook fires inside replaceStaleLock between the Remove
// and the re-OpenFile. Tests use it to simulate a racing planter (live
// or dead PID) so the IsExist branch of replaceStaleLock can be
// exercised deterministically. Production: nil = no-op.
var afterStaleRemoveHook func()

// replaceStaleLock removes a stale lock file and re-opens it with the
// same O_CREATE|O_EXCL guarantee. If the re-open fails with IsExist a
// concurrent raioz process slipped in between Remove and OpenFile —
// report that distinctly so callers don't blame their own state. Any
// other error (e.g. unremovable file, filesystem fault) is wrapped
// verbatim so the cause is preserved in the error chain.
func replaceStaleLock(path string) (*os.File, error) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove stale lock %q: %w", path, err)
	}
	if afterStaleRemoveHook != nil {
		afterStaleRemoveHook()
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err == nil {
		return file, nil
	}
	if os.IsExist(err) {
		// Another raioz process won the race for the freshly-evicted
		// slot. Distinguishing this from "lock unremovable" gives the
		// user the right next action (wait + retry vs. inspect FS).
		newPID, pidErr := readLockPID(path)
		if pidErr == nil && isProcessRunning(newPID) {
			return nil, fmt.Errorf("%s",
				i18n.T("error.lock_concurrent_acquire", newPID))
		}
	}
	return nil, fmt.Errorf("%s",
		i18n.T("error.lock_after_stale_cleanup", err))
}

func Acquire(ws *workspace.Workspace) (*Lock, error) {
	path := filepath.Join(ws.Root, lockFileName)

	// Try to open file with exclusive lock (O_CREAT | O_EXCL)
	// Use 0600 permissions (read/write for owner only) for security
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if !os.IsExist(err) {
			return nil, fmt.Errorf("failed to acquire lock: %w", err)
		}
		// Lock file exists — decide whether to evict.
		lockPID, pidErr := readLockPID(path)
		switch {
		case pidErr != nil:
			// Failed to read PID — treat as corrupt / stale.
			file, err = replaceStaleLock(path)
			if err != nil {
				return nil, err
			}
		case !isProcessRunning(lockPID) || isLockExpired(path):
			// PID dead or lock aged out (PID-reuse defense).
			file, err = replaceStaleLock(path)
			if err != nil {
				return nil, err
			}
		default:
			// Process is still running - lock is valid
			return nil, fmt.Errorf("%s",
				i18n.T("error.lock_already_held", lockPID))
		}
	}

	// Write PID and timestamp
	pid := os.Getpid()
	timestamp := time.Now().Format(time.RFC3339)
	content := fmt.Sprintf("pid=%d\ntimestamp=%s\n", pid, timestamp)
	if _, err := file.WriteString(content); err != nil {
		file.Close()
		os.Remove(path)
		return nil, fmt.Errorf("failed to write lock file: %w", err)
	}

	lock := &Lock{
		ws:   ws,
		path: path,
		file: file,
	}

	return lock, nil
}

// Release closes and removes the lock file. Idempotent: a second call is a
// no-op so callers can release early (to free the lock during long-running
// foreground phases) and still keep a `defer Release()` as safety net.
func (l *Lock) Release() error {
	if l == nil {
		return nil
	}
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
	if l.path == "" {
		return nil
	}
	path := l.path
	l.path = ""
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("remove lock file %q: %w", path, err)
	}
	return nil
}
