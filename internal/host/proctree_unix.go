//go:build !windows

package host

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

func setNewProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProcessTree(pid int) error {
	if pid <= 0 {
		return nil
	}
	// Signal the group so grandchildren (e.g. `sh -c`'s real worker)
	// also exit, not just the tracked parent.
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		if err == syscall.ESRCH {
			return nil
		}
		return fmt.Errorf("SIGTERM group %d: %w", pid, err)
	}
	return nil
}

func forceKillProcessTree(pid int) error {
	if pid <= 0 {
		return nil
	}
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		if err == syscall.ESRCH {
			return nil
		}
		return fmt.Errorf("SIGKILL group %d: %w", pid, err)
	}
	return nil
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// minCwdComponents is the minimum number of path components required for
// KillOrphansByCwd to consider a target. Anything shorter (e.g. "/home/user")
// would match every shell, editor and background daemon the user has open.
const minCwdComponents = 4

// killOrphansByCwd is the Linux implementation. macOS has no /proc, so the
// runtime check returns nil and the function is a no-op there.
func killOrphansByCwd(servicePath string) []int {
	if runtime.GOOS != "linux" {
		return nil
	}
	if servicePath == "" || !filepath.IsAbs(servicePath) {
		return nil
	}
	clean := filepath.Clean(servicePath)
	if components := strings.Count(strings.Trim(clean, "/"), "/") + 1; components < minCwdComponents {
		return nil
	}

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	self := os.Getpid()
	var killed []int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue
		}
		cwd, err := os.Readlink("/proc/" + e.Name() + "/cwd")
		if err != nil {
			continue
		}
		if cwd != clean && !strings.HasPrefix(cwd, clean+string(filepath.Separator)) {
			continue
		}
		// SIGTERM only — caller decides whether to escalate. We don't
		// wait or re-probe: the parent kill already ran, and this sweep
		// is best-effort cleanup, not a barrier.
		if err := syscall.Kill(pid, syscall.SIGTERM); err == nil {
			killed = append(killed, pid)
		}
	}
	return killed
}
