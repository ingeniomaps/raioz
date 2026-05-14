package app

import (
	"path/filepath"

	"raioz/internal/state"
)

// loadProjectComposePathFromLocalState returns the docker-compose path
// that `raioz up` persisted for the project at configPath, or "" if no
// path was recorded (the project was never up here, or up didn't detect
// a compose file). Errors loading LocalState are silently swallowed —
// callers fall back to the workspace-generated compose path, which is
// always present.
//
// Introduced by ADR-011 Phase 2 so inspection commands
// (`logs`, `exec`, `restart`, `status`) can find the user's compose
// file without consulting the deprecated whole-Deps snapshot.
func loadProjectComposePathFromLocalState(configPath string) string {
	if configPath == "" {
		return ""
	}
	projectDir, err := filepath.Abs(filepath.Dir(configPath))
	if err != nil {
		return ""
	}
	ls, err := state.LoadLocalState(projectDir)
	if err != nil || ls == nil {
		return ""
	}
	return ls.ProjectComposePath
}
