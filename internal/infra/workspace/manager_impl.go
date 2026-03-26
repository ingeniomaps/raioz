package workspace

import (
	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	workspacepkg "raioz/internal/workspace"
)

// Ensure WorkspaceManagerImpl implements all methods correctly
var _ interfaces.WorkspaceManager = (*WorkspaceManagerImpl)(nil)

// Ensure WorkspaceManagerImpl implements interfaces.WorkspaceManager
var _ interfaces.WorkspaceManager = (*WorkspaceManagerImpl)(nil)

// WorkspaceManagerImpl is the concrete implementation of WorkspaceManager
type WorkspaceManagerImpl struct{}

// NewWorkspaceManager creates a new WorkspaceManager implementation
func NewWorkspaceManager() interfaces.WorkspaceManager {
	return &WorkspaceManagerImpl{}
}

// Resolve resolves and creates the workspace structure
func (m *WorkspaceManagerImpl) Resolve(projectName string) (*interfaces.Workspace, error) {
	ws, err := workspacepkg.Resolve(projectName)
	if err != nil {
		return nil, err
	}
	// Convert to interface type (they're the same, just different type alias)
	return (*interfaces.Workspace)(ws), nil
}

// GetBaseDir returns the base directory for workspaces
func (m *WorkspaceManagerImpl) GetBaseDir() (string, error) {
	return workspacepkg.GetBaseDir()
}

// GetBaseDirFromWorkspace returns the base directory from a workspace
func (m *WorkspaceManagerImpl) GetBaseDirFromWorkspace(ws *interfaces.Workspace) string {
	// Convert interfaces.Workspace (alias) to concrete workspace.Workspace for internal use
	wsConcrete := (*workspacepkg.Workspace)(ws)
	return workspacepkg.GetBaseDirFromWorkspace(wsConcrete)
}

// GetComposePath returns the path to the docker-compose file
func (m *WorkspaceManagerImpl) GetComposePath(ws *interfaces.Workspace) string {
	// Convert interfaces.Workspace (alias) to concrete workspace.Workspace for internal use
	wsConcrete := (*workspacepkg.Workspace)(ws)
	return workspacepkg.GetComposePath(wsConcrete)
}

// GetStatePath returns the path to the state file
func (m *WorkspaceManagerImpl) GetStatePath(ws *interfaces.Workspace) string {
	// Convert interfaces.Workspace (alias) to concrete workspace.Workspace for internal use
	wsConcrete := (*workspacepkg.Workspace)(ws)
	return workspacepkg.GetStatePath(wsConcrete)
}

// GetActiveWorkspace returns the active workspace name
func (m *WorkspaceManagerImpl) GetActiveWorkspace() (string, error) {
	return workspacepkg.GetActiveWorkspace()
}

// GetRoot returns the root path of a workspace
func (m *WorkspaceManagerImpl) GetRoot(ws *interfaces.Workspace) string {
	// Convert interfaces.Workspace (alias) to concrete workspace.Workspace for internal use
	wsConcrete := (*workspacepkg.Workspace)(ws)
	return wsConcrete.Root
}

// GetServicePath returns the full path to a service directory
func (m *WorkspaceManagerImpl) GetServicePath(ws *interfaces.Workspace, serviceName string, svc config.Service) string {
	wsConcrete := (*workspacepkg.Workspace)(ws)
	return workspacepkg.GetServicePath(wsConcrete, serviceName, svc)
}

// GetServiceDir returns the base directory for a service type
func (m *WorkspaceManagerImpl) GetServiceDir(ws *interfaces.Workspace, svc config.Service) string {
	wsConcrete := (*workspacepkg.Workspace)(ws)
	return workspacepkg.GetServiceDir(wsConcrete, svc)
}

// MigrateLegacyServices migrates legacy service directory structures
func (m *WorkspaceManagerImpl) MigrateLegacyServices(ws *interfaces.Workspace, deps *config.Deps) error {
	wsConcrete := (*workspacepkg.Workspace)(ws)
	return workspacepkg.MigrateLegacyServices(wsConcrete, deps)
}

// ListWorkspaces returns a list of workspace names
func (m *WorkspaceManagerImpl) ListWorkspaces() ([]string, error) {
	return workspacepkg.ListWorkspaces()
}

// WorkspaceExists checks if a workspace exists
func (m *WorkspaceManagerImpl) WorkspaceExists(workspaceName string) (bool, error) {
	return workspacepkg.WorkspaceExists(workspaceName)
}

// SetActiveWorkspace sets the active workspace
func (m *WorkspaceManagerImpl) SetActiveWorkspace(workspaceName string) error {
	return workspacepkg.SetActiveWorkspace(workspaceName)
}
