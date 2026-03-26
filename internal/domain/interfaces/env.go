package interfaces

import (
	"raioz/internal/config"
)

// EnvManager defines operations for managing environment variables and env files
type EnvManager interface {
	// ResolveProjectEnv resolves project.env configuration
	ResolveProjectEnv(ws *Workspace, deps *config.Deps, projectDir string) (string, error)
	// GenerateEnvFromTemplate generates a .env file from a template if found
	GenerateEnvFromTemplate(ws *Workspace, deps *config.Deps, serviceName string, servicePath string, svc config.Service, projectEnvPath string, projectDir string) error
	// WriteGlobalEnvVariables writes global environment variables to the workspace
	WriteGlobalEnvVariables(ws *Workspace, deps *config.Deps, projectDir string) error
	// ResolveEnvFiles resolves and returns paths to env files for a service or infra
	ResolveEnvFiles(ws *Workspace, deps *config.Deps, serviceName string, envFiles []string, projectEnvPath string, includeProjectLevel bool, projectDir string) ([]string, error)
}
