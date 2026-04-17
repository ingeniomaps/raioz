// Package proxy manages the Caddy reverse proxy that provides unified HTTPS access
// and DNS resolution for all services, regardless of their runtime (Docker, host, etc.).
package proxy

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"raioz/internal/domain/interfaces"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/runtime"
)

// Manager implements interfaces.ProxyManager using Caddy as a Docker container.
type Manager struct {
	routes      map[string]interfaces.ProxyRoute
	networkName string
	projectName string // used for per-project container/volume naming
	domain      string // default: "localhost"
	certsDir    string // path to mkcert certificates
	tlsMode     string // "mkcert" (default/local) | "letsencrypt" (server)
	bindHost    string // "" = 127.0.0.1 (local), "0.0.0.0" = accessible from network

	// workspaceName, when non-empty, switches the proxy to workspace-shared
	// mode. A single Caddy container per workspace fronts every project
	// instead of one Caddy per project. The name is the user-declared
	// `workspace:` from raioz.yaml.
	workspaceName string

	// networkSubnet is the CIDR of the Docker network raioz owns. Used to
	// compute the default proxy IP (<base>.1.1) and to validate any
	// user-declared proxy.ip. Empty means "no subnet set, let Docker
	// auto-assign on --network attach".
	networkSubnet string
	// containerIP is the explicit IP the proxy should bind inside the
	// Docker network. Populated from raioz.yaml `proxy.ip:` or derived
	// from `networkSubnet` via DefaultProxyIP.
	containerIP string

	// publish is whether the proxy should bind host ports 80/443. Default
	// true; false skips the docker run -p flags so the proxy is only
	// reachable via its container IP. Used for multi-workspace
	// parallelism — each workspace's proxy lives entirely on its own
	// subnet without fighting over the host port pool.
	publish bool
}

// isWorkspaceShared reports whether the proxy is in shared (workspace-scoped)
// mode. Other helpers branch on this to pick workspace-scoped names, labels,
// and lifecycle behavior.
func (m *Manager) isWorkspaceShared() bool {
	return m.workspaceName != ""
}

// containerName picks the right container name based on the mode:
// shared (workspace) or per-project (legacy).
func (m *Manager) containerName() string {
	if m.isWorkspaceShared() {
		return naming.WorkspaceProxyContainer()
	}
	return ContainerName(m.projectName)
}

// caddyVolume returns the volume name to mount at /data inside Caddy.
func (m *Manager) caddyVolume() string {
	if m.isWorkspaceShared() {
		return naming.WorkspaceCaddyVolume()
	}
	return naming.CaddyVolume(m.projectName)
}

// NewManager creates a new proxy Manager.
func NewManager(certsDir string) *Manager {
	return &Manager{
		routes:   make(map[string]interfaces.ProxyRoute),
		domain:   "localhost",
		certsDir: certsDir,
		tlsMode:  "mkcert",
		publish:  true, // default: bind host 80/443
	}
}

// ContainerIP exposes the resolved IP for callers (e.g., the orchestrator's
// "add to /etc/hosts" hint). Empty when no IP is pinned.
func (m *Manager) ContainerIP() string {
	ip, _ := m.resolveContainerIP()
	return ip
}

// HostsLine renders an /etc/hosts entry that maps every route the manager
// knows about to the proxy's container IP. Returns "" when no IP is
// resolvable or when there are no routes — both signal "nothing useful to
// print" rather than an error condition.
func (m *Manager) HostsLine() string {
	ip := m.ContainerIP()
	if ip == "" || len(m.routes) == 0 {
		return ""
	}
	hosts := make([]string, 0, len(m.routes))
	for _, route := range m.routes {
		hosts = append(hosts, route.Hostname+"."+m.domain)
	}
	sort.Strings(hosts) // stable output for diffs / docs
	return ip + "  " + strings.Join(hosts, " ")
}

// resolveContainerIP picks the IP the proxy should bind to, applying the
// precedence rules: explicit > derived-from-subnet > none (auto-assign).
// An invalid user IP is rejected with a descriptive error so the problem
// surfaces before docker run.
func (m *Manager) resolveContainerIP() (string, error) {
	if m.containerIP != "" {
		if err := ValidateProxyIP(m.containerIP, m.networkSubnet); err != nil {
			return "", err
		}
		return m.containerIP, nil
	}
	return DefaultProxyIP(m.networkSubnet), nil
}

// ContainerName returns the proxy container name.
func ContainerName(project string) string {
	return naming.ProxyContainer(project)
}

