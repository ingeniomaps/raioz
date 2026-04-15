package proxy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/domain/interfaces"
)

func TestGenerateCaddyfileContent_ZeroPort(t *testing.T) {
	m := NewManager("")
	m.SetProjectName("test")
	m.AddRoute(nil, interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api-container",
		Port:        0,
	})

	got := m.GenerateCaddyfileContent()
	// Port 0 should not append ":0" to target
	if strings.Contains(got, "api-container:0") {
		t.Errorf("should not append :0 for zero port: %s", got)
	}
	if !strings.Contains(got, "reverse_proxy api-container") {
		t.Errorf("expected plain target: %s", got)
	}
}

func TestGenerateCaddyfileContent_AllRouteTypes(t *testing.T) {
	tests := []struct {
		name     string
		route    interfaces.ProxyRoute
		contains string
		absent   string
	}{
		{
			name: "plain reverse proxy",
			route: interfaces.ProxyRoute{
				ServiceName: "web",
				Hostname:    "web",
				Target:      "web:3000",
			},
			contains: "reverse_proxy web:3000\n",
			absent:   "flush_interval",
		},
		{
			name: "grpc with port append",
			route: interfaces.ProxyRoute{
				ServiceName: "grpc",
				Hostname:    "grpc",
				Target:      "grpc-svc",
				Port:        50051,
				GRPC:        true,
			},
			contains: "h2c://grpc-svc:50051",
		},
		{
			name: "stream with port append",
			route: interfaces.ProxyRoute{
				ServiceName: "sse",
				Hostname:    "sse",
				Target:      "sse-svc",
				Port:        8080,
				Stream:      true,
			},
			contains: "flush_interval -1",
		},
		{
			name: "websocket with port append",
			route: interfaces.ProxyRoute{
				ServiceName: "ws",
				Hostname:    "ws",
				Target:      "ws-svc",
				Port:        8080,
				WebSocket:   true,
			},
			contains: "X-Forwarded-Proto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager("")
			m.SetProjectName("test")
			m.AddRoute(nil, tt.route)

			got := m.GenerateCaddyfileContent()
			if !strings.Contains(got, tt.contains) {
				t.Errorf("expected %q in output:\n%s", tt.contains, got)
			}
			if tt.absent != "" && strings.Contains(got, tt.absent) {
				t.Errorf("unexpected %q in output:\n%s", tt.absent, got)
			}
		})
	}
}

func TestGenerateCaddyfile_GlobalOptions_MkcertWithCerts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("RAIOZ_HOME", home)

	m := NewManager("/certs")
	m.SetProjectName("test")
	m.SetTLSMode("mkcert")
	m.networkName = "test-net"
	m.AddRoute(nil, interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api:3000",
	})

	path, err := m.generateCaddyfile()
	if err != nil {
		t.Fatalf("generateCaddyfile: %v", err)
	}
	defer os.RemoveAll(filepath.Dir(path))

	data, _ := os.ReadFile(path)
	content := string(data)

	if !strings.Contains(content, "auto_https off") {
		t.Error("expected auto_https off for mkcert (prevents ACME fallback — BUG-12)")
	}
	if !strings.Contains(content, "https://api.localhost") {
		t.Error("expected https scheme for mkcert with certs")
	}
}

func TestGenerateCaddyfile_GlobalOptions_LetsEncrypt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("RAIOZ_HOME", home)

	m := NewManager("")
	m.SetProjectName("test")
	m.SetTLSMode("letsencrypt")
	m.SetDomain("acme.com")
	m.networkName = "test-net"
	m.AddRoute(nil, interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api:3000",
	})

	path, err := m.generateCaddyfile()
	if err != nil {
		t.Fatalf("generateCaddyfile: %v", err)
	}
	defer os.RemoveAll(filepath.Dir(path))

	data, _ := os.ReadFile(path)
	content := string(data)

	// letsencrypt should leave auto_https alone so Caddy handles ACME.
	if strings.Contains(content, "auto_https off") {
		t.Error("letsencrypt must NOT turn off auto_https (needs ACME)")
	}
	if strings.Contains(content, "auto_https disable_redirects") {
		t.Error("letsencrypt should not disable redirects")
	}
	if !strings.Contains(content, "https://api.acme.com") {
		t.Error("expected https for letsencrypt")
	}
}

func TestContainerName_WithProject(t *testing.T) {
	tests := []struct {
		project string
		want    string
	}{
		{"my-project", "raioz-proxy-my-project"},
		{"", "raioz-proxy-"},
		{"a", "raioz-proxy-a"},
	}
	for _, tt := range tests {
		got := ContainerName(tt.project)
		if got != tt.want {
			t.Errorf("ContainerName(%q) = %q, want %q",
				tt.project, got, tt.want)
		}
	}
}

func TestManager_AddRemoveRoute_Context(t *testing.T) {
	m := NewManager("")
	ctx := context.Background()

	// Add multiple routes
	routes := []interfaces.ProxyRoute{
		{ServiceName: "a", Hostname: "a", Target: "a:1"},
		{ServiceName: "b", Hostname: "b", Target: "b:2"},
		{ServiceName: "c", Hostname: "c", Target: "c:3"},
	}
	for _, r := range routes {
		if err := m.AddRoute(ctx, r); err != nil {
			t.Fatalf("AddRoute(%s): %v", r.ServiceName, err)
		}
	}
	if len(m.routes) != 3 {
		t.Errorf("expected 3 routes, got %d", len(m.routes))
	}

	// Remove middle route
	if err := m.RemoveRoute(ctx, "b"); err != nil {
		t.Fatalf("RemoveRoute: %v", err)
	}
	if len(m.routes) != 2 {
		t.Errorf("expected 2 routes, got %d", len(m.routes))
	}

	// Remove non-existent (should not error)
	if err := m.RemoveRoute(ctx, "nonexistent"); err != nil {
		t.Errorf("RemoveRoute(nonexistent): %v", err)
	}

	// Verify URLs
	if m.GetURL("a") == "" {
		t.Error("expected URL for 'a'")
	}
	if m.GetURL("b") != "" {
		t.Error("expected empty URL for removed 'b'")
	}
}

func TestEnsureCerts_HomeDirUnavailable(t *testing.T) {
	// Set HOME to empty which makes os.UserHomeDir fail on some systems
	// but more reliably: test the existing-certs path with pre-created files
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Certs now live under the per-domain subdirectory and must carry a
	// matching SAN — else EnsureCerts treats them as stale and regenerates.
	domainDir := filepath.Join(home, ".raioz", "certs", "custom.domain")
	os.MkdirAll(domainDir, 0o755)
	writeSelfSignedCert(t, filepath.Join(domainDir, certFileName),
		[]string{"custom.domain", "*.custom.domain"})
	os.WriteFile(filepath.Join(domainDir, keyFileName), []byte("key"), 0o644)

	got, err := EnsureCerts("custom.domain")
	if err != nil {
		t.Fatalf("EnsureCerts: %v", err)
	}
	if got != domainDir {
		t.Errorf("expected %q, got %q", domainDir, got)
	}
}

func TestHasExistingCerts_Directory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	certsDir := filepath.Join(home, ".raioz", "certs")
	os.MkdirAll(certsDir, 0o755)

	// Create cert path as a directory instead of file
	os.MkdirAll(filepath.Join(certsDir, certFileName), 0o755)
	os.WriteFile(filepath.Join(certsDir, keyFileName), []byte("key"), 0o644)

	// Should return false because certFileName is a directory
	if HasExistingCerts() {
		t.Error("expected false when cert path is a directory")
	}
}
