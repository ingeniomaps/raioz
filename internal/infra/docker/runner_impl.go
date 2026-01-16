package docker

import (
	"context"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	dockerpkg "raioz/internal/docker"
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
				Restarts: "", // Not available in docker.ServiceInfo
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

// FormatStatusTable formats service information as a table
func (r *DockerRunnerImpl) FormatStatusTable(services map[string]*interfaces.ServiceInfo, jsonOutput bool) error {
	// Convert from interfaces.ServiceInfo to docker.ServiceInfo
	dockerServices := make(map[string]*dockerpkg.ServiceInfo)
	for name, info := range services {
		if info != nil {
			dockerServices[name] = &dockerpkg.ServiceInfo{
				Name:      name,
				Status:    info.Status,
				Uptime:    info.Uptime,
				Memory:    info.Memory,
				CPU:       info.CPU,
				Image:     info.Image,
				Version:   info.Commit,
				Health:    info.Health,
				LastUpdated: "", // Not available in interfaces.ServiceInfo
				Linked:    false, // Not available in interfaces.ServiceInfo
				LinkTarget: "", // Not available in interfaces.ServiceInfo
			}
		}
	}
	return dockerpkg.FormatStatusTable(dockerServices, jsonOutput)
}
