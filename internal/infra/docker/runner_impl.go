package docker

import (
	"context"

	"raioz/internal/config"
	dockerpkg "raioz/internal/docker"
	"raioz/internal/domain/interfaces"
	workspacepkg "raioz/internal/workspace"
)

// Ensure DockerRunnerImpl implements interfaces.DockerRunner
var _ interfaces.DockerRunner = (*DockerRunnerImpl)(nil)

// DockerRunnerImpl is the concrete implementation of DockerRunner
type DockerRunnerImpl struct{}

// NewDockerRunner creates a new DockerRunner implementation
func NewDockerRunner() interfaces.DockerRunner {
	return &DockerRunnerImpl{}
}

// Up starts Docker Compose services
func (r *DockerRunnerImpl) Up(composePath string) error {
	return dockerpkg.Up(composePath)
}

// UpWithContext starts Docker Compose services with context support
func (r *DockerRunnerImpl) UpWithContext(ctx context.Context, composePath string) error {
	return dockerpkg.UpWithContext(ctx, composePath)
}

// Down stops Docker Compose services
func (r *DockerRunnerImpl) Down(composePath string) error {
	return dockerpkg.Down(composePath)
}

// DownWithContext stops Docker Compose services with context support
func (r *DockerRunnerImpl) DownWithContext(ctx context.Context, composePath string) error {
	return dockerpkg.DownWithContext(ctx, composePath)
}

// StopServiceWithContext stops and removes only one service from the compose project
func (r *DockerRunnerImpl) StopServiceWithContext(ctx context.Context, composePath string, serviceName string) error {
	return dockerpkg.StopServiceWithContext(ctx, composePath, serviceName)
}

// GetServicesStatus returns the status of services
func (r *DockerRunnerImpl) GetServicesStatus(composePath string) (map[string]string, error) {
	return dockerpkg.GetServicesStatus(composePath)
}

// GetServicesStatusWithContext returns the status of services with context support
func (r *DockerRunnerImpl) GetServicesStatusWithContext(ctx context.Context, composePath string) (map[string]string, error) {
	return dockerpkg.GetServicesStatusWithContext(ctx, composePath)
}

// GetServicesInfoWithContext returns detailed information about services
func (r *DockerRunnerImpl) GetServicesInfoWithContext(ctx context.Context, composePath string, serviceNames []string, projectName string, services map[string]config.Service, ws *interfaces.Workspace) (map[string]*interfaces.ServiceInfo, error) {
	// Convert interfaces.Workspace (alias) to concrete workspace.Workspace for internal use
	wsConcrete := (*workspacepkg.Workspace)(ws)
	servicesInfo, err := dockerpkg.GetServicesInfoWithContext(ctx, composePath, serviceNames, projectName, services, wsConcrete)
	if err != nil {
		return nil, err
	}

	// Convert from docker.ServiceInfo to interfaces.ServiceInfo
	result := make(map[string]*interfaces.ServiceInfo)
	for name, info := range servicesInfo {
		if info != nil {
			result[name] = &interfaces.ServiceInfo{
				Status:   info.Status,
				Uptime:   info.Uptime,
				CPU:      info.CPU,
				Memory:   info.Memory,
				Image:    info.Image,
				Commit:   info.Version,
				Branch:   "", // Not available in docker.ServiceInfo
				Health:   info.Health,
				Restarts: "",         // Not available in docker.ServiceInfo
				Ports:    []string{}, // Not available in docker.ServiceInfo
			}
		}
	}
	return result, nil
}

// GetNetworkProjects returns list of projects using a network
func (r *DockerRunnerImpl) GetNetworkProjects(networkName string, baseDir string) ([]string, error) {
	return dockerpkg.GetNetworkProjects(networkName, baseDir)
}

// GetVolumeProjects returns list of projects using a volume
func (r *DockerRunnerImpl) GetVolumeProjects(volumeName string, baseDir string) ([]string, error) {
	return dockerpkg.GetVolumeProjects(volumeName, baseDir)
}

