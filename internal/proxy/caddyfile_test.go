package proxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/domain/interfaces"
)

func TestGenerateCaddyfileContent_Empty(t *testing.T) {
	m := NewManager("")
	m.SetProjectName("test")
	got := m.GenerateCaddyfileContent()
	if got != "" {
		t.Errorf("expected empty for no routes, got %q", got)
	}
}

func TestGenerateCaddyfileContent_SimpleRoute(t *testing.T) {
	m := NewManager("")
	m.SetProjectName("test")
	m.AddRoute(nil, interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api",
		Port:        8080,
	})

	got := m.GenerateCaddyfileContent()
	if !strings.Contains(got, "api.localhost") {
		t.Errorf("expected api.localhost in output: %s", got)
	}
	if !strings.Contains(got, "reverse_proxy") {
		t.Errorf("expected reverse_proxy: %s", got)
	}
	if !strings.Contains(got, "api:8080") {
		t.Errorf("expected target api:8080: %s", got)
	}
}

// TestGenerateCaddyfileContent_DepHostnameOverride_Issue001: regression
// guard for the gouduet/keycloak case — the route emitted by buildProxyRoute
// for a dep with `hostname: mail` and `proxy.port: 8025` must produce a
// Caddyfile that routes https://mail.demo.dev → <container>:8025, not
// https://mailhog.demo.dev → <container>:1025 (the bug from issue #001 +
// #003 combined).
func TestGenerateCaddyfileContent_DepHostnameOverride_Issue001(t *testing.T) {
	m := NewManager("")
	m.SetProjectName("demo")
	m.SetDomain("demo.dev")
	m.AddRoute(nil, interfaces.ProxyRoute{
		ServiceName: "mailhog",
		Hostname:    "mail",
		Target:      "demo-mailhog",
		Port:        8025,
	})

	got := m.GenerateCaddyfileContent()
	if !strings.Contains(got, "mail.demo.dev") {
		t.Errorf("expected mail.demo.dev (issue #001), got:\n%s", got)
	}
	if strings.Contains(got, "mailhog.demo.dev") {
		t.Errorf("did not expect entry-name mailhog.demo.dev (issue #001), got:\n%s", got)
	}
	if !strings.Contains(got, "demo-mailhog:8025") {
		t.Errorf("expected reverse_proxy demo-mailhog:8025 (issue #003), got:\n%s", got)
	}
	if strings.Contains(got, ":1025") {
		t.Errorf("did not expect SMTP port 1025 leaking through (issue #003), got:\n%s", got)
	}
}

func TestGenerateCaddyfileContent_CustomDomain(t *testing.T) {
	m := NewManager("")
	m.SetProjectName("test")
	m.SetDomain("acme.dev")
	m.AddRoute(nil, interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api",
		Port:        8080,
	})

	got := m.GenerateCaddyfileContent()
	if !strings.Contains(got, "api.acme.dev") {
		t.Errorf("expected api.acme.dev: %s", got)
	}
}

func TestGenerateCaddyfileContent_GRPC(t *testing.T) {
	m := NewManager("")
	m.SetProjectName("test")
	m.AddRoute(nil, interfaces.ProxyRoute{
		ServiceName: "grpc-svc",
		Hostname:    "grpc",
		Target:      "grpc-svc",
		Port:        50051,
		GRPC:        true,
	})

	got := m.GenerateCaddyfileContent()
	if !strings.Contains(got, "h2c://") {
		t.Errorf("expected h2c:// for gRPC: %s", got)
	}
}

func TestGenerateCaddyfileContent_Stream(t *testing.T) {
	m := NewManager("")
	m.SetProjectName("test")
	m.AddRoute(nil, interfaces.ProxyRoute{
		ServiceName: "stream",
		Hostname:    "stream",
		Target:      "stream",
		Port:        8080,
		Stream:      true,
	})

	got := m.GenerateCaddyfileContent()
	if !strings.Contains(got, "flush_interval -1") {
		t.Errorf("expected flush_interval for stream: %s", got)
	}
}

