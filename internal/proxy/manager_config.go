package proxy

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

// SetProjectName sets the project name for container/volume naming.
func (m *Manager) SetProjectName(name string) {
	m.projectName = name
}

// SetNetworkSubnet records the CIDR of the Docker network the proxy will
// attach to. Empty when the user didn't declare one.
func (m *Manager) SetNetworkSubnet(cidr string) {
	m.networkSubnet = cidr
}

// SetContainerIP pins the proxy container to a specific address inside the
// Docker network. Empty means "let raioz pick the default (<subnet>.1.1)
// when a subnet is set, otherwise let Docker auto-assign".
func (m *Manager) SetContainerIP(ip string) {
	m.containerIP = ip
}

// SetWorkspace switches the manager to workspace-shared mode. Empty
// switches it back to per-project mode.
func (m *Manager) SetWorkspace(name string) {
	m.workspaceName = name
}

// SetPublish toggles the host port binding. nil/true keeps the default;
// false skips the binding entirely.
func (m *Manager) SetPublish(publish *bool) {
	if publish != nil {
		m.publish = *publish
	}
}

// IsPublished reports whether the proxy will bind host ports.
func (m *Manager) IsPublished() bool {
	return m.publish
}
