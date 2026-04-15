package proxy

import (
	"fmt"
	"net"
	"os"
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
	if !strings.Contains(content, "tls /certs/cert.pem /certs/cert-key.pem") {
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

func TestCheckPortsAvailable_AllFree(t *testing.T) {
	prev := portCheckFunc
	portCheckFunc = func(int) (bool, error) { return false, nil }
	defer func() { portCheckFunc = prev }()

	m := NewManager("")
	if err := m.checkPortsAvailable(); err != nil {
		t.Errorf("expected nil when every port probes free, got %v", err)
	}
}

func TestCheckPortsAvailable_ReportsTakenPorts(t *testing.T) {
	prev := portCheckFunc
	portCheckFunc = func(p int) (bool, error) { return p == 443, nil }
	defer func() { portCheckFunc = prev }()

	m := NewManager("")
	err := m.checkPortsAvailable()
	if err == nil || !strings.Contains(err.Error(), "443") {
		t.Errorf("expected error mentioning port 443, got %v", err)
	}
}

// TestIsHostPortInUse_PrivilegedPortsAsNonRoot guards against the original
// regression: binding :80/:443 as non-root returns EACCES, which the old
// probe misread as "port in use" and caused proxy preflight to reject every
// non-root up. Now EACCES is explicitly NOT treated as in-use.
func TestIsHostPortInUse_PrivilegedPortsAsNonRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("test is meaningful only when running as non-root")
	}
	// Port 80 is privileged. If nothing is listening on the test host, our
	// probe must NOT report it as in-use — that would be the regression.
	if inUse, err := isHostPortInUse(80); err != nil || inUse {
		t.Errorf("priv port probe as non-root: inUse=%v err=%v; want free (false, nil)",
			inUse, err)
	}
}

// TestProbeTCPDial_DetectsRealListener — the dial probe is what makes raioz
// catch port conflicts when running unprivileged (where bind alone reports
// EACCES). Spin up a real listener and confirm we report it as in-use.
func TestProbeTCPDial_DetectsRealListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listener: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	inUse, probed := probeTCPDial("127.0.0.1", port)
	if !probed || !inUse {
		t.Errorf("dial probe of real listener: inUse=%v probed=%v; want both true",
			inUse, probed)
	}
}

func TestProbeTCPDial_NobodyListening(t *testing.T) {
	// Pick a port that's almost certainly free (high ephemeral, then close).
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close() // release it

	inUse, probed := probeTCPDial("127.0.0.1", port)
	if !probed || inUse {
		t.Errorf("dial probe of empty port: inUse=%v probed=%v; want false/true",
			inUse, probed)
	}
}

func TestProbeTCPBind_RealOccupation(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listener: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	inUse, probed := probeTCPBind("127.0.0.1", port)
	if !probed {
		t.Fatal("probe should have returned a definitive answer for a real listener")
	}
	if !inUse {
		t.Errorf("expected inUse=true, got false")
	}
}

func TestCheckPortsAvailable_DetectsConflict(t *testing.T) {
	// Grab a free port, then verify isHostPortInUse flags it.
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("cannot open probe listener: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	inUse, err := isHostPortInUse(port)
	if err != nil {
		t.Fatalf("isHostPortInUse returned err: %v", err)
	}
	if !inUse {
		t.Errorf("expected port %d to be flagged as in-use", port)
	}

	_ = fmt.Sprintf(":%d", port) // keep fmt import used
}
