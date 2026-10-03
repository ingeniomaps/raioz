package app

import (
	"context"
	"fmt"
	"path/filepath"

	"raioz/internal/app/upcase"
	"raioz/internal/config"
	"raioz/internal/domain/models"
	"raioz/internal/host"
	"raioz/internal/logging"
	"raioz/internal/state"
)

// restartHostService stops and re-launches a single host service. Used by
// RestartYAML to handle every service that runs on the host — declared
// `command:` / `commands:` or an auto-detected host runtime. Workflow:
//
//  0. Resolve the command to relaunch with — the declared one, or the one
//     up inferred for an auto-detected runtime. None means nothing is
//     stopped.
//  1. Look up the running PID in .raioz.state.json. Run the user's
//     `stop:` command first when declared (typical for launchers like
//     `make dev-docker` whose grandchildren can't be killed by PID).
//  2. Otherwise, kill the process tree.
//  3. Re-launch through the orchestrator's host runner — the one `up`
//     uses, with the context `up` builds — so the settle window, the
//     launcher wait and the log file are a single implementation.
//  4. Persist the new PID back to .raioz.state.json so subsequent status
//     / down still match.
func (uc *RestartUseCase) restartHostService(
	ctx context.Context, proj *YAMLProject, name string,
) error {
	svc, ok := proj.Deps.Services[name]
	if !ok {
		return fmt.Errorf("service %q not declared in raioz.yaml", name)
	}

	// Resolve the relaunch command before touching the running process: a
	// service that cannot be relaunched must not be stopped first.
	if svc.Source.Command == "" {
		svc.Source.Command = detectedHostCommand(svc)
	}
	if svc.Source.Command == "" {
		return fmt.Errorf("no command to relaunch %q: declare `command:` in raioz.yaml", name)
	}

	projectDir, _ := filepath.Abs(filepath.Dir(proj.ConfigPath))
	localState, _ := state.LoadLocalState(projectDir)
	pid := 0
	if localState != nil {
		pid = localState.HostPIDs[name]
	}

	// The service comes back the way up starts it — same runner, same
	// context: env with PORT and every discovery var, env files, stop
	// command, proxy target. Built before the stop, while the process can
	// still tell which port it holds.
	svcCtx, ok := upcase.ServiceStartContext(ctx, uc.deps.DiscoveryManager, proj.Deps, projectDir, name)
	if !ok {
		return fmt.Errorf("cannot work out how %q starts", name)
	}

	// Stop step.
	stopCommand := ""
	if svc.Commands != nil {
		stopCommand = svc.Commands.Down
	}
	if stopCommand != "" {
		// Run via custom stop. Errors here are logged and we proceed —
		// the stop command may still have done its job (e.g. compose
		// down inside a Makefile target that surfaced a warning).
		stopDir := stopCommandDir(svc, projectDir)
		if err := uc.deps.HostRunner.StopServiceWithCommandAndPath(ctx, pid, stopCommand, stopDir); err != nil {
			logging.WarnWithContext(ctx, "Custom stop command returned error",
				"service", name, "error", err.Error())
		}
	} else if pid > 0 {
		if err := host.KillProcessTree(pid); err != nil {
			logging.WarnWithContext(ctx, "Failed to kill process tree",
				"service", name, "pid", pid, "error", err.Error())
		}
		// Same launcher-pattern sweep as down: nx/vite/etc. daemons that
		// detached via setsid escape KillProcessTree but stay rooted at
		// the service's cwd. Without this, restart leaves the old daemon
		// alive and the new launch silently reuses or competes with it.
		sweepLauncherOrphans(ctx, proj.Deps, projectDir, name)
	}

	// Start step: the runner `up` uses, so the settle window, the launcher
	// wait, the log file and the process group are one implementation.
	newPID, err := startHostServiceFn(ctx, uc.deps.DockerRunner, svcCtx)
	if err != nil {
		return fmt.Errorf("relaunch failed: %w", err)
	}

	// Persist new PID. We update only this service's entry; the rest of
	// the state file is left intact.
	if localState == nil {
		localState = &models.LocalState{
			HostPIDs: map[string]int{},
			Project:  proj.ProjectName,
		}
	}
	if localState.HostPIDs == nil {
		localState.HostPIDs = map[string]int{}
	}
	if newPID > 0 {
		localState.HostPIDs[name] = newPID
	} else {
		// Synchronous launchers (make dev-docker style) don't keep a PID.
		// Drop the entry so status doesn't pretend the old PID is alive.
		delete(localState.HostPIDs, name)
	}
	if err := state.SaveLocalState(projectDir, localState); err != nil {
		logging.WarnWithContext(ctx, "Failed to persist new PID",
			"service", name, "error", err.Error())
	}
	return nil
}

// detectedHostCommand returns the command up launched for a host service
// that declares none: the same ResolveServiceDetection up runs, preferring
// the dev command over the start one as orchestrate.HostRunner does. Empty
// when detection does not place the service on the host.
func detectedHostCommand(svc models.Service) string {
	det := config.ResolveServiceDetection(svc, svc.Source.Path)
	if !det.IsHost() {
		return ""
	}
	if det.DevCommand != "" {
		return det.DevCommand
	}
	return det.StartCommand
}