// ExtractNamedVolumes extracts named volume names from volume strings
func (r *DockerRunnerImpl) ExtractNamedVolumes(volumes []string) ([]string, error) {
	return dockerpkg.ExtractNamedVolumes(volumes)
}

// GetAvailableServicesWithContext returns the list of services defined in a compose file
func (r *DockerRunnerImpl) GetAvailableServicesWithContext(ctx context.Context, composePath string) ([]string, error) {
	return dockerpkg.GetAvailableServicesWithContext(ctx, composePath)
}

// ViewLogsWithContext displays logs for services
func (r *DockerRunnerImpl) ViewLogsWithContext(ctx context.Context, composePath string, opts interfaces.LogsOptions) error {
	return dockerpkg.ViewLogsWithContext(ctx, composePath, dockerpkg.LogsOptions{
		Follow:   opts.Follow,
		Tail:     opts.Tail,
		Services: opts.Services,
	})
}

// CleanProjectWithContext cleans a specific project's stopped containers
func (r *DockerRunnerImpl) CleanProjectWithContext(ctx context.Context, composePath string, dryRun bool) ([]string, error) {
	return dockerpkg.CleanProjectWithContext(ctx, composePath, dryRun)
}

// CleanAllProjectsWithContext cleans all projects' stopped containers
func (r *DockerRunnerImpl) CleanAllProjectsWithContext(ctx context.Context, baseDir string, dryRun bool) ([]string, error) {
	return dockerpkg.CleanAllProjectsWithContext(ctx, baseDir, dryRun)
}

// CleanUnusedImagesWithContext removes unused Docker images
func (r *DockerRunnerImpl) CleanUnusedImagesWithContext(ctx context.Context, dryRun bool) ([]string, error) {
	return dockerpkg.CleanUnusedImagesWithContext(ctx, dryRun)
}

// CleanUnusedVolumesWithContext removes unused Docker volumes
func (r *DockerRunnerImpl) CleanUnusedVolumesWithContext(ctx context.Context, dryRun bool, force bool) ([]string, error) {
	return dockerpkg.CleanUnusedVolumesWithContext(ctx, dryRun, force)
}

// CleanUnusedNetworksWithContext removes unused Docker networks
func (r *DockerRunnerImpl) CleanUnusedNetworksWithContext(ctx context.Context, dryRun bool) ([]string, error) {
	return dockerpkg.CleanUnusedNetworksWithContext(ctx, dryRun)
}

// GetAllActivePorts returns all active ports across projects
func (r *DockerRunnerImpl) GetAllActivePorts(baseDir string) ([]interfaces.PortInfo, error) {
	ports, err := dockerpkg.GetAllActivePorts(baseDir)
	if err != nil {
		return nil, err
	}
	result := make([]interfaces.PortInfo, len(ports))
	for i, p := range ports {
		result[i] = interfaces.PortInfo{
			Port:          p.Port,
			Project:       p.Project,
			Service:       p.Service,
			HostPort:      p.HostPort,
			ContainerPort: p.ContainerPort,
		}
	}
	return result, nil
}

// FormatStatusTable formats service information as a table
func (r *DockerRunnerImpl) FormatStatusTable(services map[string]*interfaces.ServiceInfo, jsonOutput bool) error {
	// Convert from interfaces.ServiceInfo to docker.ServiceInfo
	dockerServices := make(map[string]*dockerpkg.ServiceInfo)
	for name, info := range services {
		if info != nil {
			dockerServices[name] = &dockerpkg.ServiceInfo{
				Name:        name,
				Status:      info.Status,
				Uptime:      info.Uptime,
				Memory:      info.Memory,
				CPU:         info.CPU,
				Image:       info.Image,
				Version:     info.Commit,
				Health:      info.Health,
				LastUpdated: "",    // Not available in interfaces.ServiceInfo
				Linked:      false, // Not available in interfaces.ServiceInfo
				LinkTarget:  "",    // Not available in interfaces.ServiceInfo
			}
		}
	}
	return dockerpkg.FormatStatusTable(dockerServices, jsonOutput)
}
