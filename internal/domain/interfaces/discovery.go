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

	// ContainerOnly marks a Port that exists only inside the Docker
	// network: a dependency with no host binding. A caller in a container
	// gets <NAME>_PORT / <NAME>_URL from it; a caller on the host gets
	// neither, because localhost:<Port> is not this endpoint.
	ContainerOnly bool

	// Scheme is the URL scheme a caller uses to build <DEP>_URL (e.g.
	// "redis", "postgresql", "http"). Empty is treated as "http". Set
	// from the dependency image so non-HTTP datastores get a scheme their
	// client can actually parse instead of a useless http:// URL.
	Scheme string

	// ProxyURL is the HTTPS address the workspace proxy serves this
	// endpoint on — its `hostname:` (or name) under the proxy's domain.
	// Empty means the caller did not work it out; <NAME>_HTTPS_URL then
	// falls back to https://<name>.localhost.
	ProxyURL string

	// Unrouted marks an endpoint the proxy has no route for (a database
	// image, a service with `proxy: false`). No <NAME>_HTTPS_URL is
	// emitted for it: the address would resolve to nothing.
	Unrouted bool
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
