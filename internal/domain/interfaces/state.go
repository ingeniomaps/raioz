package interfaces

import (
	"raioz/internal/domain/models"
)

// StateManager defines operations for managing project state.
//
// ADR-011 Phase 3 removed the legacy whole-Deps snapshot. The methods
// that used to expose it — Load, Save, Exists, CompareDeps, and
// FormatChanges — are deleted. Liveness is probed via
// DockerRunner.IsProjectActive; runtime overrides live in LocalState
// (state.LoadLocalState / state.SaveLocalState). The remaining methods
// here cover the global state file (cross-project, persisted at the
// raioz home) and per-workspace preferences, which Phase 3 leaves
// alone.
type StateManager interface {
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
