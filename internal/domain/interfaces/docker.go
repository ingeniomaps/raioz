package interfaces

import (
	"context"
	models "raioz/internal/domain/models"
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

// LogsOptions contains options for viewing service logs
type LogsOptions struct {
	Follow   bool
	Tail     int
	Services []string
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
	GetServicesInfoWithContext(ctx context.Context, composePath string, serviceNames []string, projectName string, services map[string]models.Service, ws *Workspace) (map[string]*ServiceInfo, error)
	// GetNetworkProjects returns list of projects using a network
	GetNetworkProjects(networkName string, baseDir string) ([]string, error)
	// GetVolumeProjects returns list of projects using a volume
	GetVolumeProjects(volumeName string, baseDir string) ([]string, error)
	// ExtractNamedVolumes extracts named volume names from volume strings
	ExtractNamedVolumes(volumes []string) ([]string, error)
	// FormatStatusTable formats service information as a table
	FormatStatusTable(services map[string]*ServiceInfo, jsonOutput bool) error
	// GetAvailableServicesWithContext returns the list of services defined in a compose file
	GetAvailableServicesWithContext(ctx context.Context, composePath string) ([]string, error)
	// ViewLogsWithContext displays logs for services
	ViewLogsWithContext(ctx context.Context, composePath string, opts LogsOptions) error
	// CleanProjectWithContext cleans a specific project's stopped containers
	CleanProjectWithContext(ctx context.Context, composePath string, dryRun bool) ([]string, error)
	// CleanAllProjectsWithContext cleans all projects' stopped containers
	CleanAllProjectsWithContext(ctx context.Context, baseDir string, dryRun bool) ([]string, error)
	// CleanUnusedImagesWithContext removes unused Docker images
	CleanUnusedImagesWithContext(ctx context.Context, dryRun bool) ([]string, error)
	// CleanUnusedVolumesWithContext removes unused Docker volumes
	CleanUnusedVolumesWithContext(ctx context.Context, dryRun bool, force bool) ([]string, error)
	// CleanUnusedNetworksWithContext removes unused Docker networks
	CleanUnusedNetworksWithContext(ctx context.Context, dryRun bool) ([]string, error)
	// GetAllActivePorts returns all active ports across projects
	GetAllActivePorts(baseDir string) ([]PortInfo, error)
	// GenerateCompose generates a docker-compose file from dependencies
	GenerateCompose(deps *models.Deps, ws *Workspace, projectDir string) (string, []string, error)
	// UpServicesWithContext starts specific Docker Compose services with context support
	UpServicesWithContext(ctx context.Context, composePath string, serviceNames []string) error
	// WaitForServicesHealthy waits for services to become healthy
	WaitForServicesHealthy(ctx context.Context, composePath string, serviceNames []string, infraNames []string, projectName string) error
	// ValidatePorts checks if all ports in a project are available
	ValidatePorts(deps *models.Deps, baseDir string, projectName string) ([]PortConflict, error)
	// FormatPortConflicts formats port conflicts for display
	FormatPortConflicts(conflicts []PortConflict) string
	// ValidateAllImages validates all images (services and infra) before compose generation
	ValidateAllImages(deps *models.Deps) error
	// EnsureNetworkWithConfigAndContext ensures a Docker network exists with optional subnet
	EnsureNetworkWithConfigAndContext(ctx context.Context, name string, subnet string, askConfirmation bool) error
	// EnsureVolumeWithContext ensures a named volume exists, creating if necessary
	EnsureVolumeWithContext(ctx context.Context, name string) error
	// NormalizeVolumeName normalizes a volume name with project prefix
	NormalizeVolumeName(prefix string, name string) (string, error)
	// NormalizeContainerName normalizes a container name
	NormalizeContainerName(workspace string, service string, project string, hasExplicitWorkspace bool) (string, error)
	// NormalizeInfraName normalizes an infra container name
	NormalizeInfraName(workspace string, infra string, project string, hasExplicitWorkspace bool) (string, error)
	// GetContainerNameWithContext returns the container name for a service in a compose file
	GetContainerNameWithContext(ctx context.Context, composePath string, serviceName string) (string, error)
	// ResolveRelativeVolumes converts relative paths in bind mount volumes to absolute paths
	ResolveRelativeVolumes(volumes []string, projectDir string) ([]string, error)
	// AreServicesRunning checks if services are running in a compose project
	AreServicesRunning(composePath string, serviceNames []string) (bool, error)
	// IsNetworkInUseWithContext checks if a Docker network is in use
	IsNetworkInUseWithContext(ctx context.Context, networkName string) (bool, error)
	// StopContainerWithContext stops a container by name
	StopContainerWithContext(ctx context.Context, containerName string) error
	// BuildServiceVolumesMap builds a map of service volumes from Deps configuration
	BuildServiceVolumesMap(deps *models.Deps) (map[string]ServiceVolumes, error)
	// DetectSharedVolumes detects volumes shared between multiple services
	DetectSharedVolumes(services map[string]ServiceVolumes) map[string][]string
	// FormatSharedVolumesWarning formats a warning message for shared volumes
	FormatSharedVolumesWarning(sharedVolumes map[string][]string) string
	// RemoveVolumeWithContext removes a named volume
	RemoveVolumeWithContext(ctx context.Context, name string) error
}

// PortInfo represents information about an active port
type PortInfo struct {
	Port          string
	Project       string
	Service       string
	HostPort      int
	ContainerPort int
}

// PortConflict represents a port conflict with another project
type PortConflict struct {
	Port        string
	Project     string
	Service     string
	Alternative string
}

// ServiceVolumes represents the volumes used by a service
type ServiceVolumes struct {
	NamedVolumes []string
}
