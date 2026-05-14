package upcase

import (
	"context"
	"time"

	"raioz/internal/domain/models"
	"raioz/internal/host"
	"raioz/internal/logging"
	"raioz/internal/orchestrate"
	"raioz/internal/state"
)

// cleanStaleHostProcesses kills host processes left over from a previous
// run. Reads PIDs from .raioz.state.json, checks if they're alive, and
// kills them.
//
// `inScope` restricts the sweep to services the current `up` actually
// touches. Pass deps.Services keys: for a full up that's every service;
// for a selective `up --only api` (or `up api`) it's just the chosen
// subset, so a healthy `web` running from a previous up survives the
// invocation. nil/empty scope is a safety check that disables the sweep
// rather than silently nuking everything.
func cleanStaleHostProcesses(
	ctx context.Context,
	projectDir, projectName string,
	inScope map[string]struct{},
) {
	if len(inScope) == 0 {
		return
	}
	localState, err := state.LoadLocalState(projectDir)
	if err != nil || localState == nil {
		return
	}

	if len(localState.HostPIDs) == 0 {
		return
	}

	for name, pid := range localState.HostPIDs {
		if _, ok := inScope[name]; !ok {
			continue
		}
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

	// Clear only the in-scope PIDs from state. Out-of-scope entries
	// (services this up isn't touching) are left intact so a parallel
	// `up --only` doesn't strand them. Best-effort: if persist fails,
	// next run's isProcessAlive check handles stale entries defensively.
	for name := range inScope {
		delete(localState.HostPIDs, name)
	}
	_ = state.SaveLocalState(projectDir, localState)
}

// saveHostPIDs persists project state to .raioz.state.json. Always writes,
// even when there are no host PIDs, so `status` / `down` can rely on the
// file for project/workspace/network provenance. Projects that only use
// Docker services need this too — otherwise down loads an empty struct and
// ends up saving garbage (`project:""`, zero time) back over the file.
//
// `deferredDeps` is the list of dep names whose dispatch was skipped at
// up time because a sibling project owns them (issue #26 mode B). Pass
// nil for projects without sibling deps; the slice overwrites
// LocalState.DeferredToSibling so stale entries from previous ups are
// dropped without an explicit ClearDeferred per dep.
func saveHostPIDs(
	projectDir, projectName, workspaceName, networkName string,
	dispatcher *orchestrate.Dispatcher,
	serviceNames []string,
	detections DetectionMap,
	deferredDeps []string,
) {
	localState, _ := state.LoadLocalState(projectDir)
	if localState == nil {
		localState = &models.LocalState{
			HostPIDs: make(map[string]int),
		}
	}

	localState.Project = projectName
	localState.Workspace = workspaceName
	localState.NetworkName = networkName
	localState.LastUp = time.Now()

	localState.DeferredToSibling = deferredDeps

	// Merge in-scope host PIDs into the existing map instead of wiping.
	// Selective ups (`raioz up api`) only carry the chosen services in
	// serviceNames; wiping would orphan PIDs of services brought up by a
	// prior full `up` and leave a subsequent full `down` with no way to
	// kill them. cleanStaleHostProcesses already removed in-scope entries
	// before this point, so we're filling them back in with fresh PIDs.
	if localState.HostPIDs == nil {
		localState.HostPIDs = make(map[string]int)
	}
	if dispatcher != nil {
		for _, name := range serviceNames {
			det, ok := detections[name]
			if !ok {
				continue
			}
			if det.Runtime == models.RuntimeCompose ||
				det.Runtime == models.RuntimeDockerfile ||
				det.Runtime == models.RuntimeImage {
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
