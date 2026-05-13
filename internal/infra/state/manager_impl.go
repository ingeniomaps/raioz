package state

import (
	"fmt"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
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

// LoadGlobalState loads the global state
func (m *StateManagerImpl) LoadGlobalState() (*statepkg.GlobalState, error) {
	return statepkg.LoadGlobalState()
}

// GetGlobalStatePath returns the path to the global state file
func (m *StateManagerImpl) GetGlobalStatePath() (string, error) {
	return statepkg.GetGlobalStatePath()
}

// GetServicePreference returns the preference for a service
func (m *StateManagerImpl) GetServicePreference(
	ws *interfaces.Workspace, serviceName string,
) (*statepkg.ServicePreference, error) {
	wsConcrete := (*workspacepkg.Workspace)(ws)
	return statepkg.GetServicePreference(wsConcrete, serviceName)
}

// SetServicePreference saves a service preference
func (m *StateManagerImpl) SetServicePreference(
	ws *interfaces.Workspace, pref statepkg.ServicePreference,
) error {
	wsConcrete := (*workspacepkg.Workspace)(ws)
	return statepkg.SetServicePreference(wsConcrete, pref)
}

// GetWorkspaceProjectPreference returns the workspace project preference
func (m *StateManagerImpl) GetWorkspaceProjectPreference(
	workspaceName string,
) (*statepkg.WorkspaceProjectPreference, error) {
	return statepkg.GetWorkspaceProjectPreference(workspaceName)
}

// SetWorkspaceProjectPreference saves a workspace project preference
func (m *StateManagerImpl) SetWorkspaceProjectPreference(
	workspaceName string, pref statepkg.WorkspaceProjectPreference,
) error {
	return statepkg.SetWorkspaceProjectPreference(workspaceName, pref)
}

// BuildServiceStates builds ServiceState list from deps and service info
func (m *StateManagerImpl) BuildServiceStates(
	deps *models.Deps, serviceInfos map[string]*statepkg.ServiceInfo,
) []statepkg.ServiceState {
	return statepkg.BuildServiceStates(deps, serviceInfos)
}

// FormatIssues formats alignment issues for display
func (m *StateManagerImpl) FormatIssues(issues []statepkg.AlignmentIssue) string {
	return statepkg.FormatIssues(issues)
}
