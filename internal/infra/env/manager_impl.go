package env

import (
	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	envpkg "raioz/internal/env"
	workspacepkg "raioz/internal/workspace"
)

// Ensure EnvManagerImpl implements interfaces.EnvManager
var _ interfaces.EnvManager = (*EnvManagerImpl)(nil)

// EnvManagerImpl is the concrete implementation of EnvManager
type EnvManagerImpl struct{}

// NewEnvManager creates a new EnvManager implementation
func NewEnvManager() interfaces.EnvManager {
	return &EnvManagerImpl{}
}

// ResolveProjectEnv resolves project.env configuration
func (m *EnvManagerImpl) ResolveProjectEnv(
	ws *interfaces.Workspace, deps *config.Deps, projectDir string,
) (string, error) {
	wsConcrete := (*workspacepkg.Workspace)(ws)
	return envpkg.ResolveProjectEnv(wsConcrete, deps, projectDir)
}

// GenerateEnvFromTemplate generates a .env file from a template if found
func (m *EnvManagerImpl) GenerateEnvFromTemplate(
	ws *interfaces.Workspace, deps *config.Deps,
	serviceName string, servicePath string, svc config.Service,
	projectEnvPath string, projectDir string,
) error {
	wsConcrete := (*workspacepkg.Workspace)(ws)
	return envpkg.GenerateEnvFromTemplate(
		wsConcrete, deps, serviceName, servicePath,
		svc, projectEnvPath, projectDir,
	)
}

// WriteGlobalEnvVariables writes global environment variables to the workspace
func (m *EnvManagerImpl) WriteGlobalEnvVariables(ws *interfaces.Workspace, deps *config.Deps, projectDir string) error {
	wsConcrete := (*workspacepkg.Workspace)(ws)
	return envpkg.WriteGlobalEnvVariables(wsConcrete, deps, projectDir)
}

// ResolveEnvFiles resolves and returns paths to env files for a service or infra
func (m *EnvManagerImpl) ResolveEnvFiles(
	ws *interfaces.Workspace, deps *config.Deps,
	serviceName string, envFiles []string,
	projectEnvPath string, includeProjectLevel bool, projectDir string,
) ([]string, error) {
	wsConcrete := (*workspacepkg.Workspace)(ws)
	return envpkg.ResolveEnvFiles(
		wsConcrete, deps, serviceName, envFiles,
		projectEnvPath, includeProjectLevel, projectDir,
	)
}