func TestGenerateCaddyfileContent_WebSocket(t *testing.T) {
	m := NewManager("")
	m.SetProjectName("test")
	m.AddRoute(nil, interfaces.ProxyRoute{
		ServiceName: "ws",
		Hostname:    "ws",
		Target:      "ws",
		Port:        8080,
		WebSocket:   true,
	})

	got := m.GenerateCaddyfileContent()
	if !strings.Contains(got, "X-Forwarded-Proto") {
		t.Errorf("expected X-Forwarded-Proto for WS: %s", got)
	}
}

func TestGenerateCaddyfileContent_TLSMkcert(t *testing.T) {
	m := NewManager("/certs")
	m.SetProjectName("test")
	m.SetTLSMode("mkcert")
	m.AddRoute(nil, interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api",
		Port:        8080,
	})

	got := m.GenerateCaddyfileContent()
	if !strings.Contains(got, "https://api.localhost") {
		t.Errorf("expected https:// for mkcert: %s", got)
	}
	if !strings.Contains(got, "tls ") {
		t.Errorf("expected tls directive: %s", got)
	}
}

func TestGenerateCaddyfileContent_LetsEncrypt(t *testing.T) {
	m := NewManager("")
	m.SetProjectName("test")
	m.SetDomain("acme.com")
	m.SetTLSMode("letsencrypt")
	m.AddRoute(nil, interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api",
		Port:        8080,
	})

	got := m.GenerateCaddyfileContent()
	if !strings.Contains(got, "https://api.acme.com") {
		t.Errorf("expected https:// for letsencrypt: %s", got)
	}
	// letsencrypt should NOT have explicit tls directive
	if strings.Contains(got, "tls /certs/") {
		t.Errorf("letsencrypt should not have explicit tls path: %s", got)
	}
}

func TestGenerateCaddyfileContent_TargetWithPort(t *testing.T) {
	m := NewManager("")
	m.SetProjectName("test")
	// Target already has :port — should not append port again
	m.AddRoute(nil, interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api:9090",
		Port:        8080,
	})

	got := m.GenerateCaddyfileContent()
	if !strings.Contains(got, "api:9090") {
		t.Errorf("expected api:9090 (existing port preserved): %s", got)
	}
	if strings.Contains(got, "api:9090:8080") {
		t.Errorf("port should not be appended twice: %s", got)
	}
}

func TestGenerateCaddyfileContent_MultipleRoutes(t *testing.T) {
	m := NewManager("")
	m.SetProjectName("test")
	m.AddRoute(nil, interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api",
		Port:        8080,
	})
	m.AddRoute(nil, interfaces.ProxyRoute{
		ServiceName: "web",
		Hostname:    "web",
		Target:      "web",
		Port:        3000,
	})

	got := m.GenerateCaddyfileContent()
	if !strings.Contains(got, "api.localhost") {
		t.Error("missing api route")
	}
	if !strings.Contains(got, "web.localhost") {
		t.Error("missing web route")
	}
}

func TestGenerateCaddyfile_WritesFile(t *testing.T) {
	// Override proxy dir to temp
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("RAIOZ_HOME", home)

	m := NewManager("")
	m.SetProjectName("test")
	m.networkName = "test-net"
	m.AddRoute(nil, interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api",
		Port:        8080,
	})

	path, err := m.generateCaddyfile()
	if err != nil {
		t.Fatalf("generateCaddyfile: %v", err)
	}
	if filepath.Base(path) != "Caddyfile" {
		t.Errorf("expected Caddyfile, got %q", filepath.Base(path))
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Errorf("Caddyfile not written: %v", err)
	}

	// Verify content
	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "api.localhost") {
		t.Errorf("Caddyfile missing route: %s", content)
	}
	// Global options should be present
	if !strings.Contains(content, "{") {
		t.Errorf("missing global options: %s", content)
	}
}
