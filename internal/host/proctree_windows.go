//go:build windows

package host

import (
	"os/exec"
	"strconv"
	"strings"
)

func setNewProcessGroup(cmd *exec.Cmd) {
	// No-op: Windows has no Setpgid equivalent exposed via syscall.
	// taskkill /T is used later to walk the tree, which doesn't need
	// the child to live in a dedicated group.
	_ = cmd
}

func killProcessTree(pid int) error {
	if pid <= 0 {
		return nil
	}
	// /T walks the child tree; omitting /F asks for a graceful close.
	return runTaskkill("/T", "/PID", strconv.Itoa(pid))
}

func forceKillProcessTree(pid int) error {
	if pid <= 0 {
		return nil
	}
	return runTaskkill("/T", "/F", "/PID", strconv.Itoa(pid))
}

func runTaskkill(args ...string) error {
	err := exec.Command("taskkill", args...).Run()
	if err == nil {
		return nil
	}
	// Exit code 128 = "process not found" — treat as already dead so
	// callers match the Unix "ESRCH is not an error" semantics.
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 128 {
		return nil
	}
	return err
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// tasklist /FI "PID eq N" /FO CSV /NH emits a row per match or
	// "INFO: No tasks are running…" when there's nothing. The cheapest
	// check is whether the PID shows up in the output.
	out, err := exec.Command("tasklist",
		"/FI", "PID eq "+strconv.Itoa(pid),
		"/FO", "CSV", "/NH").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), strconv.Itoa(pid))
}
