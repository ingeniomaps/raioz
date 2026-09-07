package upcase

import (
	"context"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
)

// WorkspaceProjectConflictResult is the result of resolving a workspace-vs-project conflict
type WorkspaceProjectConflictResult int

const (
	WorkspaceConflictProceed WorkspaceProjectConflictResult = iota
	WorkspaceConflictSkip
	WorkspaceConflictCancel
)

// checkWorkspaceProjectConflict detects when the same workspace is already running
// from a different project. Returns (result, mergedDeps, error).
// When result is Proceed and mergedDeps is non-nil, the caller must use mergedDeps (merged configs).
// When result is Proceed and mergedDeps is nil, the caller uses current deps (replace).
// currentProjectDir is the absolute path to the current project (where .raioz.json is);
// used to resolve relative volumes per project when merging.
func (uc *UseCase) checkWorkspaceProjectConflict(
	ctx context.Context,
	deps *models.Deps,
	ws *interfaces.Workspace,
	currentProjectDir string,
) (WorkspaceProjectConflictResult, *models.Deps, error) {
	// ADR-011 Phase 3: the conflict prompt used to compare the current
	// project's services/infra against the previous project's
	// deserialized from .state.json. With the legacy snapshot gone there
	// is no way to materialize the "other project's" config without
	// knowing where its raioz.yaml lives — a piece of data the
	// snapshot uniquely provided.
	//
	// The feature is dropped rather than partially reconstructed:
	// reconstructing it from Docker labels alone would let us name
	// "project P is also up here" but not diff its services, so the
	// merge prompt would be unactionable. Users who hit a multi-project
	// workspace collision can resolve it by running `raioz down
	// --conflicting` (which already sweeps cross-project containers via
	// labels) before re-running `raioz up`. Documented in ADR-011.
	_ = ctx
	_ = deps
	_ = ws
	_ = currentProjectDir
	return WorkspaceConflictProceed, nil, nil
}
