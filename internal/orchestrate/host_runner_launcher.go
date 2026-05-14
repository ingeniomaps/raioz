package orchestrate

import (
	"context"
	"fmt"

	"raioz/internal/docker"
	"raioz/internal/domain/interfaces"
	"raioz/internal/host"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// waitForLauncherContainer polls Docker for the launcher's declared
// container until it appears or the configured timeout elapses.
// Issue 047 / ADR-025: prevents `raioz up` from reporting "ready"
// while `docker compose up -d --build` is still building the
// image. A user-visible message tells the dev raioz is waiting on
// purpose. On timeout we warn and continue — the run is not aborted
// because the build may legitimately take longer on a slow box.
//
// No-op when:
//   - svc.ProxyTarget is empty (user didn't declare proxy.target:)
//   - the target is host-shaped (no container to wait for)
//   - the configured timeout is zero (explicit opt-out via env)
func waitForLauncherContainer(ctx context.Context, svc interfaces.ServiceContext) {
	if docker.IsHostGatewayTarget(svc.ProxyTarget) {
		return
	}
	timeout := host.LauncherWaitTimeout()
	if timeout <= 0 {
		return
	}

	output.PrintInfo(fmt.Sprintf(
		"Waiting for launcher container '%s' to appear (up to %v)...",
		svc.ProxyTarget, timeout,
	))
	if err := docker.WaitForContainer(ctx, svc.ProxyTarget, timeout); err != nil {
		output.PrintWarning(fmt.Sprintf(
			"Service '%s': launcher exited but container '%s' did not "+
				"appear within %v — continuing. Re-run `raioz status` "+
				"shortly, or check `docker ps -a | grep %s`.",
			svc.Name, svc.ProxyTarget, timeout, svc.ProxyTarget))
		logging.WarnWithContext(ctx, "Launcher container did not appear",
			"service", svc.Name, "target", svc.ProxyTarget,
			"timeout", timeout.String(), "error", err.Error())
		return
	}
	output.PrintSuccess(fmt.Sprintf(
		"Launcher container '%s' ready", svc.ProxyTarget))
}

// drainLauncherBeforeStop waits for an in-progress launcher build to
// finish before `raioz down` runs the user's `stop:`. Without this,
// down → stop completes while `docker compose up -d --build` is
// still building, and when the build eventually finishes the
// container is left orphaned. Issue 047 option B / ADR-025.
//
// We probe via docker.WaitForContainer up to LauncherDrainTimeout.
// If the container already exists we return immediately — the
// stop: command can do its job. If it never appears (legitimate
// case: the launcher actually crashed silently), we warn and let
// stop: run anyway; the user will see whatever errors emerge.
func drainLauncherBeforeStop(ctx context.Context, svc interfaces.ServiceContext) {
	if docker.IsHostGatewayTarget(svc.ProxyTarget) {
		return
	}
	timeout := host.LauncherDrainTimeout()
	if timeout <= 0 {
		return
	}

	// Cheap probe: if the container already exists, no drain needed.
	if status, _ := docker.GetContainerStatusByName(ctx, svc.ProxyTarget); status != "" {
		return
	}

	output.PrintInfo(fmt.Sprintf(
		"Waiting up to %v for launcher build of '%s' to finish "+
			"before running stop: ...", timeout, svc.ProxyTarget,
	))
	if err := docker.WaitForContainer(ctx, svc.ProxyTarget, timeout); err != nil {
		output.PrintWarning(fmt.Sprintf(
			"Service '%s': launcher build did not produce container '%s' "+
				"within %v — running stop: anyway. Check `docker ps -a` "+
				"if an orphan appears.",
			svc.Name, svc.ProxyTarget, timeout))
		logging.WarnWithContext(ctx, "Launcher drain timed out before stop",
			"service", svc.Name, "target", svc.ProxyTarget,
			"timeout", timeout.String(), "error", err.Error())
	}
}
