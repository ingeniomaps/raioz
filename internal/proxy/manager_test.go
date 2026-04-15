package proxy

import (
	"context"
	"testing"

	"raioz/internal/domain/interfaces"
)

func TestNewManager(t *testing.T) {
	m := NewManager("/certs")
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if m.certsDir != "/certs" {
		t.Errorf("certsDir: got %q", m.certsDir)
	}
	if m.domain != "localhost" {
		t.Errorf("default domain: got %q", m.domain)
	}
	if m.tlsMode != "mkcert" {
		t.Errorf("default tlsMode: got %q", m.tlsMode)
	}
}

func TestManager_SetDomain(t *testing.T) {
	m := NewManager("")
	m.SetDomain("acme.dev")
	if m.domain != "acme.dev" {
		t.Errorf("got %q", m.domain)
	}
}

func TestManager_SetDomain_Empty(t *testing.T) {
	m := NewManager("")
	original := m.domain
	m.SetDomain("")
	if m.domain != original {
		t.Error("empty domain should not overwrite")
	}
}

func TestManager_SetTLSMode(t *testing.T) {
	m := NewManager("")
	m.SetTLSMode("letsencrypt")
	if m.tlsMode != "letsencrypt" {
		t.Errorf("got %q", m.tlsMode)
	}
}

func TestManager_SetTLSMode_Empty(t *testing.T) {
	m := NewManager("")
	original := m.tlsMode
	m.SetTLSMode("")
	if m.tlsMode != original {
		t.Error("empty mode should not overwrite")
	}
}

func TestManager_SetBindHost(t *testing.T) {
	m := NewManager("")
	m.SetBindHost("0.0.0.0")
	if m.bindHost != "0.0.0.0" {
		t.Errorf("got %q", m.bindHost)
	}
}

func TestManager_SetProjectName(t *testing.T) {
	m := NewManager("")
	m.SetProjectName("myproj")
	if m.projectName != "myproj" {
		t.Errorf("got %q", m.projectName)
	}
}

func TestManager_AddRoute(t *testing.T) {
	m := NewManager("")
	ctx := context.Background()

	err := m.AddRoute(ctx, interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api",
		Port:        8080,
	})
	if err != nil {
		t.Errorf("AddRoute: %v", err)
	}
	if len(m.routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(m.routes))
	}
}

func TestManager_AddRoute_Overwrite(t *testing.T) {
	m := NewManager("")
	ctx := context.Background()

	m.AddRoute(ctx, interfaces.ProxyRoute{ServiceName: "api", Port: 8080})
	m.AddRoute(ctx, interfaces.ProxyRoute{ServiceName: "api", Port: 9090})

	if len(m.routes) != 1 {
		t.Errorf("expected 1 route (overwritten), got %d", len(m.routes))
	}
	if m.routes["api"].Port != 9090 {
		t.Errorf("expected port 9090, got %d", m.routes["api"].Port)
	}
}

func TestManager_RemoveRoute(t *testing.T) {
	m := NewManager("")
	ctx := context.Background()

	m.AddRoute(ctx, interfaces.ProxyRoute{ServiceName: "api"})
	m.AddRoute(ctx, interfaces.ProxyRoute{ServiceName: "web"})

	if err := m.RemoveRoute(ctx, "api"); err != nil {
		t.Errorf("RemoveRoute: %v", err)
	}
	if _, ok := m.routes["api"]; ok {
		t.Error("api route should be removed")
	}
	if _, ok := m.routes["web"]; !ok {
		t.Error("web route should remain")
	}
}

func TestManager_GetURL_Found(t *testing.T) {
	m := NewManager("")
	m.SetDomain("localhost")
	m.AddRoute(context.Background(), interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
	})

	url := m.GetURL("api")
	if url != "https://api.localhost" {
		t.Errorf("got %q", url)
	}
}

func TestManager_GetURL_NotFound(t *testing.T) {
	m := NewManager("")
	url := m.GetURL("nonexistent")
	if url != "" {
		t.Errorf("expected empty, got %q", url)
	}
}

func TestManager_GetURL_CustomDomain(t *testing.T) {
	m := NewManager("")
	m.SetDomain("acme.dev")
	m.AddRoute(context.Background(), interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
	})

	url := m.GetURL("api")
	if url != "https://api.acme.dev" {
		t.Errorf("got %q", url)
	}
}