// Start starts the Caddy proxy container on the given network.
func (m *Manager) Start(ctx context.Context, networkName string) error {
	m.networkName = networkName
	containerName := m.containerName()

	// Ensure mkcert certificates exist before starting
	if m.tlsMode == "mkcert" {
		certsDir, err := EnsureCerts(m.domain)
		if err != nil {
			logging.WarnWithContext(ctx, "Failed to generate mkcert certificates", "error", err.Error())
		} else if certsDir != "" {
			m.certsDir = certsDir
		}
	}

	// Check if already running
	running, _ := m.isRunning(ctx, containerName)
	if running {
		logging.InfoWithContext(ctx, "Proxy already running", "container", containerName)
		return m.Reload(ctx)
	}

	// Remove any stale container left over from a prior failed run. Docker
	// keeps containers that failed to start in `Created` / `Exited` state,
	// and attempting `docker run --name <same>` on top of that returns
	// "container name already in use" — in the old code that error bubbled
	// up as a warning and `up` continued with a permanently broken proxy.
	if err := m.removeStaleContainer(ctx, containerName); err != nil {
		logging.WarnWithContext(ctx, "Failed to remove stale proxy container",
			"name", containerName, "error", err.Error())
	}

	// Pre-flight: fail fast if the host ports the proxy wants are already
	// taken (another web server, a sibling raioz project, etc.). Without
	// this, the `docker run` below returns a cryptic "port is already
	// allocated" and the proxy ends up stuck in Created forever.
	//
	// Skipped entirely when publish is off — we're not binding the host
	// ports at all in that mode.
	if m.publish {
		if err := m.checkPortsAvailable(); err != nil {
			return err
		}
	}

	// Validate that the required IP is resolvable when publish is off. The
	// proxy is only reachable via its container IP in that mode, so the
	// user MUST be able to predict that IP — otherwise they can't write
	// the /etc/hosts entry that makes URLs work.
	if !m.publish {
		ip, err := m.resolveContainerIP()
		if err != nil {
			return err
		}
		if ip == "" {
			return fmt.Errorf(
				"proxy.publish: false requires a deterministic IP — declare " +
					"network.subnet (raioz derives <subnet>.1.1) or proxy.ip explicitly")
		}
	}

	// Generate Caddyfile
	caddyfilePath, err := m.generateCaddyfile()
	if err != nil {
		return fmt.Errorf("failed to generate Caddyfile: %w", err)
	}

	// Build docker run args. When publish is off we omit the -p flags
	// entirely — Caddy still listens on 80/443 inside the container, and
	// callers reach it via the container's network IP.
	args := []string{"run", "-d",
		"--name", containerName,
		"--network", networkName,
		"--restart", "unless-stopped",
		"-v", caddyfilePath + ":/etc/caddy/Caddyfile:ro",
		"-v", m.caddyVolume() + ":/data",
		"--add-host=host.docker.internal:host-gateway",
	}
	if m.publish {
		httpBind := "80:80"
		httpsBind := "443:443"
		if m.bindHost != "" {
			httpBind = m.bindHost + ":80:80"
			httpsBind = m.bindHost + ":443:443"
		}
		args = append(args, "-p", httpBind, "-p", httpsBind)
	}

	// Pin the proxy to a known IP inside the network when one is resolvable.
	// Either the user declared `proxy.ip:` in raioz.yaml, or we derived
	// <subnet>.1.1 from `network.subnet:`. Without an IP arg Docker
	// auto-assigns from its pool — works fine but the address is not
	// stable across restarts, which breaks scripts and /etc/hosts entries
	// that target a specific IP.
	proxyIP, err := m.resolveContainerIP()
	if err != nil {
		return err
	}
	if proxyIP != "" {
		args = append(args, "--ip", proxyIP)
		logging.InfoWithContext(ctx, "Pinning proxy IP",
			"ip", proxyIP, "subnet", m.networkSubnet)
	}

	// Stamp raioz labels so down flows can sweep the proxy by label filter
	// instead of matching container names against brittle prefixes.
	// Workspace-shared proxies omit com.raioz.project (project=""), the same
	// signal shared deps use to indicate "no single project owns this; only
	// kill it when the workspace is empty".
	labelProject := m.projectName
	if m.isWorkspaceShared() {
		labelProject = ""
	}
	for k, v := range naming.Labels(
		m.workspaceName, labelProject, "proxy", naming.KindProxy,
	) {
		args = append(args, "--label", k+"="+v)
	}

	// Mount certs if using mkcert (local mode)
	if m.certsDir != "" && m.tlsMode == "mkcert" {
		args = append(args, "-v", m.certsDir+":/certs:ro")
	}

	// Add network aliases for all routes so containers can resolve *.localhost.
	// Each alias in route.Aliases needs its own --network-alias so
	// container→container DNS works for every hostname, not just the
	// primary.
	for _, route := range m.routes {
		args = append(args, "--network-alias", route.Hostname+"."+m.domain)
		for _, alias := range route.Aliases {
			if alias == "" {
				continue
			}
			args = append(args, "--network-alias", alias+"."+m.domain)
		}
	}

	args = append(args, "caddy:latest")

	logging.InfoWithContext(ctx, "Starting proxy", "container", containerName, "network", networkName)

	cmd := exec.CommandContext(ctx, runtime.Binary(), args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start proxy: %w\n%s", err, string(output))
	}

	return nil
}

