package interfaces

import (
	"raioz/internal/config"
	"raioz/internal/state"
)

// StateManager defines operations for managing project state
type StateManager interface {
	// Load loads the project state
	Load(ws *Workspace) (*config.Deps, error)
	// Save saves the project state
	Save(ws *Workspace, deps *config.Deps) error
	// Exists checks if state file exists
	Exists(ws *Workspace) bool
	// CompareDeps compares two dependency configurations
	CompareDeps(oldDeps, newDeps *config.Deps) ([]state.ConfigChange, error)
	// FormatChanges formats configuration changes for display
	FormatChanges(changes []state.ConfigChange) string
	// UpdateProjectState updates the global project state
	UpdateProjectState(projectName string, projectState *state.ProjectState) error
	// RemoveProject removes a project from global state
	RemoveProject(projectName string) error
}
