package upcase

import (
	"fmt"

	"raioz/internal/state"
)

// persistProjectPathsToLocalState updates the project's LocalState file
// with the docker-compose path and project identity that `raioz up` just
// established. Idempotent: re-running up with the same paths is a no-op
// from LocalState's perspective.
//
// This is the migration vehicle for ADR-011 Phase 2: information the
// legacy whole-Deps snapshot used to expose (ProjectComposePath,
// ProjectRoot) moves into LocalState so inspection commands can read it
// without reviving the snapshot.
func persistProjectPathsToLocalState(
	projectDir, projectName, workspaceName, composePath string,
) error {
	ls, err := state.LoadLocalState(projectDir)
	if err != nil {
		return fmt.Errorf("load LocalState: %w", err)
	}
	if ls.Project == "" {
		ls.Project = projectName
	}
	if ls.Workspace == "" && workspaceName != "" && workspaceName != projectName {
		ls.Workspace = workspaceName
	}
	ls.ProjectRoot = projectDir
	ls.ProjectComposePath = composePath

	if err := state.SaveLocalState(projectDir, ls); err != nil {
		return fmt.Errorf("save LocalState: %w", err)
	}
	return nil
}
