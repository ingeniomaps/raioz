package host

import "os/exec"

// SetNewProcessGroup configures cmd so the child starts in its own process
// group. On Unix this lets KillProcessTree reach every descendant via a
// signal to the negative PID. On Windows this is a no-op — taskkill /T
// already walks the process tree without needing a dedicated group.
//
// Must be called before cmd.Start. Modifying SysProcAttr after Start has
// no effect.
func SetNewProcessGroup(cmd *exec.Cmd) {
	setNewProcessGroup(cmd)
}

// KillProcessTree sends a graceful termination signal to pid and its
// descendants (SIGTERM on Unix, WM_CLOSE-equivalent via taskkill on
// Windows). The call returns without waiting for the process to actually
// exit; callers that need a barrier should poll IsProcessAlive.
//
// Returns nil when the process is already gone.
func KillProcessTree(pid int) error {
	return killProcessTree(pid)
}

// ForceKillProcessTree is the last-resort equivalent of KillProcessTree:
// SIGKILL on Unix, taskkill /F on Windows. Use only after a graceful
// KillProcessTree has failed to land within a deadline.
func ForceKillProcessTree(pid int) error {
	return forceKillProcessTree(pid)
}

// IsProcessAlive reports whether a process with the given PID is still
// running. On Unix this is a signal(0) probe. On Windows it uses the
// tasklist command — slower, but avoids pulling a system-call binding
// just to answer yes/no.
func IsProcessAlive(pid int) bool {
	return isProcessAlive(pid)
}
