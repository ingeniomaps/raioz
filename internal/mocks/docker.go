package mocks

import (
	"context"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
)

// Compile-time check
var _ interfaces.DockerRunner = (*MockDockerRunner)(nil)

// MockDockerRunner is a mock implementation of interfaces.DockerRunner
type MockDockerRunner struct {
	UpFunc                           func(composePath string) error
	UpWithContextFunc                func(ctx context.Context, composePath string) error
	DownFunc                         func(composePath string) error
	DownWithContextFunc              func(ctx context.Context, composePath string) error
	StopServiceWithContextFunc       func(ctx context.Context, composePath string, serviceName string) error
	GetServicesStatusFunc            func(composePath string) (map[string]string, error)
	GetServicesStatusWithContextFunc func(ctx context.Context, composePath string) (map[string]string, error)
	GetServicesInfoWithContextFunc   func(
		ctx context.Context, composePath string, serviceNames []string,
		projectName string, services map[string]config.Service,
		ws *interfaces.Workspace,
	) (map[string]*interfaces.ServiceInfo, error)
	GetNetworkProjectsFunc              func(networkName string, baseDir string) ([]string, error)
	GetVolumeProjectsFunc               func(volumeName string, baseDir string) ([]string, error)
	ExtractNamedVolumesFunc             func(volumes []string) ([]string, error)
	FormatStatusTableFunc               func(services map[string]*interfaces.ServiceInfo, jsonOutput bool) error
	GetAvailableServicesWithContextFunc func(ctx context.Context, composePath string) ([]string, error)
	ViewLogsWithContextFunc             func(ctx context.Context, composePath string, opts interfaces.LogsOptions) error
	CleanProjectWithContextFunc         func(ctx context.Context, composePath string, dryRun bool) ([]string, error)
	CleanAllProjectsWithContextFunc     func(ctx context.Context, baseDir string, dryRun bool) ([]string, error)
	CleanUnusedImagesWithContextFunc    func(ctx context.Context, dryRun bool) ([]string, error)
	CleanUnusedVolumesWithContextFunc   func(ctx context.Context, dryRun bool, force bool) ([]string, error)
	CleanUnusedNetworksWithContextFunc  func(ctx context.Context, dryRun bool) ([]string, error)
	GetAllActivePortsFunc               func(baseDir string) ([]interfaces.PortInfo, error)
	GenerateComposeFunc                 func(
		deps *config.Deps, ws *interfaces.Workspace, projectDir string,
	) (string, []string, error)
	UpServicesWithContextFunc            func(ctx context.Context, composePath string, serviceNames []string) error
	RestartServicesWithContextFunc       func(ctx context.Context, composePath string, serviceNames []string) error
	ForceRecreateServicesWithContextFunc func(ctx context.Context, composePath string, serviceNames []string) error
	ExecInServiceFunc                    func(
		ctx context.Context, composePath string,
		serviceName string, command []string, interactive bool,
	) error
	WaitForServicesHealthyFunc func(
		ctx context.Context, composePath string,
		serviceNames []string, infraNames []string, projectName string,
	) error
	ValidatePortsFunc func(
		deps *config.Deps, baseDir string, projectName string,
	) ([]interfaces.PortConflict, error)
	FormatPortConflictsFunc               func(conflicts []interfaces.PortConflict) string
	ValidateAllImagesFunc                 func(deps *config.Deps) error
	EnsureNetworkWithConfigAndContextFunc func(ctx context.Context, name string, subnet string, askConfirmation bool) error
	EnsureVolumeWithContextFunc           func(ctx context.Context, name string) error
	NormalizeVolumeNameFunc               func(prefix string, name string) (string, error)
	NormalizeContainerNameFunc            func(
		workspace string, service string, project string,
		hasExplicitWorkspace bool,
	) (string, error)
	NormalizeInfraNameFunc func(
		workspace string, infra string, project string,
		hasExplicitWorkspace bool,
	) (string, error)
	GetContainerNameWithContextFunc func(ctx context.Context, composePath string, serviceName string) (string, error)
	GetContainerStatusByNameFunc    func(ctx context.Context, containerName string) (string, error)
	ResolveRelativeVolumesFunc      func(volumes []string, projectDir string) ([]string, error)
	AreServicesRunningFunc          func(composePath string, serviceNames []string) (bool, error)
	IsNetworkInUseWithContextFunc   func(ctx context.Context, networkName string) (bool, error)
	StopContainerWithContextFunc    func(ctx context.Context, containerName string) error
	BuildServiceVolumesMapFunc      func(deps *config.Deps) (map[string]interfaces.ServiceVolumes, error)
	DetectSharedVolumesFunc         func(services map[string]interfaces.ServiceVolumes) map[string][]string
	FormatSharedVolumesWarningFunc  func(sharedVolumes map[string][]string) string
	RemoveVolumeWithContextFunc     func(ctx context.Context, name string) error
}

