package interfaces

import (
	"raioz/internal/config"
	"raioz/internal/workspace"
)

// Workspace represents a workspace structure (domain model)
// This is an alias for the concrete type, keeping it in interfaces allows domain layer to reference it
type Workspace = workspace.Workspace

// WorkspaceManager defines operations for workspace management
type WorkspaceManager interface {
	// Resolve resolves and creates the workspace structure
	Resolve(projectName string) (*Workspace, error)
	// GetBaseDir returns the base directory for workspaces
	GetBaseDir() (string, error)
	// GetBaseDirFromWorkspace returns the base directory from a workspace
	GetBaseDirFromWorkspace(ws *Workspace) string
	// GetComposePath returns the path to the docker-compose file
	GetComposePath(ws *Workspace) string
	// GetStatePath returns the path to the state file
	GetStatePath(ws *Workspace) string
	// GetActiveWorkspace returns the active workspace name
	GetActiveWorkspace() (string, error)
	// GetRoot returns the root path of a workspace
	GetRoot(ws *Workspace) string
	// GetServicePath returns the full path to a service directory
	GetServicePath(ws *Workspace, serviceName string, svc config.Service) string
	// GetServiceDir returns the base directory for a service type
	GetServiceDir(ws *Workspace, svc config.Service) string
	// MigrateLegacyServices migrates legacy service directory structures
	MigrateLegacyServices(ws *Workspace, deps *config.Deps) error
	// ListWorkspaces returns a list of workspace names
	ListWorkspaces() ([]string, error)
	// WorkspaceExists checks if a workspace exists
	WorkspaceExists(workspaceName string) (bool, error)
	// SetActiveWorkspace sets the active workspace
	SetActiveWorkspace(workspaceName string) error
}
