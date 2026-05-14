package interfaces

import "context"

// ContainerManager covers the lower-level container operations that
// don't go through `docker compose`. Status probes, stop, look-up by
// name or labels.
//
// ADR-012: this is one of six segregated interfaces that `DockerRunner`
// composes. New callers should reference the smallest interface that
// covers what they actually use; this surfaces real dependencies and
// keeps mocks focused.
type ContainerManager interface {
	// GetContainerStatusByName returns the Docker state of a container
	// (running, exited, created, ...) looked up by name. Empty string +
	// nil error means the container does not exist.
	GetContainerStatusByName(ctx context.Context, containerName string) (string, error)

	// FindManagedContainerByService returns the actual container name
	// for a raioz-managed service/dep by matching the com.raioz.project
	// + com.raioz.service labels, or "" if none exists. Fallback when
	// the canonical name doesn't match — typically because the user's
	// compose file dictated a custom container_name. See.
	FindManagedContainerByService(ctx context.Context, project, service string) string

	// StopContainerWithContext stops a container by name.
	StopContainerWithContext(ctx context.Context, containerName string) error

	// IsProjectActive reports whether (workspace, project) has at least
	// one running raioz-managed container. Used by inspection commands
	// (status, logs, exec, restart) to gate work without consulting the
	// legacy state snapshot — see ADR-011.
	IsProjectActive(ctx context.Context, workspace, project string) (bool, error)
}
