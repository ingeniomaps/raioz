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

// SharedContainer returns the workspace-scoped container name used for
// dependencies that must be shared across every project in the same
// workspace (e.g. a single postgres serving several sibling projects).
// Format: {prefix}-{dep}. The missing project segment is intentional —
// including it would split the dep into one container per consuming
// project, defeating the "dependencies are workspace-shared" mental model.
func SharedContainer(dep string) string {
	return fmt.Sprintf("%s-%s", prefix, dep)
}

// DepContainer resolves the container name for a dependency given an optional
// user-specified override. Precedence:
//
//  1. nameOverride (raioz.yaml `dependencies.<dep>.name`) is used verbatim.
//  2. When a workspace is set, the container is workspace-shared
//     ({workspace}-{dep}) so sibling projects reuse it.
//  3. Without a workspace, fall back to the legacy per-project scheme
//     ({prefix}-{project}-{dep}) so two unrelated projects on the same
//     machine don't fight over a global "raioz-postgres".
func DepContainer(project, dep, nameOverride string) string {
	if nameOverride != "" {
		return nameOverride
	}
	if WorkspaceName() != "" {
		return SharedContainer(dep)
	}
	return Container(project, dep)
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

// WorkspaceProxyContainer returns the workspace-scoped proxy container
// name. Format: {workspace}-proxy. Used when a single Caddy fronts every
// project in the workspace (the new shared model). Callers should only
// reach for this when WorkspaceName() returns non-empty; otherwise prefer
// per-project ProxyContainer.
func WorkspaceProxyContainer() string {
	return fmt.Sprintf("%s-proxy", prefix)
}

// CaddyVolume returns the Caddy data volume name.
// Format: {prefix}-caddy-{project}
func CaddyVolume(project string) string {
	return fmt.Sprintf("%s-caddy-%s", prefix, project)
}

// WorkspaceCaddyVolume returns the workspace-shared Caddy data volume name.
// Format: {workspace}-caddy. Lives alongside WorkspaceProxyContainer so the
// shared proxy keeps its on-disk state (issued certs, ACME accounts, etc.)
// for the whole workspace.
func WorkspaceCaddyVolume() string {
	return fmt.Sprintf("%s-caddy", prefix)
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

// WorkspaceProxyDir returns the workspace-shared proxy config directory.
// Format: /tmp/{workspace}/proxy/. Lives outside any per-project temp tree
// so the directory survives individual project teardowns and is the single
// source of truth for the shared Caddyfile.
func WorkspaceProxyDir() string {
	return filepath.Join(os.TempDir(), prefix, "proxy")
}

// WorkspaceCaddyfilePath returns the path to the shared Caddyfile for the
// current workspace.
func WorkspaceCaddyfilePath() string {
	return filepath.Join(WorkspaceProxyDir(), "Caddyfile")
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
