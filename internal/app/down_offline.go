package app

import (
	"context"
	"strings"

	"raioz/internal/domain/interfaces"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
	"raioz/internal/root"
)

// isDockerUnreachable matches well-known daemon-down signatures via
// substring. Avoids importing docker-go-client typed errors into the
// app layer; the matched strings are stable across docker versions.
func isDockerUnreachable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	signatures := []string{
		"cannot connect to the docker daemon",
		"connection refused",
		"no such host",
		"is the docker daemon running",
		"dial unix /var/run/docker.sock",
	}
	for _, sig := range signatures {
		if strings.Contains(msg, sig) {
			return true
		}
	}
	return false
}

// forceOfflineCleanup runs the cleanup steps that don't touch Docker:
// host processes, LocalState, and raioz.root.json. Surviving containers
// are surfaced by label in the warning so the user can `docker rm`
// them once the daemon is back.
func (uc *DownUseCase) forceOfflineCleanup(
	ctx context.Context, ws *interfaces.Workspace, opts DownOptions,
	projectName string, probeErr error,
) error {
	output.PrintWarning(i18n.T("warning.docker_unreachable_force_cleanup", projectName))
	logging.WarnWithContext(ctx, "Docker unreachable; proceeding with offline state cleanup",
		"project", projectName, "error", probeErr.Error())

	uc.stopHostProcesses(ctx, ws, opts)
	uc.cleanLocalState(ctx, opts)

	if ws != nil {
		if err := root.Delete(ws); err != nil {
			logging.WarnWithContext(ctx, "Failed to remove raioz.root.json",
				"project", projectName, "error", err.Error())
		}
	}

	output.PrintSuccess(i18n.T("output.offline_cleanup_done", projectName))
	return nil
}
