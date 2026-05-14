package proxy

import "raioz/internal/domain/interfaces"

// Configure applies every field from cfg to the manager in one call.
// ADR-013 Phase 2 / ADR-032: the per-field setters that used to back
// this method are gone. Configure is the only configuration entry
// point now.
//
// Zero-value semantics inherited from the deprecated setters:
//
//   - Empty Domain → keep current (default "localhost").
//   - Zero TLSMode → TLSModeLocal (mkcert under the Caddy adapter).
//   - Empty BindHost → 127.0.0.1.
//   - Empty ProjectName / Workspace / NetworkSubnet / ContainerIP →
//     unchanged from previous Configure.
//   - nil Publish → unchanged (default true).
func (m *Manager) Configure(cfg interfaces.ProxyConfig) {
	if cfg.Domain != "" {
		m.domain = cfg.Domain
	}
	m.tlsMode = caddyTLSValue(cfg.TLSMode)
	m.bindHost = cfg.BindHost
	m.projectName = cfg.ProjectName
	m.workspaceName = cfg.Workspace
	m.networkSubnet = cfg.NetworkSubnet
	m.containerIP = cfg.ContainerIP
	if cfg.Publish != nil {
		m.publish = *cfg.Publish
	}
}

// caddyTLSValue maps the vendor-neutral TLSMode enum onto the Caddy
// adapter's internal string. The Manager still works in Caddy's
// vocabulary because Caddyfile generation and EnsureCerts branch on
// these literal values — a future adapter (Traefik, …) would supply
// its own helper here. ADR-032.
func caddyTLSValue(mode interfaces.TLSMode) string {
	switch mode {
	case interfaces.TLSModeACME:
		return "letsencrypt"
	case interfaces.TLSModeManual:
		return "manual"
	default: // TLSModeLocal or zero → mkcert (the Caddy default)
		return "mkcert"
	}
}

// IsPublished reports whether the proxy will bind host ports. Kept
// public because test fixtures and the down flow consult it; not
// deprecated.
func (m *Manager) IsPublished() bool {
	return m.publish
}
