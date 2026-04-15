package mocks

import (
	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/workspace"
)

// Compile-time check
var _ interfaces.WorkspaceManager = (*MockWorkspaceManager)(nil)

// MockWorkspaceManager is a mock implementation of interfaces.WorkspaceManager
type MockWorkspaceManager struct {
	ResolveFunc                 func(projectName string) (*workspace.Workspace, error)
	GetBaseDirFunc              func() (string, error)
	GetBaseDirFromWorkspaceFunc func(ws *workspace.Workspace) string
	GetComposePathFunc          func(ws *workspace.Workspace) string
	GetStatePathFunc            func(ws *workspace.Workspace) string
	GetActiveWorkspaceFunc      func() (string, error)
	GetRootFunc                 func(ws *workspace.Workspace) string
	GetServicePathFunc          func(ws *workspace.Workspace, serviceName string, svc config.Service) string
	GetServiceDirFunc           func(ws *workspace.Workspace, svc config.Service) string
	MigrateLegacyServicesFunc   func(ws *workspace.Workspace, deps *config.Deps) error
	ListWorkspacesFunc          func() ([]string, error)
	WorkspaceExistsFunc         func(workspaceName string) (bool, error)
	SetActiveWorkspaceFunc      func(workspaceName string) error
	DeleteWorkspaceFunc         func(workspaceName string) error
}

func (m *MockWorkspaceManager) Resolve(projectName string) (*workspace.Workspace, error) {
	if m.ResolveFunc != nil {
		return m.ResolveFunc(projectName)
	}
	return nil, nil
}

func (m *MockWorkspaceManager) GetBaseDir() (string, error) {
	if m.GetBaseDirFunc != nil {
		return m.GetBaseDirFunc()
	}
	return "", nil
}

func (m *MockWorkspaceManager) GetBaseDirFromWorkspace(ws *workspace.Workspace) string {
	if m.GetBaseDirFromWorkspaceFunc != nil {
		return m.GetBaseDirFromWorkspaceFunc(ws)
	}
	return ""
}

func (m *MockWorkspaceManager) GetComposePath(ws *workspace.Workspace) string {
	if m.GetComposePathFunc != nil {
		return m.GetComposePathFunc(ws)
	}
	return ""
}

func (m *MockWorkspaceManager) GetStatePath(ws *workspace.Workspace) string {
	if m.GetStatePathFunc != nil {
		return m.GetStatePathFunc(ws)
	}
	return ""
}

func (m *MockWorkspaceManager) GetActiveWorkspace() (string, error) {
	if m.GetActiveWorkspaceFunc != nil {
		return m.GetActiveWorkspaceFunc()
	}
	return "", nil
}

func (m *MockWorkspaceManager) GetRoot(ws *workspace.Workspace) string {
	if m.GetRootFunc != nil {
		return m.GetRootFunc(ws)
	}
	return ""
}

func (m *MockWorkspaceManager) GetServicePath(ws *workspace.Workspace, serviceName string, svc config.Service) string {
	if m.GetServicePathFunc != nil {
		return m.GetServicePathFunc(ws, serviceName, svc)
	}
	return ""
}

func (m *MockWorkspaceManager) GetServiceDir(ws *workspace.Workspace, svc config.Service) string {
	if m.GetServiceDirFunc != nil {
		return m.GetServiceDirFunc(ws, svc)
	}
	return ""
}

func (m *MockWorkspaceManager) MigrateLegacyServices(ws *workspace.Workspace, deps *config.Deps) error {
	if m.MigrateLegacyServicesFunc != nil {
		return m.MigrateLegacyServicesFunc(ws, deps)
	}
	return nil
}

func (m *MockWorkspaceManager) ListWorkspaces() ([]string, error) {
	if m.ListWorkspacesFunc != nil {
		return m.ListWorkspacesFunc()
	}
	return nil, nil
}

func (m *MockWorkspaceManager) WorkspaceExists(workspaceName string) (bool, error) {
	if m.WorkspaceExistsFunc != nil {
		return m.WorkspaceExistsFunc(workspaceName)
	}
	return false, nil
}

func (m *MockWorkspaceManager) SetActiveWorkspace(workspaceName string) error {
	if m.SetActiveWorkspaceFunc != nil {
		return m.SetActiveWorkspaceFunc(workspaceName)
	}
	return nil
}

func (m *MockWorkspaceManager) DeleteWorkspace(workspaceName string) error {
	if m.DeleteWorkspaceFunc != nil {
		return m.DeleteWorkspaceFunc(workspaceName)
	}
	return nil
}
