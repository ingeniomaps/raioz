package docker

import (
	"context"

	"raioz/internal/config"
	dockerpkg "raioz/internal/docker"
	"raioz/internal/domain/interfaces"
	workspacepkg "raioz/internal/workspace"
)

// GenerateCompose generates a docker-compose file from dependencies
func (r *DockerRunnerImpl) GenerateCompose(
	deps *config.Deps, ws *interfaces.Workspace, projectDir string,
) (string, []string, error) {
	wsConcrete := (*workspacepkg.Workspace)(ws)
	return dockerpkg.GenerateCompose(deps, wsConcrete, projectDir)
}

// UpServicesWithContext starts specific Docker Compose services with context support
func (r *DockerRunnerImpl) UpServicesWithContext(
	ctx context.Context, composePath string, serviceNames []string,
) error {
	return dockerpkg.UpServicesWithContext(ctx, composePath, serviceNames)
}

// RestartServicesWithContext restarts specific Docker Compose services
func (r *DockerRunnerImpl) RestartServicesWithContext(
	ctx context.Context, composePath string, serviceNames []string,
) error {
	return dockerpkg.RestartServicesWithContext(ctx, composePath, serviceNames)
}

// ForceRecreateServicesWithContext recreates and starts services
func (r *DockerRunnerImpl) ForceRecreateServicesWithContext(
	ctx context.Context, composePath string, serviceNames []string,
) error {
	return dockerpkg.ForceRecreateServicesWithContext(ctx, composePath, serviceNames)
}

// ExecInService runs a command inside a running container
func (r *DockerRunnerImpl) ExecInService(
	ctx context.Context, composePath string, serviceName string,
	command []string, interactive bool,
) error {
	return dockerpkg.ExecInService(ctx, composePath, serviceName, command, interactive)
}

// WaitForServicesHealthy waits for services to become healthy
func (r *DockerRunnerImpl) WaitForServicesHealthy(
	ctx context.Context, composePath string, serviceNames []string,
	infraNames []string, projectName string,
) error {
	return dockerpkg.WaitForServicesHealthy(
		ctx, composePath, serviceNames, infraNames, projectName,
	)
}

// ValidatePorts checks if all ports in a project are available
func (r *DockerRunnerImpl) ValidatePorts(
	deps *config.Deps, baseDir string, projectName string,
) ([]interfaces.PortConflict, error) {
	conflicts, err := dockerpkg.ValidatePorts(deps, baseDir, projectName)
	if err != nil {
		return nil, err
	}
	result := make([]interfaces.PortConflict, len(conflicts))
	for i, c := range conflicts {
		result[i] = interfaces.PortConflict{
			Port:        c.Port,
			Project:     c.Project,
			Service:     c.Service,
			Alternative: c.Alternative,
		}
	}
	return result, nil
}

// FormatPortConflicts formats port conflicts for display
func (r *DockerRunnerImpl) FormatPortConflicts(conflicts []interfaces.PortConflict) string {
	dockerConflicts := make([]dockerpkg.PortConflict, len(conflicts))
	for i, c := range conflicts {
		dockerConflicts[i] = dockerpkg.PortConflict{
			Port:        c.Port,
			Project:     c.Project,
			Service:     c.Service,
			Alternative: c.Alternative,
		}
	}
	return dockerpkg.FormatPortConflicts(dockerConflicts)
}

// ValidateAllImages validates all images (services and infra) before compose generation
func (r *DockerRunnerImpl) ValidateAllImages(deps *config.Deps) error {
	return dockerpkg.ValidateAllImages(deps)
}

// EnsureNetworkWithConfigAndContext ensures a Docker network exists with optional subnet
func (r *DockerRunnerImpl) EnsureNetworkWithConfigAndContext(
	ctx context.Context, name string, subnet string, askConfirmation bool,
) error {
	cfg := dockerpkg.NetworkConfig{
		Name:   name,
		Subnet: subnet,
	}
	return dockerpkg.EnsureNetworkWithConfigAndContext(ctx, cfg, askConfirmation)
}

