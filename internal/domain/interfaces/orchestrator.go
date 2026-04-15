package interfaces

import (
	"context"
	"raioz/internal/detect"
)

// ServiceContext holds all information needed to start/stop a service.
type ServiceContext struct {
	Name          string
	Path          string
	Detection     detect.DetectResult
	NetworkName   string
	EnvVars       map[string]string
	Ports         []string
	DependsOn     []string
	ContainerName string
	ProjectName   string // Used for project-isolated temp dirs and naming

	// StopCommand is set when raioz.yaml declares `stop:` on the service.
	// Runners that support it (HostRunner) use it instead of killing the PID
	// so commands like `make start` can cleanly tear down their own children.
	StopCommand string

	// ExternalComposeFiles is populated for dependencies declared with
	// `compose:` in raioz.yaml (user-supplied fragments instead of bare
	// `image:`). ImageRunner uses these files as-is and writes an overlay
	// next to them to add the shared network + raioz labels. Empty means
	// the dep is image-based (generated compose).
	ExternalComposeFiles []string

	// EnvFilePaths is the list of .env files the dependency declared. Used
	// as --env-file flags when running docker compose, so ${VAR}
	// interpolation in the user's compose resolves correctly. Populated
	// alongside ExternalComposeFiles.
	EnvFilePaths []string
}

// Orchestrator defines operations for starting and stopping services
// using their native runtime tools (compose, docker, npm, go, etc.).
type Orchestrator interface {
	// Start starts a service using its detected runtime
	Start(ctx context.Context, svc ServiceContext) error
	// Stop stops a running service
	Stop(ctx context.Context, svc ServiceContext) error
	// Restart restarts a running service
	Restart(ctx context.Context, svc ServiceContext) error
	// Status returns the status of a service ("running", "stopped", "unknown")
	Status(ctx context.Context, svc ServiceContext) (string, error)
	// Logs streams or fetches logs for a service
	Logs(ctx context.Context, svc ServiceContext, follow bool, tail int) error
}
