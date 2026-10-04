package upcase

import (
	"context"
	"path/filepath"
	"time"

	"raioz/internal/domain/models"
	"raioz/internal/host"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
	"raioz/internal/state"
)

// runningHostServices reports which in-scope host services an earlier up
// left running, by recorded PID, and drops the recorded PIDs that no longer
// stand for a service. Nothing is killed: up only starts what is not
// running, and `raioz restart` is how a running service is relaunched.
//
// A recorded PID counts as the service when the process is alive and,
// where it can be told, still runs in the service's path — a PID that the
// OS has since handed to an unrelated process is forgotten, not adopted.
//
// `inScope` is the service-name subset the current `up` touches;
// nil/empty returns nothing and leaves the state alone, so a selective up
// never disturbs the entries of services it is not starting.
func runningHostServices(
	ctx context.Context,
	projectDir string,
	deps *models.Deps,
	inScope map[string]struct{},
) map[string]int {
	if len(inScope) == 0 {
		return nil
	}
	localState, err := state.LoadLocalState(projectDir)
	if err != nil || localState == nil || len(localState.HostPIDs) == 0 {
		return nil
	}

	running := map[string]int{}
	dropped := false
	for name, pid := range localState.HostPIDs {
		if _, ok := inScope[name]; !ok {
			continue
		}
		if pid > 0 && isProcessAlive(pid) && pidIsService(pid, deps, projectDir, name) {
			running[name] = pid
			continue
		}
		logging.InfoWithContext(ctx, "Forgetting recorded host PID: not the service anymore",
			"service", name, "pid", pid)
		delete(localState.HostPIDs, name)
		dropped = true
	}
	if dropped {
		// Best-effort: a stale entry that survives is caught again by the
		// same checks on the next run.
		_ = state.SaveLocalState(projectDir, localState)
	}
	return running
}

// pidIsService reports whether pid still belongs to the named service: its
// working directory is the service's path. Where that cannot be read, a
// live recorded PID is taken at its word.
func pidIsService(pid int, deps *models.Deps, projectDir, name string) bool {
	svc, ok := deps.Services[name]
	if !ok {
		return false
	}
	path := svc.Source.Path
	switch {
	case path == "" || path == ".":
		path = projectDir
	case !filepath.IsAbs(path):
		path = filepath.Join(projectDir, path)
	}
	within, known := host.ProcessRunsIn(pid, path)
	return !known || within
}

// saveHostPIDs persists project state to .raioz.state.json. Always writes,
// even when there are no host PIDs, so `status` / `down` can rely on the
// file for project/workspace/network provenance. Projects that only use
// Docker services need this too — otherwise down loads an empty struct and
// ends up saving garbage (`project:""`, zero time) back over the file.
//
// `deferredDeps` is the list of dep names whose dispatch was skipped at
// up time because a sibling project owns them (ADR-008 mode B). Pass
// nil for projects without sibling deps; the slice overwrites
// LocalState.DeferredToSibling so stale entries from previous ups are
// dropped without an explicit ClearDeferred per dep.
func saveHostPIDs(
	projectDir, projectName, workspaceName, networkName string,
	dispatcher serviceDispatcher,
	serviceNames []string,
	detections DetectionMap,
	deferredDeps []string,
) {
	persistHostPIDs(projectDir, projectName, workspaceName, networkName,
		dispatcher, serviceNames, detections, deferredDeps, true)
}

// savePartialHostPIDs persists what a FAILED up started before it bailed.
// `down` and `status` only know the PIDs recorded here, so an unrecorded one
// is a live process nobody can stop or even report.
//
// LastUp stays untouched on purpose: the up did not complete.
func savePartialHostPIDs(
	projectDir, projectName, workspaceName, networkName string,
	dispatcher serviceDispatcher,
	startedNames []string,
	detections DetectionMap,
	deferredDeps []string,
) {
	if len(startedNames) == 0 {
		return
	}
	persistHostPIDs(projectDir, projectName, workspaceName, networkName,
		dispatcher, startedNames, detections, deferredDeps, false)
}

func persistHostPIDs(
	projectDir, projectName, workspaceName, networkName string,
	dispatcher serviceDispatcher,
	serviceNames []string,
	detections DetectionMap,
	deferredDeps []string,
	bumpLastUp bool,
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
	if bumpLastUp {
		localState.LastUp = time.Now()
	}

	localState.DeferredToSibling = deferredDeps

	// Merge in-scope host PIDs into the existing map instead of wiping.
	// Selective ups (`raioz up api`) only carry the chosen services in
	// serviceNames; wiping would orphan PIDs of services brought up by a
	// prior full `up` and leave a subsequent full `down` with no way to
	// kill them. In-scope entries are overwritten below with the PIDs this
	// run started or adopted.
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

// stopHostServiceFn stops one host service by PID. A package var so tests
// can observe the call without a real process.
var stopHostServiceFn = host.StopServiceWithCommandAndPath

// stopGitServicesBeforeReclone stops every running host service whose
// directory `--force-reclone` is about to delete. A process left alive
// keeps its port with a working directory that no longer exists: the same
// run then fails against it, raioz stops recognizing the PID as the
// service, and no later `down` can reach it.
func stopGitServicesBeforeReclone(ctx context.Context, deps *models.Deps, projectDir string) {
	localState, err := state.LoadLocalState(projectDir)
	if err != nil || localState == nil || len(localState.HostPIDs) == 0 {
		return
	}
	stopped := false
	for name, svc := range deps.Services {
		pid, tracked := localState.HostPIDs[name]
		if svc.Source.Kind != "git" || !tracked || pid <= 0 {
			continue
		}
		if !isProcessAlive(pid) || !pidIsService(pid, deps, projectDir, name) {
			continue
		}
		output.PrintInfo(i18n.T("up.git.stopping_for_reclone", name))
		var stopCommand string
		if svc.Commands != nil {
			stopCommand = svc.Commands.Down
		}
		if err := stopHostServiceFn(ctx, pid, stopCommand, svc.Source.Path); err != nil {
			logging.WarnWithContext(ctx, "Could not stop service before re-clone",
				"service", name, "pid", pid, "error", err.Error())
			continue
		}
		delete(localState.HostPIDs, name)
		stopped = true
	}
	if stopped {
		_ = state.SaveLocalState(projectDir, localState)
	}
}

// devOverrideRunning reports whether a dependency promoted with `raioz dev`
// to a host-run path still has its recorded process alive in that path.
func devOverrideRunning(projectDir, name, localPath string) bool {
	localState, err := state.LoadLocalState(projectDir)
	if err != nil || localState == nil {
		return false
	}
	pid := localState.HostPIDs[name]
	if pid <= 0 || !isProcessAlive(pid) {
		return false
	}
	within, known := host.ProcessRunsIn(pid, localPath)
	return !known || within
}