// Stop stops and removes the proxy container.
func (m *Manager) Stop(ctx context.Context) error {
	containerName := m.containerName()

	// Best-effort: container may already be stopped or removed.
	stop := exec.CommandContext(ctx, runtime.Binary(), "stop", containerName)
	_ = stop.Run()

	rm := exec.CommandContext(ctx, runtime.Binary(), "rm", "-f", containerName)
	_ = rm.Run()

	return nil
}

// AddRoute adds or updates a proxy route for a service.
func (m *Manager) AddRoute(_ context.Context, route interfaces.ProxyRoute) error {
	m.routes[route.ServiceName] = route
	return nil
}

// RemoveRoute removes a proxy route.
func (m *Manager) RemoveRoute(_ context.Context, serviceName string) error {
	delete(m.routes, serviceName)
	return nil
}

// GetURL returns the HTTPS URL for a service.
func (m *Manager) GetURL(serviceName string) string {
	route, ok := m.routes[serviceName]
	if !ok {
		return ""
	}
	return "https://" + route.Hostname + "." + m.domain
}

// Reload regenerates the Caddyfile and reloads Caddy without downtime.
//
// The Caddyfile is bind-mounted from the host into the container at
// /etc/caddy/Caddyfile, so writing on the host (which generateCaddyfile
// already did) propagates instantly inside. We only need to ask Caddy to
// re-read it via the admin API. A historical `docker cp` here used to fail
// with "device or resource busy" because the bind mount target is
// read-only on the container side — keep the path simple.
func (m *Manager) Reload(ctx context.Context) error {
	if _, err := m.generateCaddyfile(); err != nil {
		return err
	}
	reload := exec.CommandContext(
		ctx, runtime.Binary(), "exec", m.containerName(),
		"caddy", "reload", "--config", "/etc/caddy/Caddyfile",
	)
	if output, err := reload.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to reload proxy: %w\n%s", err, string(output))
	}
	logging.InfoWithContext(ctx, "Proxy reloaded", "routes", len(m.routes))
	return nil
}

// Status returns whether the proxy is running.
func (m *Manager) Status(ctx context.Context) (bool, error) {
	return m.isRunning(ctx, m.containerName())
}

func (m *Manager) isRunning(ctx context.Context, containerName string) (bool, error) {
	cmd := exec.CommandContext(ctx, runtime.Binary(), "inspect",
		"--format", "{{.State.Status}}", containerName)
	output, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(string(output)) == "running", nil
}

// removeStaleContainer wipes any pre-existing container with the proxy name
// that is NOT in the running state. Covers Created (docker run failed mid-way)
// and Exited (previous run crashed) — both of which block `docker run --name`
// from succeeding until the stale entry is removed.
func (m *Manager) removeStaleContainer(ctx context.Context, containerName string) error {
	cmd := exec.CommandContext(ctx, runtime.Binary(), "inspect",
		"--format", "{{.State.Status}}", containerName)
	out, err := cmd.Output()
	if err != nil {
		return nil // container does not exist, nothing to remove
	}
	state := strings.TrimSpace(string(out))
	if state == "" || state == "running" {
		return nil
	}
	logging.InfoWithContext(ctx, "Removing stale proxy container",
		"container", containerName, "state", state)
	rm := exec.CommandContext(ctx, runtime.Binary(), "rm", "-f", containerName)
	if rmOut, rmErr := rm.CombinedOutput(); rmErr != nil {
		return fmt.Errorf("docker rm %s: %w\n%s", containerName, rmErr, string(rmOut))
	}
	return nil
}

// shouldn't require free 80/443 on the test host.
