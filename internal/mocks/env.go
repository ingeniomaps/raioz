package mocks

import (
	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/workspace"
)

// Compile-time check
var _ interfaces.EnvManager = (*MockEnvManager)(nil)

// MockEnvManager is a mock implementation of interfaces.EnvManager
type MockEnvManager struct {
	ResolveProjectEnvFunc       func(ws *workspace.Workspace, deps *models.Deps, projectDir string) (string, error)
	GenerateEnvFromTemplateFunc func(
		ws *workspace.Workspace, deps *models.Deps,
		serviceName string, servicePath string, svc models.Service,
		projectEnvPath string, projectDir string,
	) error
	WriteGlobalEnvVariablesFunc func(
		ws *workspace.Workspace, deps *models.Deps, projectDir string,
	) error
	ResolveEnvFilesFunc func(
		ws *workspace.Workspace, deps *models.Deps,
		serviceName string, envFiles []string,
		projectEnvPath string, includeProjectLevel bool,
		projectDir string,
	) ([]string, error)
}

func (m *MockEnvManager) ResolveProjectEnv(
	ws *workspace.Workspace, deps *models.Deps, projectDir string,
) (string, error) {
	if m.ResolveProjectEnvFunc != nil {
		return m.ResolveProjectEnvFunc(ws, deps, projectDir)
	}
	return "", nil
}

func (m *MockEnvManager) GenerateEnvFromTemplate(
	ws *workspace.Workspace, deps *models.Deps,
	serviceName string, servicePath string, svc models.Service,
	projectEnvPath string, projectDir string,
) error {
	if m.GenerateEnvFromTemplateFunc != nil {
		return m.GenerateEnvFromTemplateFunc(ws, deps, serviceName, servicePath, svc, projectEnvPath, projectDir)
	}
	return nil
}

func (m *MockEnvManager) WriteGlobalEnvVariables(ws *workspace.Workspace, deps *models.Deps, projectDir string) error {
	if m.WriteGlobalEnvVariablesFunc != nil {
		return m.WriteGlobalEnvVariablesFunc(ws, deps, projectDir)
	}
	return nil
}

func (m *MockEnvManager) ResolveEnvFiles(
	ws *workspace.Workspace, deps *models.Deps,
	serviceName string, envFiles []string,
	projectEnvPath string, includeProjectLevel bool,
	projectDir string,
) ([]string, error) {
	if m.ResolveEnvFilesFunc != nil {
		return m.ResolveEnvFilesFunc(ws, deps, serviceName, envFiles, projectEnvPath, includeProjectLevel, projectDir)
	}
	return nil, nil
}
