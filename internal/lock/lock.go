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

	"raioz/internal/workspace"
)

const lockFileName = ".raioz.lock"

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
	if err != nil {
		return false
	}
	return true
}

// readLockPID reads the PID from an existing lock file
func readLockPID(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
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

func Acquire(ws *workspace.Workspace) (*Lock, error) {
	path := filepath.Join(ws.Root, lockFileName)

	// Try to open file with exclusive lock (O_CREAT | O_EXCL)
	// Use 0600 permissions (read/write for owner only) for security
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if os.IsExist(err) {
			// Lock file exists - check if the process is still running
			lockPID, pidErr := readLockPID(path)
			if pidErr != nil {
				// Failed to read PID - remove stale lock and try again
				os.Remove(path)
				file, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if err != nil {
					return nil, fmt.Errorf("failed to acquire lock after cleaning stale lock: %w", err)
				}
			} else if !isProcessRunning(lockPID) {
				// Process is not running - remove stale lock and try again
				os.Remove(path)
				file, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if err != nil {
					return nil, fmt.Errorf("failed to acquire lock after cleaning stale lock: %w", err)
				}
			} else {
				// Process is still running - lock is valid
				return nil, fmt.Errorf("lock already exists: another raioz process may be running")
			}
		} else {
			return nil, fmt.Errorf("failed to acquire lock: %w", err)
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

func (l *Lock) Release() error {
	if l.file != nil {
		l.file.Close()
	}
	if l.path != "" {
		return os.Remove(l.path)
	}
	return nil
}