// EnsureVolumeWithContext ensures a named volume exists, creating if necessary
func (r *DockerRunnerImpl) EnsureVolumeWithContext(ctx context.Context, name string) error {
	return dockerpkg.EnsureVolumeWithContext(ctx, name)
}

// NormalizeVolumeName normalizes a volume name with project prefix
func (r *DockerRunnerImpl) NormalizeVolumeName(prefix string, name string) (string, error) {
	return dockerpkg.NormalizeVolumeName(prefix, name)
}

// NormalizeContainerName normalizes a container name
func (r *DockerRunnerImpl) NormalizeContainerName(
	workspace string, service string, project string, hasExplicitWorkspace bool,
) (string, error) {
	return dockerpkg.NormalizeContainerName(workspace, service, project, hasExplicitWorkspace)
}

// NormalizeInfraName normalizes an infra container name
func (r *DockerRunnerImpl) NormalizeInfraName(
	workspace string, infra string, project string, hasExplicitWorkspace bool,
) (string, error) {
	return dockerpkg.NormalizeInfraName(workspace, infra, project, hasExplicitWorkspace)
}

// GetContainerNameWithContext returns the container name for a service
func (r *DockerRunnerImpl) GetContainerNameWithContext(
	ctx context.Context, composePath string, serviceName string,
) (string, error) {
	return dockerpkg.GetContainerNameWithContext(ctx, composePath, serviceName)
}

// ResolveRelativeVolumes converts relative paths in bind mount volumes to absolute paths
func (r *DockerRunnerImpl) ResolveRelativeVolumes(volumes []string, projectDir string) ([]string, error) {
	return dockerpkg.ResolveRelativeVolumes(volumes, projectDir)
}

// AreServicesRunning checks if services are running in a compose project
func (r *DockerRunnerImpl) AreServicesRunning(composePath string, serviceNames []string) (bool, error) {
	return dockerpkg.AreServicesRunning(composePath, serviceNames)
}

// IsNetworkInUseWithContext checks if a Docker network is in use
func (r *DockerRunnerImpl) IsNetworkInUseWithContext(ctx context.Context, networkName string) (bool, error) {
	return dockerpkg.IsNetworkInUseWithContext(ctx, networkName)
}

// StopContainerWithContext stops a container by name
func (r *DockerRunnerImpl) StopContainerWithContext(ctx context.Context, containerName string) error {
	return dockerpkg.StopContainerWithContext(ctx, containerName)
}

// BuildServiceVolumesMap builds a map of service volumes from Deps configuration
func (r *DockerRunnerImpl) BuildServiceVolumesMap(deps *config.Deps) (map[string]interfaces.ServiceVolumes, error) {
	svcVols, err := dockerpkg.BuildServiceVolumesMap(deps)
	if err != nil {
		return nil, err
	}
	result := make(map[string]interfaces.ServiceVolumes, len(svcVols))
	for name, sv := range svcVols {
		result[name] = interfaces.ServiceVolumes{
			NamedVolumes: sv.NamedVolumes,
		}
	}
	return result, nil
}

// DetectSharedVolumes detects volumes shared between multiple services
func (r *DockerRunnerImpl) DetectSharedVolumes(services map[string]interfaces.ServiceVolumes) map[string][]string {
	dockerServices := make(map[string]dockerpkg.ServiceVolumes, len(services))
	for name, sv := range services {
		dockerServices[name] = dockerpkg.ServiceVolumes{
			NamedVolumes: sv.NamedVolumes,
		}
	}
	return dockerpkg.DetectSharedVolumes(dockerServices)
}

// FormatSharedVolumesWarning formats a warning message for shared volumes
func (r *DockerRunnerImpl) FormatSharedVolumesWarning(sharedVolumes map[string][]string) string {
	return dockerpkg.FormatSharedVolumesWarning(sharedVolumes)
}

// RemoveVolumeWithContext removes a named volume
func (r *DockerRunnerImpl) RemoveVolumeWithContext(ctx context.Context, name string) error {
	return dockerpkg.RemoveVolumeWithContext(ctx, name)
}
