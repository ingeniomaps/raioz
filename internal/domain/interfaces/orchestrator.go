package interfaces

import (
	"context"

	models "raioz/internal/domain/models"
)

// ServiceContext holds all information needed to start/stop a service.
type ServiceContext struct {
	Name          string
	Path          string
	Detection     models.DetectResult
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

	// ProxyTarget forwards `proxy.target:` to runners. HostRunner uses
	// it for the launcher-pattern container wait/drain (ADR-025).
	ProxyTarget string

	// Volumes carries the user-declared `volumes:` list for image-mode
	// dependencies. Strings follow docker-compose syntax (bind:
	// `./host/path:/container/path[:ro]`, named: `myvol:/data`). ImageRunner
	// resolves relative bind paths against ProjectDir and registers any
	// named volumes top-level. Empty for services (their compose/Dockerfile
	// drives volumes natively) and for deps that didn't declare any.
	Volumes []string

	// ProjectDir is the absolute directory of the project's raioz.yaml.
	// Used to resolve relative bind paths in Volumes — without it,
	// `./foo.yml:/etc/foo.yml` would land relative to the raioz process
	// cwd, not the project root. Empty when the runner doesn't need
	// path resolution (services already track their own path).
	ProjectDir string
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
