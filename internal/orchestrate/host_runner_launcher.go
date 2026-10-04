package orchestrate

import (
	"context"

	"raioz/internal/docker"
	"raioz/internal/domain/interfaces"
	"raioz/internal/host"
	"raioz/internal/i18n"
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

	output.PrintInfo(i18n.T("launcher.waiting", svc.ProxyTarget, timeout))
	if err := docker.WaitForContainer(ctx, svc.ProxyTarget, timeout); err != nil {
		output.PrintWarning(i18n.T("launcher.not_appeared",
			svc.Name, svc.ProxyTarget, timeout, svc.ProxyTarget))
		logging.WarnWithContext(ctx, "Launcher container did not appear",
			"service", svc.Name, "target", svc.ProxyTarget,
			"timeout", timeout.String(), "error", err.Error())
		return
	}
	output.PrintSuccess(i18n.T("launcher.ready", svc.ProxyTarget))
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

	output.PrintInfo(i18n.T("launcher.drain_waiting", timeout, svc.ProxyTarget))
	if err := docker.WaitForContainer(ctx, svc.ProxyTarget, timeout); err != nil {
		output.PrintWarning(i18n.T("launcher.drain_timeout",
			svc.Name, svc.ProxyTarget, timeout))
		logging.WarnWithContext(ctx, "Launcher drain timed out before stop",
			"service", svc.Name, "target", svc.ProxyTarget,
			"timeout", timeout.String(), "error", err.Error())
	}
}
