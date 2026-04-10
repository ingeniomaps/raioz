// Package proxy manages the Caddy reverse proxy that provides unified HTTPS access
// and DNS resolution for all services, regardless of their runtime (Docker, host, etc.).
package proxy

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"raioz/internal/domain/interfaces"
	"raioz/internal/logging"
	"raioz/internal/naming"
)

// Manager implements interfaces.ProxyManager using Caddy as a Docker container.
type Manager struct {
	routes      map[string]interfaces.ProxyRoute
	networkName string
	domain      string // default: "localhost"
	certsDir    string // path to mkcert certificates
	tlsMode     string // "mkcert" (default/local) | "letsencrypt" (server)
	bindHost    string // "" = 127.0.0.1 (local), "0.0.0.0" = accessible from network
}

// NewManager creates a new proxy Manager.
func NewManager(certsDir string) *Manager {
	return &Manager{
		routes:   make(map[string]interfaces.ProxyRoute),
		domain:   "localhost",
		certsDir: certsDir,
		tlsMode:  "mkcert",
	}
}

// SetDomain sets a custom domain (e.g., "dev.acme.com" instead of "localhost").
func (m *Manager) SetDomain(domain string) {
	if domain != "" {
		m.domain = domain
	}
}

// SetTLSMode sets the TLS mode: "mkcert" for local certs, "letsencrypt" for real certs.
func (m *Manager) SetTLSMode(mode string) {
	if mode != "" {
		m.tlsMode = mode
	}
}

// SetBindHost sets the bind address. Use "0.0.0.0" for shared dev servers.
func (m *Manager) SetBindHost(host string) {
	m.bindHost = host
}

// ContainerName returns the proxy container name for a workspace.
func ContainerName(workspace string) string {
	return naming.ProxyContainer(workspace)
}

// Start starts the Caddy proxy container on the given network.
func (m *Manager) Start(ctx context.Context, networkName string) error {
	m.networkName = networkName
	containerName := ContainerName(networkName)

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

	// Generate Caddyfile
	caddyfilePath, err := m.generateCaddyfile()
	if err != nil {
		return fmt.Errorf("failed to generate Caddyfile: %w", err)
	}

	// Build docker run args
	httpBind := "80:80"
	httpsBind := "443:443"
	if m.bindHost != "" {
		httpBind = m.bindHost + ":80:80"
		httpsBind = m.bindHost + ":443:443"
	}

	args := []string{"run", "-d",
		"--name", containerName,
		"--network", networkName,
		"--restart", "unless-stopped",
		"-p", httpBind,
		"-p", httpsBind,
		"-v", caddyfilePath + ":/etc/caddy/Caddyfile:ro",
		"-v", naming.CaddyVolume(m.networkName)+":/data",
		"--add-host=host.docker.internal:host-gateway",
	}

	// Mount certs if using mkcert (local mode)
	if m.certsDir != "" && m.tlsMode == "mkcert" {
		args = append(args, "-v", m.certsDir+":/certs:ro")
	}

	// Add network aliases for all routes so containers can resolve *.localhost
	for _, route := range m.routes {
		hostname := route.Hostname + "." + m.domain
		args = append(args, "--network-alias", hostname)
	}

	args = append(args, "caddy:latest")

	logging.InfoWithContext(ctx, "Starting proxy", "container", containerName, "network", networkName)

	cmd := exec.CommandContext(ctx, "docker", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start proxy: %w\n%s", err, string(output))
	}

	return nil
}

// Stop stops and removes the proxy container.
func (m *Manager) Stop(ctx context.Context) error {
	containerName := ContainerName(m.networkName)

	stop := exec.CommandContext(ctx, "docker", "stop", containerName)
	stop.Run()

	rm := exec.CommandContext(ctx, "docker", "rm", "-f", containerName)
	rm.Run()

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
func (m *Manager) Reload(ctx context.Context) error {
	caddyfilePath, err := m.generateCaddyfile()
	if err != nil {
		return err
	}

	containerName := ContainerName(m.networkName)

	// Copy new Caddyfile into the container
	cp := exec.CommandContext(ctx, "docker", "cp", caddyfilePath, containerName+":/etc/caddy/Caddyfile")
	if output, err := cp.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to copy Caddyfile: %w\n%s", err, string(output))
	}

	// Reload Caddy
	reload := exec.CommandContext(ctx, "docker", "exec", containerName, "caddy", "reload", "--config", "/etc/caddy/Caddyfile")
	if output, err := reload.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to reload proxy: %w\n%s", err, string(output))
	}

	logging.InfoWithContext(ctx, "Proxy reloaded", "routes", len(m.routes))
	return nil
}

// Status returns whether the proxy is running.
func (m *Manager) Status(ctx context.Context) (bool, error) {
	containerName := ContainerName(m.networkName)
	return m.isRunning(ctx, containerName)
}

func (m *Manager) isRunning(ctx context.Context, containerName string) (bool, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect",
		"--format", "{{.State.Status}}", containerName)
	output, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(string(output)) == "running", nil
}
