package interfaces

import (
	"context"

	"raioz/internal/host"
)

// HostRunner defines operations for running services directly on the host
type HostRunner interface {
	// StopServiceWithCommand stops a host service by PID with an optional stop command
	StopServiceWithCommand(ctx context.Context, pid int, stopCommand string) error
	// LoadProcessesState loads the host processes state for a workspace
	LoadProcessesState(ws *Workspace) (map[string]*host.ProcessInfo, error)
	// SaveProcessesState saves the host processes state for a workspace
	SaveProcessesState(ws *Workspace, processes map[string]*host.ProcessInfo) error
	// RemoveProcessesState removes the host processes state file
	RemoveProcessesState(ws *Workspace) error
	// DetectComposePath detects the compose path for a host service
	DetectComposePath(servicePath string, command string, explicitComposePath string) string
	// StopServiceWithCommandAndPath stops a host service by PID with an optional stop command and service path
	StopServiceWithCommandAndPath(ctx context.Context, pid int, stopCommand string, servicePath string) error
	// IsServiceRunning checks if a host service is still running by PID
	IsServiceRunning(pid int) (bool, error)
}
