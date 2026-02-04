package interfaces

import (
	"context"
	"raioz/internal/config"
)

// ServiceInfo represents information about a Docker service
type ServiceInfo struct {
	Status   string
	Uptime   string
	CPU      string
	Memory   string
	Image    string
	Commit   string
	Branch   string
	Health   string
	Restarts string
	Ports    []string
}

// DockerRunner defines operations for running Docker Compose commands
type DockerRunner interface {
	// Up starts Docker Compose services
	Up(composePath string) error
	// UpWithContext starts Docker Compose services with context support
	UpWithContext(ctx context.Context, composePath string) error
	// Down stops Docker Compose services
	Down(composePath string) error
	// DownWithContext stops Docker Compose services with context support
	DownWithContext(ctx context.Context, composePath string) error
	// StopServiceWithContext stops and removes only one service from the compose project (for conflict resolution)
	StopServiceWithContext(ctx context.Context, composePath string, serviceName string) error
	// GetServicesStatus returns the status of services
	GetServicesStatus(composePath string) (map[string]string, error)
	// GetServicesStatusWithContext returns the status of services with context support
	GetServicesStatusWithContext(ctx context.Context, composePath string) (map[string]string, error)
	// GetServicesInfoWithContext returns detailed information about services
	GetServicesInfoWithContext(ctx context.Context, composePath string, serviceNames []string, projectName string, services map[string]config.Service, ws *Workspace) (map[string]*ServiceInfo, error)
	// GetNetworkProjects returns list of projects using a network
	GetNetworkProjects(networkName string, baseDir string) ([]string, error)
	// GetVolumeProjects returns list of projects using a volume
	GetVolumeProjects(volumeName string, baseDir string) ([]string, error)
	// ExtractNamedVolumes extracts named volume names from volume strings
	ExtractNamedVolumes(volumes []string) ([]string, error)
	// FormatStatusTable formats service information as a table
	FormatStatusTable(services map[string]*ServiceInfo, jsonOutput bool) error
}
