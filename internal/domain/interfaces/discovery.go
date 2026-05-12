package interfaces

import (
	models "raioz/internal/domain/models"
)

// ServiceEndpoint represents a reachable service for discovery purposes.
type ServiceEndpoint struct {
	Name    string
	Runtime models.Runtime
	Host    string // container name, "host.docker.internal", or "localhost"
	Port    int    // Port the container/process listens on internally.

	// HostPort is set when raioz published this endpoint to a host port
	// (via `publish:` on a dep, or for a host service that raioz bound to
	// a specific port). Host-side callers use HostPort, container-side
	// callers use Port. Zero means "no host binding" — the endpoint is
	// only reachable from inside the Docker network.
	HostPort int
}

// DiscoveryManager generates service discovery environment variables
// so each service knows how to reach its dependencies.
type DiscoveryManager interface {
	// GenerateEnvVars generates environment variables for a specific service
	// based on its runtime and the runtimes of its dependencies.
	// Returns a map of VAR_NAME=value for the given service.
	GenerateEnvVars(
		serviceName string,
		serviceRuntime models.Runtime,
		endpoints map[string]ServiceEndpoint,
		proxyEnabled bool,
	) map[string]string
}
