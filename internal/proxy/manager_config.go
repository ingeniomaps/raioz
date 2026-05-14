package proxy

import "raioz/internal/domain/interfaces"

// Configure applies every field from cfg to the manager in one call.
// ADR-013's canonical replacement for the per-field setters below;
// preferred for all new call sites. The per-field setters remain for
// backward compatibility but are deprecated.
//
// Behavior matches the setters' individual zero-value rules: empty
// strings fall through to defaults, nil Publish keeps the current
// value, *false flips it off.
func (m *Manager) Configure(cfg interfaces.ProxyConfig) {
	m.SetDomain(cfg.Domain)
	m.SetTLSMode(cfg.TLSMode)
	m.SetBindHost(cfg.BindHost)
	m.SetProjectName(cfg.ProjectName)
	m.SetWorkspace(cfg.Workspace)
	m.SetNetworkSubnet(cfg.NetworkSubnet)
	m.SetContainerIP(cfg.ContainerIP)
	m.SetPublish(cfg.Publish)
}

// SetDomain sets a custom domain (e.g., "dev.acme.com" instead of "localhost").
//
// Deprecated: build a Config and call Configure (ADR-013).
func (m *Manager) SetDomain(domain string) {
	if domain != "" {
		m.domain = domain
	}
}

// SetTLSMode sets the TLS mode: "mkcert" for local certs, "letsencrypt" for real certs.
//
// Deprecated: see Configure (ADR-013).
func (m *Manager) SetTLSMode(mode string) {
	if mode != "" {
		m.tlsMode = mode
	}
}

// SetBindHost sets the bind address. Use "0.0.0.0" for shared dev servers.
//
// Deprecated: see Configure (ADR-013).
func (m *Manager) SetBindHost(host string) {
	m.bindHost = host
}

// SetProjectName sets the project name for container/volume naming.
//
// Deprecated: see Configure (ADR-013).
func (m *Manager) SetProjectName(name string) {
	m.projectName = name
}

// SetNetworkSubnet records the CIDR of the Docker network the proxy will
// attach to. Empty when the user didn't declare one.
//
// Deprecated: see Configure (ADR-013).
func (m *Manager) SetNetworkSubnet(cidr string) {
	m.networkSubnet = cidr
}

// SetContainerIP pins the proxy container to a specific address inside the
// Docker network. Empty means "let raioz pick the default (<subnet>.1.1)
// when a subnet is set, otherwise let Docker auto-assign".
//
// Deprecated: see Configure (ADR-013).
func (m *Manager) SetContainerIP(ip string) {
	m.containerIP = ip
}

// SetWorkspace switches the manager to workspace-shared mode. Empty
// switches it back to per-project mode.
//
// Deprecated: see Configure (ADR-013).
func (m *Manager) SetWorkspace(name string) {
	m.workspaceName = name
}

// SetPublish toggles the host port binding. nil/true keeps the default;
// false skips the binding entirely.
//
// Deprecated: see Configure (ADR-013).
func (m *Manager) SetPublish(publish *bool) {
	if publish != nil {
		m.publish = *publish
	}
}

// IsPublished reports whether the proxy will bind host ports.
func (m *Manager) IsPublished() bool {
	return m.publish
}
