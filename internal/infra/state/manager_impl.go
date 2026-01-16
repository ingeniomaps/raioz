package state

import (
	"fmt"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	statepkg "raioz/internal/state"
	workspacepkg "raioz/internal/workspace"
)

// Ensure StateManagerImpl implements interfaces.StateManager
var _ interfaces.StateManager = (*StateManagerImpl)(nil)

// StateManagerImpl is the concrete implementation of StateManager
type StateManagerImpl struct{}

// NewStateManager creates a new StateManager implementation
func NewStateManager() interfaces.StateManager {
	return &StateManagerImpl{}
}

// Load loads the project state
func (m *StateManagerImpl) Load(ws *interfaces.Workspace) (*config.Deps, error) {
	// Convert interfaces.Workspace (alias) to concrete workspace.Workspace for internal use
	wsConcrete := (*workspacepkg.Workspace)(ws)
	return statepkg.Load(wsConcrete)
}

// Save saves the project state
func (m *StateManagerImpl) Save(ws *interfaces.Workspace, deps *config.Deps) error {
	// Convert interfaces.Workspace (alias) to concrete workspace.Workspace for internal use
	wsConcrete := (*workspacepkg.Workspace)(ws)
	return statepkg.Save(wsConcrete, deps)
}

// Exists checks if state file exists
func (m *StateManagerImpl) Exists(ws *interfaces.Workspace) bool {
	// Convert interfaces.Workspace (alias) to concrete workspace.Workspace for internal use
	wsConcrete := (*workspacepkg.Workspace)(ws)
	return statepkg.Exists(wsConcrete)
}

// CompareDeps compares two dependency configurations
func (m *StateManagerImpl) CompareDeps(oldDeps, newDeps *config.Deps) ([]statepkg.ConfigChange, error) {
	return statepkg.CompareDeps(oldDeps, newDeps)
}

// FormatChanges formats configuration changes for display
func (m *StateManagerImpl) FormatChanges(changes []statepkg.ConfigChange) string {
	return statepkg.FormatChanges(changes)
}

// UpdateProjectState updates the global project state
func (m *StateManagerImpl) UpdateProjectState(projectName string, projectState *statepkg.ProjectState) error {
	if projectState == nil {
		return fmt.Errorf("projectState cannot be nil")
	}
	return statepkg.UpdateProjectState(projectName, *projectState)
}

// RemoveProject removes a project from global state
func (m *StateManagerImpl) RemoveProject(projectName string) error {
	return statepkg.RemoveProject(projectName)
}
