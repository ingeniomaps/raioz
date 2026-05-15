package app

import (
	"context"
	"errors"

	"raioz/internal/domain/interfaces"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
	"raioz/internal/root"
)

// isDockerUnreachable reads the typed sentinel
// interfaces.ErrDaemonUnreachable that the docker adapter wraps onto
// daemon-down failures. The CLI prose → sentinel translation lives in
// internal/docker; the app layer no longer matches strings.
func isDockerUnreachable(err error) bool {
	return errors.Is(err, interfaces.ErrDaemonUnreachable)
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
