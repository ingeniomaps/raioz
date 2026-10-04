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

// isProcessGroupAlive probes the group with signal 0. A pid that leads no
// group (ESRCH) falls back to the process itself.
func isProcessGroupAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if err := syscall.Kill(-pid, syscall.Signal(0)); err == nil || err == syscall.EPERM {
		return true
	}
	return isProcessAlive(pid)
}

// forceKillPID sends SIGKILL to one process, for a swept orphan that leads
// no group of its own.
func forceKillPID(pid int) {
	if pid > 0 {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

// hasServiceMarker reports whether the process was started — directly or
// through any number of forks — as the host service the marker names.
func hasServiceMarker(pid, marker string) bool {
	data, err := os.ReadFile("/proc/" + pid + "/environ")
	if err != nil {
		return false
	}
	want := ServiceMarkerEnv + "=" + marker
	for _, kv := range strings.Split(string(data), "\x00") {
		if kv == want {
			return true
		}
	}
	return false
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

// maxAncestorWalk bounds the /proc ppid walk in ancestorPIDs. Real process
// chains are a handful deep; the cap only guards against a cyclic or
// corrupted /proc read looping forever.
const maxAncestorWalk = 128

// ancestorPIDs returns the calling process plus its full parent chain up
// to init. The launcher daemons the cwd sweep exists for (nx, vite,
// esbuild) re-parent to init and are never our ancestors — but the shell
// that invoked `raioz down` from inside the project IS, and with
// `path: .` its cwd falls inside the swept path. Signalling it would kill
// the invoker (scripts, CI, non-interactive shells ignore nothing), so
// the whole chain is excluded from the kill set.
//
// Best-effort: an unreadable /proc/<pid>/stat truncates the walk, leaving
// a smaller (still safe) exclusion set. Non-Linux callers get {self}.
func ancestorPIDs() map[int]bool {
	protected := map[int]bool{os.Getpid(): true}
	pid := os.Getpid()
	for range maxAncestorWalk {
		ppid, ok := parentPID(pid)
		if !ok || ppid <= 1 {
			if ok && ppid == 1 {
				protected[1] = true
			}
			break
		}
		protected[ppid] = true
		pid = ppid
	}
	return protected
}

// parentPID reads the ppid of pid from /proc/<pid>/stat. The comm field
// (2nd) may contain spaces and parentheses, so parsing anchors on the
// LAST ')': the fields after it are state, ppid, pgrp, ...
func parentPID(pid int) (int, bool) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, false
	}
	rest := string(data)
	idx := strings.LastIndexByte(rest, ')')
	if idx < 0 || idx+2 >= len(rest) {
		return 0, false
	}
	fields := strings.Fields(rest[idx+2:])
	if len(fields) < 2 {
		return 0, false
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, false
	}
	return ppid, true
}

// killOrphansByCwd is the Linux implementation. macOS has no /proc, so the
// runtime check returns nil and the function is a no-op there.
func killOrphansByCwd(servicePath, marker string) []int {
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
	// The ancestor chain is excluded as well as self: a `raioz restart`
	// run from a terminal inside a host service carries that service's
	// marker. See docs/decisions/025-launcher-pattern-container-wait.md.
	protected := ancestorPIDs()
	var killed []int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || protected[pid] {
			continue
		}
		cwd, err := os.Readlink("/proc/" + e.Name() + "/cwd")
		if err != nil {
			continue
		}
		if cwd != clean && !strings.HasPrefix(cwd, clean+string(filepath.Separator)) {
			continue
		}
		// Only what raioz started as this service: the directory is also
		// where the user's own editor, shells and tools run.
		if !hasServiceMarker(e.Name(), marker) {
			continue
		}
		// SIGTERM here; KillOrphansByCwd waits and escalates.
		if err := syscall.Kill(pid, syscall.SIGTERM); err == nil {
			killed = append(killed, pid)
		}
	}
	return killed
}
