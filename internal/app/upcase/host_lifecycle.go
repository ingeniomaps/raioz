package upcase

import (
	"context"
	"time"

	"raioz/internal/detect"
	"raioz/internal/host"
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

	// Clear PIDs from state. Best-effort: if persist fails, next run's
	// isProcessAlive check already handles stale entries defensively.
	localState.HostPIDs = make(map[string]int)
	_ = state.SaveLocalState(projectDir, localState)
}

// saveHostPIDs persists project state to .raioz.state.json. Always writes,
// even when there are no host PIDs, so `status` / `down` can rely on the
// file for project/workspace/network provenance. Projects that only use
// Docker services need this too — otherwise down loads an empty struct and
// ends up saving garbage (`project:""`, zero time) back over the file.
func saveHostPIDs(
	projectDir, projectName, workspaceName, networkName string,
	dispatcher *orchestrate.Dispatcher,
	serviceNames []string,
	detections DetectionMap,
) {
	localState, _ := state.LoadLocalState(projectDir)
	if localState == nil {
		localState = &state.LocalState{
			HostPIDs: make(map[string]int),
		}
	}

	localState.Project = projectName
	localState.Workspace = workspaceName
	localState.NetworkName = networkName
	localState.LastUp = time.Now()

	localState.HostPIDs = make(map[string]int)
	if dispatcher != nil {
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
	}

	// Best-effort: persisting PIDs is optional — `down` can still sweep
	// by container labels when the state file is missing or partial.
	_ = state.SaveLocalState(projectDir, localState)
}

// isProcessAlive checks if a process with the given PID is running.
func isProcessAlive(pid int) bool {
	return host.IsProcessAlive(pid)
}

// killProcessGraceful sends a graceful tree kill, then force-kills if still alive.
func killProcessGraceful(pid int) {
	if pid <= 0 {
		return // negative/zero PIDs are special values in kill(2) — never use them
	}
	// Kill the whole tree so grandchildren (e.g. `go run`'s compiled
	// binary) also exit. Best-effort: the process may already be dead
	// or lack permission — the probe below covers both cases.
	_ = host.KillProcessTree(pid)

	// Brief grace period, then force-kill if still alive.
	time.Sleep(100 * time.Millisecond)
	if isProcessAlive(pid) {
		_ = host.ForceKillProcessTree(pid)
	}
}
