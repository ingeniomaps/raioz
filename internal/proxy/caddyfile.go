package proxy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"raioz/internal/domain/interfaces"
	"raioz/internal/naming"
)

// tlsConfig holds TLS generation parameters.
type tlsConfig struct {
	mode     string // "mkcert", "letsencrypt", or ""
	certsDir string
	domain   string
}

// generateCaddyfile creates a Caddyfile from the current routes and writes it to a temp file.
//
// In workspace-shared mode the Caddyfile is the union of every project's
// persisted route file (see SaveProjectRoutes / loadAllProjectRoutes), so
// project A keeps HTTPS routing while project B independently `up`s its own
// routes. In per-project mode it falls back to the manager's in-memory
// route map — preserving the legacy behavior for projects without a
// workspace declaration.
func (m *Manager) generateCaddyfile() (string, error) {
	var b strings.Builder

	// Pick the global TLS mode. In shared mode the union of projects may
	// contain mixed TLS modes; we err on the safe side and emit
	// `auto_https off` if ANY contributor is using mkcert (otherwise Caddy
	// would try ACME for those routes and hang on custom domains without
	// public DNS — see BUG-12).
	globalTLS := m.tlsMode
	if m.isWorkspaceShared() {
		for _, pp := range m.loadAllProjectRoutes() {
			if pp.TLSMode == "mkcert" {
				globalTLS = "mkcert"
				break
			}
		}
	}

	b.WriteString("{\n")
	switch globalTLS {
	case "mkcert":
		b.WriteString("\tauto_https off\n")
	case "letsencrypt":
		// Real TLS: let Caddy handle ACME as usual.
	}
	b.WriteString("}\n\n")

	if m.isWorkspaceShared() {
		// Render every project's routes with that project's own domain
		// and tls mode — siblings can run on different subdomains, even
		// different domains, without overwriting each other's site blocks.
		for _, pp := range m.loadAllProjectRoutes() {
			tls := tlsConfig{
				mode:     pp.TLSMode,
				certsDir: m.certsDir,
				domain:   pp.Domain,
			}
			for _, route := range pp.Routes {
				writeRouteBlock(&b, route, pp.Domain, tls)
				b.WriteString("\n")
			}
		}
	} else {
		tls := tlsConfig{mode: m.tlsMode, certsDir: m.certsDir, domain: m.domain}
		for _, route := range m.routes {
			writeRouteBlock(&b, route, m.domain, tls)
			b.WriteString("\n")
		}
	}

	// Workspace-shared mode keeps the Caddyfile in a workspace-scoped
	// directory (/tmp/<workspace>/proxy/) so all projects in the workspace
	// see the same canonical file.
	var dir string
	if m.isWorkspaceShared() {
		dir = naming.WorkspaceProxyDir()
	} else {
		dir = naming.ProxyDir(m.networkName)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create proxy config dir: %w", err)
	}

	path := filepath.Join(dir, "Caddyfile")
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		return "", fmt.Errorf("failed to write Caddyfile: %w", err)
	}

	return path, nil
}

// writeRouteBlock writes a single Caddy site block for a service route.
func writeRouteBlock(b *strings.Builder, route interfaces.ProxyRoute, domain string, tls tlsConfig) {
	hostname := route.Hostname + "." + domain

	switch tls.mode {
	case "letsencrypt":
		// Real domain with automatic Let's Encrypt
		fmt.Fprintf(b, "https://%s {\n", hostname)
		// Caddy handles TLS automatically for real domains
	case "mkcert":
		if tls.certsDir != "" {
			fmt.Fprintf(b, "https://%s {\n", hostname)
			fmt.Fprintf(b, "\ttls /certs/%s /certs/%s\n", certFileName, keyFileName)
		} else {
			fmt.Fprintf(b, "http://%s {\n", hostname)
		}
	default:
		fmt.Fprintf(b, "http://%s {\n", hostname)
	}

	// reverse_proxy directive with appropriate options
	target := route.Target
	if route.Port > 0 && !strings.Contains(target, ":") {
		target = fmt.Sprintf("%s:%d", target, route.Port)
	}

	if route.GRPC {
		fmt.Fprintf(b, "\treverse_proxy h2c://%s\n", target)
	} else if route.Stream {
		fmt.Fprintf(b, "\treverse_proxy %s {\n", target)
		b.WriteString("\t\tflush_interval -1\n")
		b.WriteString("\t}\n")
	} else if route.WebSocket {
		fmt.Fprintf(b, "\treverse_proxy %s {\n", target)
		b.WriteString("\t\theader_up X-Forwarded-Proto {scheme}\n")
		b.WriteString("\t}\n")
	} else {
		fmt.Fprintf(b, "\treverse_proxy %s\n", target)
	}

	b.WriteString("}\n")
}

// GenerateCaddyfileContent returns the Caddyfile content as a string (for testing).
func (m *Manager) GenerateCaddyfileContent() string {
	var b strings.Builder
	tls := tlsConfig{mode: m.tlsMode, certsDir: m.certsDir, domain: m.domain}
	for _, route := range m.routes {
		writeRouteBlock(&b, route, m.domain, tls)
		b.WriteString("\n")
	}
	return b.String()
}
