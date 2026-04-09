package proxy

import (
	"strings"
	"testing"

	"raioz/internal/domain/interfaces"
)

func TestContainerName(t *testing.T) {
	name := ContainerName("acme-corp-net")
	if name != "raioz-proxy-acme-corp-net" {
		t.Errorf("expected 'raioz-proxy-acme-corp-net', got '%s'", name)
	}
}

func TestGetURL(t *testing.T) {
	m := NewManager("")
	m.routes["api"] = interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
	}

	url := m.GetURL("api")
	if url != "https://api.localhost" {
		t.Errorf("expected 'https://api.localhost', got '%s'", url)
	}

	url = m.GetURL("nonexistent")
	if url != "" {
		t.Errorf("expected empty, got '%s'", url)
	}
}

func TestCaddyfileGeneration_Basic(t *testing.T) {
	m := NewManager("")
	m.routes["api"] = interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api-container",
		Port:        3000,
	}

	content := m.GenerateCaddyfileContent()

	if !strings.Contains(content, "http://api.localhost") {
		t.Error("expected http://api.localhost in Caddyfile")
	}
	if !strings.Contains(content, "reverse_proxy api-container:3000") {
		t.Errorf("expected reverse_proxy directive, got:\n%s", content)
	}
}

func TestCaddyfileGeneration_WithCerts(t *testing.T) {
	m := NewManager("/certs")
	m.routes["api"] = interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api-container",
		Port:        3000,
	}

	content := m.GenerateCaddyfileContent()

	if !strings.Contains(content, "https://api.localhost") {
		t.Error("expected https with certs")
	}
	if !strings.Contains(content, "tls /certs/localhost.pem /certs/localhost-key.pem") {
		t.Error("expected TLS cert paths")
	}
}

func TestCaddyfileGeneration_WebSocket(t *testing.T) {
	m := NewManager("")
	m.routes["chat"] = interfaces.ProxyRoute{
		ServiceName: "chat",
		Hostname:    "chat",
		Target:      "chat-service:3000",
		WebSocket:   true,
	}

	content := m.GenerateCaddyfileContent()

	if !strings.Contains(content, "header_up X-Forwarded-Proto") {
		t.Error("expected WebSocket header_up directive")
	}
}

func TestCaddyfileGeneration_Stream(t *testing.T) {
	m := NewManager("")
	m.routes["notifications"] = interfaces.ProxyRoute{
		ServiceName: "notifications",
		Hostname:    "notifications",
		Target:      "notif-service:3000",
		Stream:      true,
	}

	content := m.GenerateCaddyfileContent()

	if !strings.Contains(content, "flush_interval -1") {
		t.Error("expected flush_interval for streaming")
	}
}

func TestCaddyfileGeneration_GRPC(t *testing.T) {
	m := NewManager("")
	m.routes["grpc-gw"] = interfaces.ProxyRoute{
		ServiceName: "grpc-gw",
		Hostname:    "grpc-gw",
		Target:      "grpc-service:50051",
		GRPC:        true,
	}

	content := m.GenerateCaddyfileContent()

	if !strings.Contains(content, "h2c://") {
		t.Error("expected h2c protocol for gRPC")
	}
}

func TestAddRemoveRoute(t *testing.T) {
	m := NewManager("")

	m.AddRoute(nil, interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api:3000",
	})

	if len(m.routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(m.routes))
	}

	m.RemoveRoute(nil, "api")

	if len(m.routes) != 0 {
		t.Errorf("expected 0 routes, got %d", len(m.routes))
	}
}

func TestHasMkcert(t *testing.T) {
	// This just verifies the function doesn't panic
	_ = HasMkcert()
}
