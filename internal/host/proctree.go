package host

import (
	"os/exec"
	"time"
)

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

// IsProcessGroupAlive reports whether any process of the group led by pid
// is still running. The leader of a host service is usually a wrapper
// (`sh -c`, a package manager) that dies at once; the server that holds
// the port is further down the group and may take seconds longer. On
// Windows there are no groups and it answers for pid alone.
func IsProcessGroupAlive(pid int) bool {
	return isProcessGroupAlive(pid)
}

// ServiceMarkerEnv is the variable raioz sets in the environment of every
// host service it starts. The whole process tree inherits it, including a
// daemon that detaches into its own session, so it tells a process raioz
// launched apart from anything else that happens to run in the same
// directory.
const ServiceMarkerEnv = "RAIOZ_HOST_SERVICE"

// ServiceMarker is the value of ServiceMarkerEnv for one service.
func ServiceMarker(project, service string) string {
	return project + "/" + service
}

// KillOrphansByCwd stops what a host service left behind after its process
// group was killed: every process that carries the service's marker in its
// environment and whose working directory is servicePath or below. It
// sends SIGTERM, waits for them to exit and forces the ones that do not.
// Returns the PIDs that were signalled.
//
// The targets are the "launcher pattern" daemons: tools that fork their
// workers into a new session (nx, vite, esbuild watchers, certain dev
// servers), out of reach of `kill -<pgid>`.
//
// Both conditions must hold. The directory alone is not enough: with
// `path: .` the service path is the project itself, where the user's
// editor, shells and unrelated tools also live. The marker alone is not
// enough either: a terminal opened from inside a service would carry it
// wherever it went.
//
// servicePath must be absolute, cleaned, and have at least 4 path
// components. Empty or non-absolute input, or an empty marker, returns
// nil without scanning. The calling process and its ancestor chain are
// never signalled.
//
// Linux: walks /proc. macOS/Windows: returns nil (no /proc).
func KillOrphansByCwd(servicePath, marker string) []int {
	if marker == "" {
		return nil
	}
	killed := killOrphansByCwd(servicePath, marker)
	waitForExit(killed)
	return killed
}

// waitForExit gives signalled processes stopShutdownDeadline to go, then
// kills the ones still there: a caller about to relaunch the service needs
// its port free.
func waitForExit(pids []int) {
	deadline := time.Now().Add(stopShutdownDeadline)
	for {
		alive := pids[:0:0]
		for _, pid := range pids {
			if IsProcessAlive(pid) {
				alive = append(alive, pid)
			}
		}
		if len(alive) == 0 {
			return
		}
		if !time.Now().Before(deadline) {
			for _, pid := range alive {
				_ = ForceKillProcessTree(pid)
				forceKillPID(pid)
			}
			return
		}
		pids = alive
		time.Sleep(50 * time.Millisecond)
	}
}
