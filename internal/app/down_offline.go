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

// isDockerUnreachable inspects the error message for the well-known
// Docker daemon-down signatures. Heuristic — but the alternative is
// reaching into the docker-go-client typed errors from `internal/app/`,
// which would tunnel infra concerns into the use case layer. The
// matched substrings come from the docker CLI / engine API and are
// stable across versions. Issue 071.
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

// forceOfflineCleanup runs the down-side state cleanup that does NOT
// require Docker: host processes (PIDs are local), LocalState (file
// in projectDir), and raioz.root.json (file under RaiozStateDir).
// Called from Execute when `--force-state-cleanup` is set AND the
// docker probe failed with an "unreachable" signature.
//
// Containers that may still be alive when Docker comes back are
// labelled `com.raioz.project=<name>`; the warning quotes the
// `docker ps -a --filter` command verbatim so the user has a
// one-line recovery path.
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
