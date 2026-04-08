package interfaces

import (
	models "raioz/internal/domain/models"
)

// StateManager defines operations for managing project state
type StateManager interface {
	// Load loads the project state
	Load(ws *Workspace) (*models.Deps, error)
	// Save saves the project state
	Save(ws *Workspace, deps *models.Deps) error
	// Exists checks if state file exists
	Exists(ws *Workspace) bool
	// CompareDeps compares two dependency configurations
	CompareDeps(oldDeps, newDeps *models.Deps) ([]models.ConfigChange, error)
	// FormatChanges formats configuration changes for display
	FormatChanges(changes []models.ConfigChange) string
	// UpdateProjectState updates the global project state
	UpdateProjectState(projectName string, projectState *models.ProjectState) error
	// RemoveProject removes a project from global state
	RemoveProject(projectName string) error
	// LoadGlobalState loads the global state
	LoadGlobalState() (*models.GlobalState, error)
	// GetGlobalStatePath returns the path to the global state file
	GetGlobalStatePath() (string, error)
	// GetServicePreference returns the preference for a service
	GetServicePreference(ws *Workspace, serviceName string) (*models.ServicePreference, error)
	// SetServicePreference saves a service preference
	SetServicePreference(ws *Workspace, pref models.ServicePreference) error
	// GetWorkspaceProjectPreference returns the workspace project preference
	GetWorkspaceProjectPreference(workspaceName string) (*models.WorkspaceProjectPreference, error)
	// SetWorkspaceProjectPreference saves a workspace project preference
	SetWorkspaceProjectPreference(workspaceName string, pref models.WorkspaceProjectPreference) error
	// BuildServiceStates builds ServiceState list from deps and service info
	BuildServiceStates(deps *models.Deps, serviceInfos map[string]*models.ServiceInfo) []models.ServiceState
	// FormatIssues formats alignment issues for display
	FormatIssues(issues []models.AlignmentIssue) string
}
