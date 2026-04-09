package interfaces

import (
	"raioz/internal/detect"
)

// ServiceEndpoint represents a reachable service for discovery purposes.
type ServiceEndpoint struct {
	Name    string
	Runtime detect.Runtime
	Host    string // container name, "host.docker.internal", or "localhost"
	Port    int
}

// DiscoveryManager generates service discovery environment variables
// so each service knows how to reach its dependencies.
type DiscoveryManager interface {
	// GenerateEnvVars generates environment variables for a specific service
	// based on its runtime and the runtimes of its dependencies.
	// Returns a map of VAR_NAME=value for the given service.
	GenerateEnvVars(
		serviceName string,
		serviceRuntime detect.Runtime,
		endpoints map[string]ServiceEndpoint,
		proxyEnabled bool,
	) map[string]string
}
