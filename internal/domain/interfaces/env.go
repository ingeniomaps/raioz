package interfaces

import (
	models "raioz/internal/domain/models"
)

// EnvManager defines operations for managing environment variables and env files
type EnvManager interface {
	// ResolveProjectEnv resolves project.env configuration
	ResolveProjectEnv(ws *Workspace, deps *models.Deps, projectDir string) (string, error)
	// GenerateEnvFromTemplate generates a .env file from a template
	GenerateEnvFromTemplate(
		ws *Workspace, deps *models.Deps,
		serviceName string, servicePath string,
		svc models.Service, projectEnvPath string,
		projectDir string,
	) error
	// WriteGlobalEnvVariables writes global environment variables to the workspace
	WriteGlobalEnvVariables(ws *Workspace, deps *models.Deps, projectDir string) error
	// ResolveEnvFiles resolves and returns paths to env files
	ResolveEnvFiles(
		ws *Workspace, deps *models.Deps,
		serviceName string, envFiles []string,
		projectEnvPath string,
		includeProjectLevel bool, projectDir string,
	) ([]string, error)
}
