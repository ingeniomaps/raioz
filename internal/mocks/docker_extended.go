package mocks

import (
	"context"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
)

func (m *MockDockerRunner) CleanProjectWithContext(ctx context.Context, composePath string, dryRun bool) ([]string, error) {
	if m.CleanProjectWithContextFunc != nil {
		return m.CleanProjectWithContextFunc(ctx, composePath, dryRun)
	}
	return nil, nil
}

func (m *MockDockerRunner) CleanAllProjectsWithContext(ctx context.Context, baseDir string, dryRun bool) ([]string, error) {
	if m.CleanAllProjectsWithContextFunc != nil {
		return m.CleanAllProjectsWithContextFunc(ctx, baseDir, dryRun)
	}
	return nil, nil
}

func (m *MockDockerRunner) CleanUnusedImagesWithContext(ctx context.Context, dryRun bool) ([]string, error) {
	if m.CleanUnusedImagesWithContextFunc != nil {
		return m.CleanUnusedImagesWithContextFunc(ctx, dryRun)
	}
	return nil, nil
}

func (m *MockDockerRunner) CleanUnusedVolumesWithContext(ctx context.Context, dryRun bool, force bool) ([]string, error) {
	if m.CleanUnusedVolumesWithContextFunc != nil {
		return m.CleanUnusedVolumesWithContextFunc(ctx, dryRun, force)
	}
	return nil, nil
}

func (m *MockDockerRunner) CleanUnusedNetworksWithContext(ctx context.Context, dryRun bool) ([]string, error) {
	if m.CleanUnusedNetworksWithContextFunc != nil {
		return m.CleanUnusedNetworksWithContextFunc(ctx, dryRun)
	}
	return nil, nil
}

func (m *MockDockerRunner) GetAllActivePorts(baseDir string) ([]interfaces.PortInfo, error) {
	if m.GetAllActivePortsFunc != nil {
		return m.GetAllActivePortsFunc(baseDir)
	}
	return nil, nil
}

func (m *MockDockerRunner) GenerateCompose(deps *config.Deps, ws *interfaces.Workspace, projectDir string) (string, []string, error) {
	if m.GenerateComposeFunc != nil {
		return m.GenerateComposeFunc(deps, ws, projectDir)
	}
	return "", nil, nil
}

func (m *MockDockerRunner) UpServicesWithContext(ctx context.Context, composePath string, serviceNames []string) error {
	if m.UpServicesWithContextFunc != nil {
		return m.UpServicesWithContextFunc(ctx, composePath, serviceNames)
	}
	return nil
}

func (m *MockDockerRunner) RestartServicesWithContext(ctx context.Context, composePath string, serviceNames []string) error {
	if m.RestartServicesWithContextFunc != nil {
		return m.RestartServicesWithContextFunc(ctx, composePath, serviceNames)
	}
	return nil
}

func (m *MockDockerRunner) ForceRecreateServicesWithContext(ctx context.Context, composePath string, serviceNames []string) error {
	if m.ForceRecreateServicesWithContextFunc != nil {
		return m.ForceRecreateServicesWithContextFunc(ctx, composePath, serviceNames)
	}
	return nil
}

func (m *MockDockerRunner) ExecInService(ctx context.Context, composePath string, serviceName string, command []string, interactive bool) error {
	if m.ExecInServiceFunc != nil {
		return m.ExecInServiceFunc(ctx, composePath, serviceName, command, interactive)
	}
	return nil
}

func (m *MockDockerRunner) WaitForServicesHealthy(ctx context.Context, composePath string, serviceNames []string, infraNames []string, projectName string) error {
	if m.WaitForServicesHealthyFunc != nil {
		return m.WaitForServicesHealthyFunc(ctx, composePath, serviceNames, infraNames, projectName)
	}
	return nil
}

func (m *MockDockerRunner) ValidatePorts(deps *config.Deps, baseDir string, projectName string) ([]interfaces.PortConflict, error) {
	if m.ValidatePortsFunc != nil {
		return m.ValidatePortsFunc(deps, baseDir, projectName)
	}
	return nil, nil
}

func (m *MockDockerRunner) FormatPortConflicts(conflicts []interfaces.PortConflict) string {
	if m.FormatPortConflictsFunc != nil {
		return m.FormatPortConflictsFunc(conflicts)
	}
	return ""
}

