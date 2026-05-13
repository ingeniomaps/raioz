package mocks

import (
	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/workspace"
)

// Compile-time check
var _ interfaces.StateManager = (*MockStateManager)(nil)

// MockStateManager is a mock implementation of interfaces.StateManager
type MockStateManager struct {
	LoadFunc                          func(ws *workspace.Workspace) (*models.Deps, error)
	SaveFunc                          func(ws *workspace.Workspace, deps *models.Deps) error
	ExistsFunc                        func(ws *workspace.Workspace) bool
	CompareDepsFunc                   func(oldDeps, newDeps *models.Deps) ([]models.ConfigChange, error)
	FormatChangesFunc                 func(changes []models.ConfigChange) string
	UpdateProjectStateFunc            func(projectName string, projectState *models.ProjectState) error
	RemoveProjectFunc                 func(projectName string) error
	LoadGlobalStateFunc               func() (*models.GlobalState, error)
	GetGlobalStatePathFunc            func() (string, error)
	GetServicePreferenceFunc          func(ws *workspace.Workspace, serviceName string) (*models.ServicePreference, error)
	SetServicePreferenceFunc          func(ws *workspace.Workspace, pref models.ServicePreference) error
	GetWorkspaceProjectPreferenceFunc func(workspaceName string) (*models.WorkspaceProjectPreference, error)
	SetWorkspaceProjectPreferenceFunc func(workspaceName string, pref models.WorkspaceProjectPreference) error
	BuildServiceStatesFunc            func(
		deps *models.Deps, serviceInfos map[string]*models.ServiceInfo,
	) []models.ServiceState
	FormatIssuesFunc func(issues []models.AlignmentIssue) string
}

func (m *MockStateManager) Load(ws *workspace.Workspace) (*models.Deps, error) {
	if m.LoadFunc != nil {
		return m.LoadFunc(ws)
	}
	return nil, nil
}

func (m *MockStateManager) Save(ws *workspace.Workspace, deps *models.Deps) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(ws, deps)
	}
	return nil
}

func (m *MockStateManager) Exists(ws *workspace.Workspace) bool {
	if m.ExistsFunc != nil {
		return m.ExistsFunc(ws)
	}
	return false
}

func (m *MockStateManager) CompareDeps(oldDeps, newDeps *models.Deps) ([]models.ConfigChange, error) {
	if m.CompareDepsFunc != nil {
		return m.CompareDepsFunc(oldDeps, newDeps)
	}
	return nil, nil
}

func (m *MockStateManager) FormatChanges(changes []models.ConfigChange) string {
	if m.FormatChangesFunc != nil {
		return m.FormatChangesFunc(changes)
	}
	return ""
}

func (m *MockStateManager) UpdateProjectState(projectName string, projectState *models.ProjectState) error {
	if m.UpdateProjectStateFunc != nil {
		return m.UpdateProjectStateFunc(projectName, projectState)
	}
	return nil
}

func (m *MockStateManager) RemoveProject(projectName string) error {
	if m.RemoveProjectFunc != nil {
		return m.RemoveProjectFunc(projectName)
	}
	return nil
}

func (m *MockStateManager) LoadGlobalState() (*models.GlobalState, error) {
	if m.LoadGlobalStateFunc != nil {
		return m.LoadGlobalStateFunc()
	}
	return nil, nil
}

func (m *MockStateManager) GetGlobalStatePath() (string, error) {
	if m.GetGlobalStatePathFunc != nil {
		return m.GetGlobalStatePathFunc()
	}
	return "", nil
}

func (m *MockStateManager) GetServicePreference(
	ws *workspace.Workspace, serviceName string,
) (*models.ServicePreference, error) {
	if m.GetServicePreferenceFunc != nil {
		return m.GetServicePreferenceFunc(ws, serviceName)
	}
	return nil, nil
}

func (m *MockStateManager) SetServicePreference(ws *workspace.Workspace, pref models.ServicePreference) error {
	if m.SetServicePreferenceFunc != nil {
		return m.SetServicePreferenceFunc(ws, pref)
	}
	return nil
}

func (m *MockStateManager) GetWorkspaceProjectPreference(
	workspaceName string,
) (*models.WorkspaceProjectPreference, error) {
	if m.GetWorkspaceProjectPreferenceFunc != nil {
		return m.GetWorkspaceProjectPreferenceFunc(workspaceName)
	}
	return nil, nil
}

func (m *MockStateManager) SetWorkspaceProjectPreference(
	workspaceName string, pref models.WorkspaceProjectPreference,
) error {
	if m.SetWorkspaceProjectPreferenceFunc != nil {
		return m.SetWorkspaceProjectPreferenceFunc(workspaceName, pref)
	}
	return nil
}

func (m *MockStateManager) BuildServiceStates(
	deps *models.Deps, serviceInfos map[string]*models.ServiceInfo,
) []models.ServiceState {
	if m.BuildServiceStatesFunc != nil {
		return m.BuildServiceStatesFunc(deps, serviceInfos)
	}
	return nil
}

func (m *MockStateManager) FormatIssues(issues []models.AlignmentIssue) string {
	if m.FormatIssuesFunc != nil {
		return m.FormatIssuesFunc(issues)
	}
	return ""
}
