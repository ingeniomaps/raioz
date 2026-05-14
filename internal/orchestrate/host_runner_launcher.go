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

// Polls for svc.ProxyTarget after the launcher exits; ADR-025. Timeout
// is a warning, never an abort — a slow box should still make progress.
// No-op when the target is empty/host-shaped or the timeout is zero.
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

// Waits for an in-progress launcher build to produce the container
// before stop: runs. Without this, a stop that wins the race leaves
// an orphan when the build finishes. ADR-025.
func drainLauncherBeforeStop(ctx context.Context, svc interfaces.ServiceContext) {
	if docker.IsHostGatewayTarget(svc.ProxyTarget) {
		return
	}
	timeout := host.LauncherDrainTimeout()
	if timeout <= 0 {
		return
	}

	// Already up → nothing to drain.
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
