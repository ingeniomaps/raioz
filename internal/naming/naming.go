// Package naming centralizes all resource name generation for raioz.
// Every Docker resource (container, network, volume), file path, and
// temp directory should use these functions for consistency.
//
// The prefix defaults to "raioz" but can be overridden per-project
// via SetPrefix (typically set to the workspace name from raioz.yaml).
package naming

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultPrefix is used when no workspace is configured.
const DefaultPrefix = "raioz"

// prefix is the active prefix for Docker resources.
// Set via SetPrefix(), defaults to DefaultPrefix.
var prefix = DefaultPrefix

// SetPrefix sets the prefix for all Docker resource names.
// Call this with the workspace name (if set) during config loading.
// An empty string resets to the default.
func SetPrefix(p string) {
	if p == "" {
		prefix = DefaultPrefix
		return
	}
	prefix = p
}

// GetPrefix returns the current active prefix.
func GetPrefix() string {
	return prefix
}

// --- Docker resources ---

// Container returns the container name for a service.
// Format: {prefix}-{project}-{service}
func Container(project, service string) string {
	return fmt.Sprintf("%s-%s-%s", prefix, project, service)
}

// Network returns the network name for a project.
// Format: {project}-net (or {workspace}-net if workspace is set)
func Network(projectOrWorkspace string) string {
	return projectOrWorkspace + "-net"
}

// ProxyContainer returns the proxy container name.
// Format: {prefix}-proxy-{project}
func ProxyContainer(project string) string {
	return fmt.Sprintf("%s-proxy-%s", prefix, project)
}

// CaddyVolume returns the Caddy data volume name.
// Format: {prefix}-caddy-{project}
func CaddyVolume(project string) string {
	return fmt.Sprintf("%s-caddy-%s", prefix, project)
}

// --- Temp directories (project-isolated) ---

// TempDir returns the base temp directory for a project.
// Format: /tmp/{prefix}-{project}/
func TempDir(project string) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("%s-%s", prefix, project))
}

// LogDir returns the log directory for a project.
func LogDir(project string) string {
	return filepath.Join(TempDir(project), "logs")
}

// LogFile returns the log file path for a host service.
func LogFile(project, service string) string {
	return filepath.Join(LogDir(project), service+".log")
}

// DepsDir returns the dependency compose directory for a project.
func DepsDir(project string) string {
	return filepath.Join(TempDir(project), "deps")
}

// DepComposePath returns the compose file path for a dependency.
func DepComposePath(project, dep string) string {
	return filepath.Join(DepsDir(project), dep, "docker-compose.yml")
}

// ProxyDir returns the proxy config directory for a project.
func ProxyDir(project string) string {
	return filepath.Join(TempDir(project), "proxy")
}

// CaddyfilePath returns the Caddyfile path for a project.
func CaddyfilePath(project string) string {
	return filepath.Join(ProxyDir(project), "Caddyfile")
}

// ContainerPrefix returns the prefix used for listing/filtering containers.
// Format: {prefix}-{project}-
func ContainerPrefix(project string) string {
	return fmt.Sprintf("%s-%s-", prefix, project)
}
