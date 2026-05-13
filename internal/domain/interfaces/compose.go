package interfaces

import (
	"context"

	"raioz/internal/domain/models"
)

// ComposeRunner covers all docker-compose-shaped operations: bring
// up/down a project, list services, watch logs, exec, generate the
// compose file from a Deps.
//
// ADR-012: one of six segregated interfaces composed by DockerRunner.
type ComposeRunner interface {
	Up(composePath string) error
	UpWithContext(ctx context.Context, composePath string) error
	Down(composePath string) error
	DownWithContext(ctx context.Context, composePath string) error
	StopServiceWithContext(ctx context.Context, composePath, serviceName string) error

	GetServicesStatus(composePath string) (map[string]string, error)
	GetServicesStatusWithContext(ctx context.Context, composePath string) (map[string]string, error)
	GetServicesInfoWithContext(
		ctx context.Context, composePath string,
		serviceNames []string, projectName string,
		services map[string]models.Service, ws *Workspace,
	) (map[string]*ServiceInfo, error)
	GetAvailableServicesWithContext(ctx context.Context, composePath string) ([]string, error)

	ViewLogsWithContext(ctx context.Context, composePath string, opts LogsOptions) error

	UpServicesWithContext(ctx context.Context, composePath string, serviceNames []string) error
	RestartServicesWithContext(ctx context.Context, composePath string, serviceNames []string) error
	ForceRecreateServicesWithContext(ctx context.Context, composePath string, serviceNames []string) error

	ExecInService(
		ctx context.Context, composePath, serviceName string,
		command []string, interactive bool,
	) error
	WaitForServicesHealthy(
		ctx context.Context, composePath string,
		serviceNames []string, infraNames []string,
		projectName string,
	) error

	GetContainerNameWithContext(
		ctx context.Context, composePath, serviceName string,
	) (string, error)
	AreServicesRunning(composePath string, serviceNames []string) (bool, error)

	GenerateCompose(
		deps *models.Deps, ws *Workspace, projectDir string,
	) (string, []string, error)

	CleanProjectWithContext(
		ctx context.Context, composePath string, dryRun bool,
	) ([]string, error)
	CleanAllProjectsWithContext(
		ctx context.Context, baseDir string, dryRun bool,
	) ([]string, error)
}
