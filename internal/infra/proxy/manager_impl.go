// Package proxy is a thin wrapper around internal/proxy so the wiring
// layer (internal/cli/wiring.go, ADR-018) builds every
// port adapter from a single uniform location under internal/infra/.
//
// The wrapper exists for layering, not for behavior — every call
// passes through to internal/proxy unchanged.
package proxy

import (
	"raioz/internal/domain/interfaces"
	"raioz/internal/proxy"
)

// NewManager returns the ProxyManager port implementation backed by
// the default mkcert certificates directory.
func NewManager() interfaces.ProxyManager {
	return proxy.NewManager(proxy.CertsDir())
}

// NewManagerWithCertsDir lets the caller pin a different certificates
// directory; useful for tests and rare CLI overrides.
func NewManagerWithCertsDir(certsDir string) interfaces.ProxyManager {
	return proxy.NewManager(certsDir)
}

// WriteRemoteRoutes is the wiring-layer hook that the meta runner uses
// to materialize a workspace Caddy route for a remote-mode sub-project
// (ADR-049). Lives here so the app layer can stay free of an
// internal/proxy import (ADR-029) — call sites under internal/cli
// import this adapter instead.
func WriteRemoteRoutes(
	workspace, project, domain, tlsMode string,
	routes []interfaces.ProxyRoute,
) error {
	return proxy.WriteRemoteProjectRoutes(workspace, project, domain, tlsMode, routes)
}
