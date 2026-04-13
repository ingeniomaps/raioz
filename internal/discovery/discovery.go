// Package discovery generates service discovery environment variables so each
// service knows how to reach its dependencies, regardless of runtime.
package discovery

import (
	"fmt"
	"strings"

	"raioz/internal/detect"
	"raioz/internal/domain/interfaces"
)

// Manager implements interfaces.DiscoveryManager.
type Manager struct{}

// NewManager creates a new discovery Manager.
func NewManager() *Manager {
	return &Manager{}
}

// GenerateEnvVars generates environment variables for a specific service
// based on its runtime and the runtimes of its dependencies.
//
// Resolution rules:
//
//	Container → Container (same Docker network): use container name as host
//	Container → Host process: use host.docker.internal
//	Host → Container: use localhost (port is mapped to host)
//	Host → Host: use localhost
//	With proxy enabled: also set _URL=https://<name>.localhost
func (m *Manager) GenerateEnvVars(
	serviceName string,
	serviceRuntime detect.Runtime,
	endpoints map[string]interfaces.ServiceEndpoint,
	proxyEnabled bool,
) map[string]string {
	vars := make(map[string]string)
	isServiceDocker := isDockerRuntime(serviceRuntime)

	for name, ep := range endpoints {
		if name == serviceName {
			continue // skip self
		}

		envPrefix := toEnvPrefix(name)
		targetIsDocker := isDockerRuntime(ep.Runtime)
		host := resolveHost(isServiceDocker, targetIsDocker, ep)

		// Pick the port the caller can actually reach:
		//   docker → docker : container port via DNS name
		//   host   → docker : host port (required to be published)
		//   host   → host   : same port (only one port)
		//   docker → host   : host port (via host.docker.internal)
		port := ep.Port
		if !isServiceDocker && targetIsDocker && ep.HostPort > 0 {
			port = ep.HostPort
		}

		vars[envPrefix+"_HOST"] = host
		if port > 0 {
			vars[envPrefix+"_PORT"] = fmt.Sprintf("%d", port)
			vars[envPrefix+"_URL"] = fmt.Sprintf("http://%s:%d", host, port)
		}

		// With proxy, also provide HTTPS URL
		if proxyEnabled {
			vars[envPrefix+"_HTTPS_URL"] = fmt.Sprintf("https://%s.localhost", name)
		}
	}

	// Raioz metadata
	vars["RAIOZ_SERVICE"] = serviceName
	vars["RAIOZ_RUNTIME"] = string(serviceRuntime)

	return vars
}

// resolveHost determines the correct host for service-to-service communication.
func resolveHost(callerIsDocker, targetIsDocker bool, target interfaces.ServiceEndpoint) string {
	switch {
	case callerIsDocker && targetIsDocker:
		// Both in Docker network — use container name as DNS
		return target.Host
	case callerIsDocker && !targetIsDocker:
		// Caller in Docker, target on host — use host.docker.internal
		return "host.docker.internal"
	case !callerIsDocker && targetIsDocker:
		// Caller on host, target in Docker — use localhost (port mapped to host)
		return "localhost"
	default:
		// Both on host
		return "localhost"
	}
}

// isDockerRuntime returns true for runtimes that run inside Docker containers.
func isDockerRuntime(rt detect.Runtime) bool {
	switch rt {
	case detect.RuntimeCompose, detect.RuntimeDockerfile, detect.RuntimeImage:
		return true
	default:
		return false
	}
}

// toEnvPrefix converts a service name to an environment variable prefix.
// "auth-api" → "AUTH_API", "postgres" → "POSTGRES"
func toEnvPrefix(name string) string {
	s := strings.ToUpper(name)
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, ".", "_")
	return s
}
