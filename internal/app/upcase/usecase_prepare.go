package upcase

import (
	"context"
	"os"
	"path/filepath"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/logging"
)

// resolveProjectContext resolves the absolute project dir, sweeps the
// pre-ADR-011 .state.json, and resolves the project env path. First
// phase of the Execute decomposition.
func (uc *UseCase) resolveProjectContext(
	ctx context.Context, opts Options, deps *models.Deps, ws *interfaces.Workspace,
) (projectDir, projectEnvPath string, err error) {
	projectDir, err = filepath.Abs(filepath.Dir(opts.ConfigPath))
	if err != nil {
		return "", "", errors.New(
			errors.ErrCodeWorkspaceError,
			i18n.T("error.project_dir"),
		).WithError(err)
	}

	uc.sweepLegacyStateSnapshot(ctx, ws)

	projectEnvPath, err = uc.deps.EnvManager.ResolveProjectEnv(ws, deps, projectDir)
	if err != nil {
		return "", "", errors.New(
			errors.ErrCodeWorkspaceError,
			i18n.T("error.project_env_resolve"),
		).WithSuggestion(
			i18n.T("error.project_env_resolve_suggestion"),
		).WithError(err)
	}
	return projectDir, projectEnvPath, nil
}

// sweepLegacyStateSnapshot best-effort removes pre-ADR-011
// `.state.json` snapshots. Missing file is the common case (silent);
// a permissions error is logged but not fatal.
func (uc *UseCase) sweepLegacyStateSnapshot(ctx context.Context, ws *interfaces.Workspace) {
	legacyStatePath := filepath.Join(ws.Root, ".state.json")
	if err := os.Remove(legacyStatePath); err == nil {
		logging.InfoWithContext(ctx,
			"removed legacy .state.json snapshot (ADR-011)",
			"path", legacyStatePath)
	} else if !os.IsNotExist(err) {
		logging.WarnWithContext(ctx,
			"failed to remove legacy .state.json",
			"path", legacyStatePath, "error", err.Error())
	}
}
