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
	// LoadGlobalState loads the global state
	LoadGlobalState() (*state.GlobalState, error)
	// GetGlobalStatePath returns the path to the global state file
	GetGlobalStatePath() (string, error)
	// GetServicePreference returns the preference for a service
	GetServicePreference(ws *Workspace, serviceName string) (*state.ServicePreference, error)
	// SetServicePreference saves a service preference
	SetServicePreference(ws *Workspace, pref state.ServicePreference) error
	// GetWorkspaceProjectPreference returns the workspace project preference
	GetWorkspaceProjectPreference(workspaceName string) (*state.WorkspaceProjectPreference, error)
	// SetWorkspaceProjectPreference saves a workspace project preference
	SetWorkspaceProjectPreference(workspaceName string, pref state.WorkspaceProjectPreference) error
	// BuildServiceStates builds ServiceState list from deps and service info
	BuildServiceStates(deps *config.Deps, serviceInfos map[string]*state.ServiceInfo) []state.ServiceState
	// FormatIssues formats alignment issues for display
	FormatIssues(issues []state.AlignmentIssue) string
}
