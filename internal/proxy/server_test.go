package proxy

import (
	"context"
	"strings"
	"testing"

	"raioz/internal/domain/interfaces"
)

func TestServerMode_LetsEncrypt(t *testing.T) {
	m := NewManager("")
	m.SetDomain("dev.acme.com")
	m.SetTLSMode("letsencrypt")

	m.AddRoute(context.Background(), interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api-container",
		Port:        3000,
	})

	content := m.GenerateCaddyfileContent()

	if !strings.Contains(content, "https://api.dev.acme.com") {
		t.Errorf("expected custom domain, got:\n%s", content)
	}
	// Let's Encrypt: should NOT have manual tls directive (Caddy auto-manages)
	if strings.Contains(content, "tls /certs/") {
		t.Error("letsencrypt mode should not have manual cert paths")
	}
}

func TestServerMode_CustomDomain(t *testing.T) {
	m := NewManager("/certs")
	m.SetDomain("dev.mycompany.io")

	m.AddRoute(context.Background(), interfaces.ProxyRoute{
		ServiceName: "frontend",
		Hostname:    "frontend",
		Target:      "host.docker.internal",
		Port:        3000,
	})

	url := m.GetURL("frontend")
	if url != "https://frontend.dev.mycompany.io" {
		t.Errorf("URL = %q, want 'https://frontend.dev.mycompany.io'", url)
	}
}

func TestServerMode_BindHost(t *testing.T) {
	m := NewManager("")
	m.SetBindHost("0.0.0.0")

	if m.bindHost != "0.0.0.0" {
		t.Errorf("bindHost = %q, want '0.0.0.0'", m.bindHost)
	}
}

func TestLocalMode_DefaultDomain(t *testing.T) {
	m := NewManager("/certs")

	m.AddRoute(context.Background(), interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api:3000",
	})

	content := m.GenerateCaddyfileContent()

	if !strings.Contains(content, "api.localhost") {
		t.Errorf("expected localhost domain, got:\n%s", content)
	}
	if !strings.Contains(content, "tls /certs/cert.pem") {
		t.Errorf("expected mkcert TLS, got:\n%s", content)
	}
}

func TestNoTLS_Mode(t *testing.T) {
	m := NewManager("")
	m.SetTLSMode("")

	m.AddRoute(context.Background(), interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api:3000",
	})

	content := m.GenerateCaddyfileContent()

	if !strings.Contains(content, "http://") {
		t.Errorf("expected http (no TLS), got:\n%s", content)
	}
	if strings.Contains(content, "https://") {
		t.Error("should not have HTTPS without TLS mode")
	}
}