func (m *MockDockerRunner) ValidateAllImages(deps *config.Deps) error {
	if m.ValidateAllImagesFunc != nil {
		return m.ValidateAllImagesFunc(deps)
	}
	return nil
}

func (m *MockDockerRunner) EnsureNetworkWithConfigAndContext(ctx context.Context, name string, subnet string, askConfirmation bool) error {
	if m.EnsureNetworkWithConfigAndContextFunc != nil {
		return m.EnsureNetworkWithConfigAndContextFunc(ctx, name, subnet, askConfirmation)
	}
	return nil
}

func (m *MockDockerRunner) EnsureVolumeWithContext(ctx context.Context, name string) error {
	if m.EnsureVolumeWithContextFunc != nil {
		return m.EnsureVolumeWithContextFunc(ctx, name)
	}
	return nil
}

func (m *MockDockerRunner) NormalizeVolumeName(prefix string, name string) (string, error) {
	if m.NormalizeVolumeNameFunc != nil {
		return m.NormalizeVolumeNameFunc(prefix, name)
	}
	return "", nil
}

func (m *MockDockerRunner) NormalizeContainerName(workspace string, service string, project string, hasExplicitWorkspace bool) (string, error) {
	if m.NormalizeContainerNameFunc != nil {
		return m.NormalizeContainerNameFunc(workspace, service, project, hasExplicitWorkspace)
	}
	return "", nil
}

func (m *MockDockerRunner) NormalizeInfraName(workspace string, infra string, project string, hasExplicitWorkspace bool) (string, error) {
	if m.NormalizeInfraNameFunc != nil {
		return m.NormalizeInfraNameFunc(workspace, infra, project, hasExplicitWorkspace)
	}
	return "", nil
}

func (m *MockDockerRunner) GetContainerNameWithContext(ctx context.Context, composePath string, serviceName string) (string, error) {
	if m.GetContainerNameWithContextFunc != nil {
		return m.GetContainerNameWithContextFunc(ctx, composePath, serviceName)
	}
	return "", nil
}

func (m *MockDockerRunner) ResolveRelativeVolumes(volumes []string, projectDir string) ([]string, error) {
	if m.ResolveRelativeVolumesFunc != nil {
		return m.ResolveRelativeVolumesFunc(volumes, projectDir)
	}
	return nil, nil
}

func (m *MockDockerRunner) AreServicesRunning(composePath string, serviceNames []string) (bool, error) {
	if m.AreServicesRunningFunc != nil {
		return m.AreServicesRunningFunc(composePath, serviceNames)
	}
	return false, nil
}

func (m *MockDockerRunner) IsNetworkInUseWithContext(ctx context.Context, networkName string) (bool, error) {
	if m.IsNetworkInUseWithContextFunc != nil {
		return m.IsNetworkInUseWithContextFunc(ctx, networkName)
	}
	return false, nil
}

func (m *MockDockerRunner) StopContainerWithContext(ctx context.Context, containerName string) error {
	if m.StopContainerWithContextFunc != nil {
		return m.StopContainerWithContextFunc(ctx, containerName)
	}
	return nil
}

func (m *MockDockerRunner) BuildServiceVolumesMap(deps *config.Deps) (map[string]interfaces.ServiceVolumes, error) {
	if m.BuildServiceVolumesMapFunc != nil {
		return m.BuildServiceVolumesMapFunc(deps)
	}
	return nil, nil
}

func (m *MockDockerRunner) DetectSharedVolumes(services map[string]interfaces.ServiceVolumes) map[string][]string {
	if m.DetectSharedVolumesFunc != nil {
		return m.DetectSharedVolumesFunc(services)
	}
	return nil
}

func (m *MockDockerRunner) FormatSharedVolumesWarning(sharedVolumes map[string][]string) string {
	if m.FormatSharedVolumesWarningFunc != nil {
		return m.FormatSharedVolumesWarningFunc(sharedVolumes)
	}
	return ""
}

func (m *MockDockerRunner) RemoveVolumeWithContext(ctx context.Context, name string) error {
	if m.RemoveVolumeWithContextFunc != nil {
		return m.RemoveVolumeWithContextFunc(ctx, name)
	}
	return nil
}
