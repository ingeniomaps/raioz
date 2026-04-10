// Package naming centralizes all resource name generation for raioz.
// Every Docker resource (container, network, volume), file path, and
// temp directory should use these functions for consistency.
package naming

import (
	"fmt"
	"os"
	"path/filepath"
)

// Prefix is the global prefix for all raioz-managed Docker resources.
const Prefix = "raioz"

// --- Docker resources ---

// Container returns the container name for a service.
// Format: raioz-{project}-{service}
func Container(project, service string) string {
	return fmt.Sprintf("%s-%s-%s", Prefix, project, service)
}

// Network returns the network name for a project.
// Format: {project}-net (or {workspace}-net if workspace is set)
func Network(projectOrWorkspace string) string {
	return projectOrWorkspace + "-net"
}

// ProxyContainer returns the proxy container name.
// Format: raioz-proxy-{project}
func ProxyContainer(project string) string {
	return fmt.Sprintf("%s-proxy-%s", Prefix, project)
}

// CaddyVolume returns the Caddy data volume name.
// Format: raioz-caddy-{project}
func CaddyVolume(project string) string {
	return fmt.Sprintf("%s-caddy-%s", Prefix, project)
}

// --- Temp directories (project-isolated) ---

// TempDir returns the base temp directory for a project.
// Format: /tmp/raioz-{project}/
func TempDir(project string) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("%s-%s", Prefix, project))
}

// LogDir returns the log directory for a project.
// Format: /tmp/raioz-{project}/logs/
func LogDir(project string) string {
	return filepath.Join(TempDir(project), "logs")
}

// LogFile returns the log file path for a host service.
// Format: /tmp/raioz-{project}/logs/{service}.log
func LogFile(project, service string) string {
	return filepath.Join(LogDir(project), service+".log")
}

// DepsDir returns the dependency compose directory for a project.
// Format: /tmp/raioz-{project}/deps/
func DepsDir(project string) string {
	return filepath.Join(TempDir(project), "deps")
}

// DepComposePath returns the compose file path for a dependency.
// Format: /tmp/raioz-{project}/deps/{dep}/docker-compose.yml
func DepComposePath(project, dep string) string {
	return filepath.Join(DepsDir(project), dep, "docker-compose.yml")
}

// ProxyDir returns the proxy config directory for a project.
// Format: /tmp/raioz-{project}/proxy/
func ProxyDir(project string) string {
	return filepath.Join(TempDir(project), "proxy")
}

// CaddyfilePath returns the Caddyfile path for a project.
// Format: /tmp/raioz-{project}/proxy/Caddyfile
func CaddyfilePath(project string) string {
	return filepath.Join(ProxyDir(project), "Caddyfile")
}
