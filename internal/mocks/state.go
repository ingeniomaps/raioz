package mocks

import (
	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/state"
	"raioz/internal/workspace"
)

// Compile-time check
var _ interfaces.StateManager = (*MockStateManager)(nil)

// MockStateManager is a mock implementation of interfaces.StateManager
type MockStateManager struct {
	LoadFunc                          func(ws *workspace.Workspace) (*config.Deps, error)
	SaveFunc                          func(ws *workspace.Workspace, deps *config.Deps) error
	ExistsFunc                        func(ws *workspace.Workspace) bool
	CompareDepsFunc                   func(oldDeps, newDeps *config.Deps) ([]state.ConfigChange, error)
	FormatChangesFunc                 func(changes []state.ConfigChange) string
	UpdateProjectStateFunc            func(projectName string, projectState *state.ProjectState) error
	RemoveProjectFunc                 func(projectName string) error
	LoadGlobalStateFunc               func() (*state.GlobalState, error)
	GetGlobalStatePathFunc            func() (string, error)
	GetServicePreferenceFunc          func(ws *workspace.Workspace, serviceName string) (*state.ServicePreference, error)
	SetServicePreferenceFunc          func(ws *workspace.Workspace, pref state.ServicePreference) error
	GetWorkspaceProjectPreferenceFunc func(workspaceName string) (*state.WorkspaceProjectPreference, error)
	SetWorkspaceProjectPreferenceFunc func(workspaceName string, pref state.WorkspaceProjectPreference) error
	BuildServiceStatesFunc            func(deps *config.Deps, serviceInfos map[string]*state.ServiceInfo) []state.ServiceState
	FormatIssuesFunc                  func(issues []state.AlignmentIssue) string
}

func (m *MockStateManager) Load(ws *workspace.Workspace) (*config.Deps, error) {
	if m.LoadFunc != nil {
		return m.LoadFunc(ws)
	}
	return nil, nil
}

func (m *MockStateManager) Save(ws *workspace.Workspace, deps *config.Deps) error {
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

func (m *MockStateManager) CompareDeps(oldDeps, newDeps *config.Deps) ([]state.ConfigChange, error) {
	if m.CompareDepsFunc != nil {
		return m.CompareDepsFunc(oldDeps, newDeps)
	}
	return nil, nil
}

func (m *MockStateManager) FormatChanges(changes []state.ConfigChange) string {
	if m.FormatChangesFunc != nil {
		return m.FormatChangesFunc(changes)
	}
	return ""
}

func (m *MockStateManager) UpdateProjectState(projectName string, projectState *state.ProjectState) error {
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

func (m *MockStateManager) LoadGlobalState() (*state.GlobalState, error) {
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

func (m *MockStateManager) GetServicePreference(ws *workspace.Workspace, serviceName string) (*state.ServicePreference, error) {
	if m.GetServicePreferenceFunc != nil {
		return m.GetServicePreferenceFunc(ws, serviceName)
	}
	return nil, nil
}

func (m *MockStateManager) SetServicePreference(ws *workspace.Workspace, pref state.ServicePreference) error {
	if m.SetServicePreferenceFunc != nil {
		return m.SetServicePreferenceFunc(ws, pref)
	}
	return nil
}

func (m *MockStateManager) GetWorkspaceProjectPreference(workspaceName string) (*state.WorkspaceProjectPreference, error) {
	if m.GetWorkspaceProjectPreferenceFunc != nil {
		return m.GetWorkspaceProjectPreferenceFunc(workspaceName)
	}
	return nil, nil
}

func (m *MockStateManager) SetWorkspaceProjectPreference(workspaceName string, pref state.WorkspaceProjectPreference) error {
	if m.SetWorkspaceProjectPreferenceFunc != nil {
		return m.SetWorkspaceProjectPreferenceFunc(workspaceName, pref)
	}
	return nil
}

func (m *MockStateManager) BuildServiceStates(deps *config.Deps, serviceInfos map[string]*state.ServiceInfo) []state.ServiceState {
	if m.BuildServiceStatesFunc != nil {
		return m.BuildServiceStatesFunc(deps, serviceInfos)
	}
	return nil
}

func (m *MockStateManager) FormatIssues(issues []state.AlignmentIssue) string {
	if m.FormatIssuesFunc != nil {
		return m.FormatIssuesFunc(issues)
	}
	return ""
}
