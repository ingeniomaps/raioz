//go:build windows

package host

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// setNewProcessGroup configures cmd so the child gets its own console
// process group. That makes CTRL_BREAK_EVENT (the Windows analogue of
// SIGTERM for console apps) deliverable to it without also signalling
// the parent. Required for killProcessTree to be graceful — without
// the group flag, taskkill's WM_CLOSE is the only handle we have and
// console processes (ping.exe, sleep equivalents) ignore it.
func setNewProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
}

// kernel32 + GenerateConsoleCtrlEvent are pulled in via LazyDLL so we
// avoid a hard dependency on golang.org/x/sys/windows for one function.
var (
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procGenerateConsoleCtrlEvent   = kernel32.NewProc("GenerateConsoleCtrlEvent")
	ctrlBreakEvent uintptr         = 1
)

// killProcessTree sends CTRL_BREAK_EVENT to pid's process group. Console
// children created with CREATE_NEW_PROCESS_GROUP (see setNewProcessGroup)
// receive it as a graceful-shutdown signal and terminate cleanly. When
// the syscall call itself fails (Windows API returned 0), fall back to
// `taskkill /T` for any window-owning descendants the break-event
// doesn't reach.
func killProcessTree(pid int) error {
	if pid <= 0 {
		return nil
	}
	r1, _, callErr := procGenerateConsoleCtrlEvent.Call(ctrlBreakEvent, uintptr(pid))
	if r1 != 0 {
		return nil
	}
	if err := runTaskkill("/T", "/PID", strconv.Itoa(pid)); err == nil {
		return nil
	}
	return callErr
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

// killOrphansByCwd is a no-op on Windows. The Linux implementation walks
// /proc/<pid>/cwd, which doesn't exist here; replicating it via WMI or
// NtQueryInformationProcess isn't worth the complexity while raioz's
// primary host platforms remain Linux/macOS dev machines.
func killOrphansByCwd(_ string) []int {
	return nil
}
