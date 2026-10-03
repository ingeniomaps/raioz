package app

import (
	"context"
	"fmt"
	"path/filepath"

	"raioz/internal/app/upcase"
	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
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
//  3. Re-launch through HostRunner.StartService — the same path the up
//     flow uses, so the settle window and proxy.target
//     bookkeeping apply uniformly.
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

	// The service comes back on the env up gave it: without the recompute
	// it would lose PORT and every discovery var. Worked out before the
	// stop, while the process can still tell which port it holds.
	startCtx := host.WithExtraEnv(ctx, upcase.ComputedServiceEnv(
		ctx, uc.deps.DiscoveryManager, containerLookup(), proj.Deps, projectDir, name,
	))

	// Stop step.
	stopCommand := ""
	if svc.Commands != nil {
		stopCommand = svc.Commands.Down
	}
	if stopCommand != "" {
		// Run via custom stop. Errors here are logged and we proceed —
		// the stop command may still have done its job (e.g. compose
		// down inside a Makefile target that surfaced a warning).
		if err := uc.deps.HostRunner.StopServiceWithCommand(ctx, pid, stopCommand); err != nil {
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

	// Start step.
	ws := &interfaces.Workspace{Root: projectDir}
	processInfo, err := uc.deps.HostRunner.StartService(
		startCtx, ws, proj.Deps, name, svc, projectDir,
	)
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
	if processInfo != nil && processInfo.PID > 0 {
		localState.HostPIDs[name] = processInfo.PID
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
