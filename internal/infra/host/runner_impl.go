package host

import (
	"context"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	hostpkg "raioz/internal/host"
	workspacepkg "raioz/internal/workspace"
)

// Ensure HostRunnerImpl implements interfaces.HostRunner
var _ interfaces.HostRunner = (*HostRunnerImpl)(nil)

// HostRunnerImpl is the concrete implementation of HostRunner
type HostRunnerImpl struct{}

// NewHostRunner creates a new HostRunner implementation
func NewHostRunner() interfaces.HostRunner {
	return &HostRunnerImpl{}
}

// StartService starts a service directly on the host (without Docker)
func (r *HostRunnerImpl) StartService(
	ctx context.Context,
	ws *interfaces.Workspace,
	deps *config.Deps,
	serviceName string,
	svc config.Service,
	projectDir string,
) (*hostpkg.ProcessInfo, error) {
	wsConcrete := (*workspacepkg.Workspace)(ws)
	return hostpkg.StartService(ctx, wsConcrete, deps, serviceName, svc, projectDir)
}

// StopServiceWithCommand stops a host service by PID with an optional stop command
func (r *HostRunnerImpl) StopServiceWithCommand(ctx context.Context, pid int, stopCommand string) error {
	return hostpkg.StopServiceWithCommand(ctx, pid, stopCommand)
}

// LoadProcessesState loads the host processes state for a workspace
func (r *HostRunnerImpl) LoadProcessesState(ws *interfaces.Workspace) (map[string]*hostpkg.ProcessInfo, error) {
	wsConcrete := (*workspacepkg.Workspace)(ws)
	return hostpkg.LoadProcessesState(wsConcrete)
}

// SaveProcessesState saves the host processes state for a workspace
func (r *HostRunnerImpl) SaveProcessesState(ws *interfaces.Workspace, processes map[string]*hostpkg.ProcessInfo) error {
	wsConcrete := (*workspacepkg.Workspace)(ws)
	return hostpkg.SaveProcessesState(wsConcrete, processes)
}

// RemoveProcessesState removes the host processes state file
func (r *HostRunnerImpl) RemoveProcessesState(ws *interfaces.Workspace) error {
	wsConcrete := (*workspacepkg.Workspace)(ws)
	return hostpkg.RemoveProcessesState(wsConcrete)
}

// DetectComposePath detects the compose path for a host service
func (r *HostRunnerImpl) DetectComposePath(servicePath string, command string, explicitComposePath string) string {
	return hostpkg.DetectComposePath(servicePath, command, explicitComposePath)
}

// StopServiceWithCommandAndPath stops a host service by PID with an optional stop command and service path
func (r *HostRunnerImpl) StopServiceWithCommandAndPath(
	ctx context.Context, pid int, stopCommand string, servicePath string,
) error {
	return hostpkg.StopServiceWithCommandAndPath(ctx, pid, stopCommand, servicePath)
}

// IsServiceRunning checks if a host service is still running by PID
func (r *HostRunnerImpl) IsServiceRunning(pid int) (bool, error) {
	return hostpkg.IsServiceRunning(pid)
}
