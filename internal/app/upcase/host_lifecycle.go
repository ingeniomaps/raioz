package upcase

import (
	"context"
	"os"
	"syscall"

	"raioz/internal/detect"
	"raioz/internal/logging"
	"raioz/internal/orchestrate"
	"raioz/internal/state"
)

// cleanStaleHostProcesses kills host processes left over from a previous run.
// Reads PIDs from .raioz.state.json, checks if they're alive, and kills them.
func cleanStaleHostProcesses(ctx context.Context, projectDir, projectName string) {
	localState, err := state.LoadLocalState(projectDir)
	if err != nil || localState == nil {
		return
	}

	if len(localState.HostPIDs) == 0 {
		return
	}

	for name, pid := range localState.HostPIDs {
		if pid <= 0 {
			continue
		}
		if !isProcessAlive(pid) {
			continue
		}
		logging.InfoWithContext(ctx, "Stopping stale host process",
			"service", name, "pid", pid)
		killProcessGraceful(pid)
	}

	// Clear PIDs from state
	localState.HostPIDs = make(map[string]int)
	state.SaveLocalState(projectDir, localState)
}

// saveHostPIDs persists host service PIDs to .raioz.state.json.
func saveHostPIDs(
	projectDir, projectName string,
	dispatcher *orchestrate.Dispatcher,
	serviceNames []string,
	detections DetectionMap,
) {
	localState, _ := state.LoadLocalState(projectDir)
	if localState == nil {
		localState = &state.LocalState{
			Project:  projectName,
			HostPIDs: make(map[string]int),
		}
	}

	localState.HostPIDs = make(map[string]int)
	for _, name := range serviceNames {
		det, ok := detections[name]
		if !ok {
			continue
		}
		if det.Runtime == detect.RuntimeCompose ||
			det.Runtime == detect.RuntimeDockerfile ||
			det.Runtime == detect.RuntimeImage {
			continue // Docker-managed, not a host process
		}
		pid := dispatcher.GetHostPID(name)
		if pid > 0 {
			localState.HostPIDs[name] = pid
		}
	}

	if len(localState.HostPIDs) > 0 {
		state.SaveLocalState(projectDir, localState)
	}
}

// isProcessAlive checks if a process with the given PID is running.
func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

// killProcessGraceful sends SIGTERM, then SIGKILL after a short wait.
func killProcessGraceful(pid int) {
	// Kill the process group to catch child processes (e.g., go run spawns a child)
	pgid, err := syscall.Getpgid(pid)
	if err == nil && pgid > 0 {
		syscall.Kill(-pgid, syscall.SIGTERM)
	} else {
		syscall.Kill(pid, syscall.SIGTERM)
	}

	// Also send to PID directly as fallback
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	proc.Signal(syscall.SIGTERM)

	// Give it a moment, then force kill if still alive
	if isProcessAlive(pid) {
		if pgid > 0 {
			syscall.Kill(-pgid, syscall.SIGKILL)
		}
		proc.Kill()
	}
}