func (m *MockDockerRunner) Up(composePath string) error {
	if m.UpFunc != nil {
		return m.UpFunc(composePath)
	}
	return nil
}

func (m *MockDockerRunner) UpWithContext(ctx context.Context, composePath string) error {
	if m.UpWithContextFunc != nil {
		return m.UpWithContextFunc(ctx, composePath)
	}
	return nil
}

func (m *MockDockerRunner) Down(composePath string) error {
	if m.DownFunc != nil {
		return m.DownFunc(composePath)
	}
	return nil
}

func (m *MockDockerRunner) DownWithContext(ctx context.Context, composePath string) error {
	if m.DownWithContextFunc != nil {
		return m.DownWithContextFunc(ctx, composePath)
	}
	return nil
}

func (m *MockDockerRunner) StopServiceWithContext(ctx context.Context, composePath string, serviceName string) error {
	if m.StopServiceWithContextFunc != nil {
		return m.StopServiceWithContextFunc(ctx, composePath, serviceName)
	}
	return nil
}

func (m *MockDockerRunner) GetServicesStatus(composePath string) (map[string]string, error) {
	if m.GetServicesStatusFunc != nil {
		return m.GetServicesStatusFunc(composePath)
	}
	return nil, nil
}

func (m *MockDockerRunner) GetServicesStatusWithContext(
	ctx context.Context, composePath string,
) (map[string]string, error) {
	if m.GetServicesStatusWithContextFunc != nil {
		return m.GetServicesStatusWithContextFunc(ctx, composePath)
	}
	return nil, nil
}

func (m *MockDockerRunner) GetServicesInfoWithContext(
	ctx context.Context, composePath string, serviceNames []string,
	projectName string, services map[string]config.Service,
	ws *interfaces.Workspace,
) (map[string]*interfaces.ServiceInfo, error) {
	if m.GetServicesInfoWithContextFunc != nil {
		return m.GetServicesInfoWithContextFunc(ctx, composePath, serviceNames, projectName, services, ws)
	}
	return nil, nil
}

func (m *MockDockerRunner) GetNetworkProjects(networkName string, baseDir string) ([]string, error) {
	if m.GetNetworkProjectsFunc != nil {
		return m.GetNetworkProjectsFunc(networkName, baseDir)
	}
	return nil, nil
}

func (m *MockDockerRunner) GetVolumeProjects(volumeName string, baseDir string) ([]string, error) {
	if m.GetVolumeProjectsFunc != nil {
		return m.GetVolumeProjectsFunc(volumeName, baseDir)
	}
	return nil, nil
}

func (m *MockDockerRunner) ExtractNamedVolumes(volumes []string) ([]string, error) {
	if m.ExtractNamedVolumesFunc != nil {
		return m.ExtractNamedVolumesFunc(volumes)
	}
	return nil, nil
}

func (m *MockDockerRunner) FormatStatusTable(services map[string]*interfaces.ServiceInfo, jsonOutput bool) error {
	if m.FormatStatusTableFunc != nil {
		return m.FormatStatusTableFunc(services, jsonOutput)
	}
	return nil
}

func (m *MockDockerRunner) GetAvailableServicesWithContext(ctx context.Context, composePath string) ([]string, error) {
	if m.GetAvailableServicesWithContextFunc != nil {
		return m.GetAvailableServicesWithContextFunc(ctx, composePath)
	}
	return nil, nil
}

func (m *MockDockerRunner) ViewLogsWithContext(
	ctx context.Context, composePath string, opts interfaces.LogsOptions,
) error {
	if m.ViewLogsWithContextFunc != nil {
		return m.ViewLogsWithContextFunc(ctx, composePath, opts)
	}
	return nil
}
