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
func (m *Manager) generateCaddyfile() (string, error) {
	var b strings.Builder

	tls := tlsConfig{mode: m.tlsMode, certsDir: m.certsDir, domain: m.domain}

	// Global options
	b.WriteString("{\n")
	if tls.mode == "mkcert" && tls.certsDir != "" {
		b.WriteString("\tauto_https disable_redirects\n")
	}
	// Let's Encrypt: Caddy handles TLS automatically, no global override needed
	b.WriteString("}\n\n")

	// Generate a site block per route
	for _, route := range m.routes {
		writeRouteBlock(&b, route, m.domain, tls)
		b.WriteString("\n")
	}

	// Write to temp file
	dir := naming.ProxyDir(m.networkName)
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
