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

// resolveProjectContext computes the absolute project directory,
// sweeps any legacy ADR-011 state snapshot, and resolves the
// project-level env path. Extracted from Execute so the early
// "wire up paths" phase has a single home and Execute stays under
// the 400-line cap. Issue 079.
//
// Returns (projectDir, projectEnvPath, error). All callers in
// Execute should use the returned projectDir / projectEnvPath as
// the canonical values for the rest of the run.
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

// sweepLegacyStateSnapshot best-effort removes the legacy
// `.state.json` left behind by pre-ADR-011 binaries. New world
// writes only LocalState in the project dir and reads from Docker
// + raioz.yaml. A non-existent file is the common case (no log);
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
