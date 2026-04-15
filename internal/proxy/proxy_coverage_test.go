package proxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/domain/interfaces"
)

func TestNewManager_Defaults(t *testing.T) {
	m := NewManager("/tmp/certs")
	if m.domain != "localhost" {
		t.Errorf("domain = %q, want %q", m.domain, "localhost")
	}
	if m.tlsMode != "mkcert" {
		t.Errorf("tlsMode = %q, want %q", m.tlsMode, "mkcert")
	}
	if m.certsDir != "/tmp/certs" {
		t.Errorf("certsDir = %q", m.certsDir)
	}
	if m.routes == nil {
		t.Error("routes map should be initialized")
	}
}

func TestSetDomain(t *testing.T) {
	m := NewManager("")
	m.SetDomain("dev.acme.com")
	if m.domain != "dev.acme.com" {
		t.Errorf("domain = %q, want %q", m.domain, "dev.acme.com")
	}

	// Empty string should not change domain
	m.SetDomain("")
	if m.domain != "dev.acme.com" {
		t.Errorf("domain should not change for empty string: %q", m.domain)
	}
}

func TestSetTLSMode(t *testing.T) {
	m := NewManager("")
	m.SetTLSMode("letsencrypt")
	if m.tlsMode != "letsencrypt" {
		t.Errorf("tlsMode = %q, want %q", m.tlsMode, "letsencrypt")
	}

	// Empty string should not change
	m.SetTLSMode("")
	if m.tlsMode != "letsencrypt" {
		t.Error("tlsMode should not change for empty string")
	}
}

func TestSetBindHost(t *testing.T) {
	m := NewManager("")
	m.SetBindHost("0.0.0.0")
	if m.bindHost != "0.0.0.0" {
		t.Errorf("bindHost = %q", m.bindHost)
	}
}

func TestSetProjectName(t *testing.T) {
	m := NewManager("")
	m.SetProjectName("myproject")
	if m.projectName != "myproject" {
		t.Errorf("projectName = %q", m.projectName)
	}
}

func TestGetURL_CustomDomain(t *testing.T) {
	m := NewManager("")
	m.SetDomain("dev.acme.com")
	m.routes["api"] = interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
	}

	url := m.GetURL("api")
	if url != "https://api.dev.acme.com" {
		t.Errorf("url = %q, want %q", url, "https://api.dev.acme.com")
	}
}

func TestGetURL_NotFound(t *testing.T) {
	m := NewManager("")
	url := m.GetURL("nonexistent")
	if url != "" {
		t.Errorf("expected empty for unknown service, got %q", url)
	}
}

func TestCaddyfileGeneration_LetsEncrypt(t *testing.T) {
	m := NewManager("")
	m.tlsMode = "letsencrypt"
	m.routes["api"] = interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api-container",
		Port:        3000,
	}

	content := m.GenerateCaddyfileContent()

	if !strings.Contains(content, "https://api.localhost") {
		t.Error("expected https for letsencrypt")
	}
	// Should NOT contain tls directive (caddy handles it automatically)
	if strings.Contains(content, "tls /certs") {
		t.Error("letsencrypt should not have manual tls directive")
	}
}

func TestCaddyfileGeneration_NoCerts(t *testing.T) {
	m := NewManager("")
	m.tlsMode = "mkcert"
	// No certsDir set
	m.routes["api"] = interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api-container",
		Port:        3000,
	}

	content := m.GenerateCaddyfileContent()

	if !strings.Contains(content, "http://api.localhost") {
		t.Error("expected http when no certs available")
	}
}

func TestCaddyfileGeneration_DefaultTLS(t *testing.T) {
	m := NewManager("")
	m.tlsMode = ""
	m.routes["api"] = interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api-container",
		Port:        3000,
	}

	content := m.GenerateCaddyfileContent()

	if !strings.Contains(content, "http://api.localhost") {
		t.Error("expected http for empty tls mode")
	}
}

func TestCaddyfileGeneration_MultipleRoutes(t *testing.T) {
	m := NewManager("/certs")
	m.routes["api"] = interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api:3000",
	}
	m.routes["web"] = interfaces.ProxyRoute{
		ServiceName: "web",
		Hostname:    "web",
		Target:      "web:8080",
	}

	content := m.GenerateCaddyfileContent()
	if !strings.Contains(content, "api.localhost") {
		t.Error("expected api.localhost")
	}
	if !strings.Contains(content, "web.localhost") {
		t.Error("expected web.localhost")
	}
}

func TestCaddyfileGeneration_TargetWithoutPort(t *testing.T) {
	m := NewManager("")
	m.routes["api"] = interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api-service",
		Port:        8080,
	}

	content := m.GenerateCaddyfileContent()
	if !strings.Contains(content, "api-service:8080") {
		t.Error("expected port to be appended to target")
	}
}

func TestCaddyfileGeneration_TargetWithPort(t *testing.T) {
	m := NewManager("")
	m.routes["api"] = interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api-service:3000",
		Port:        8080, // should be ignored since target already has port
	}

	content := m.GenerateCaddyfileContent()
	if strings.Contains(content, "api-service:3000:8080") {
		t.Error("should not double-add port when target already has one")
	}
	if !strings.Contains(content, "api-service:3000") {
		t.Error("expected original target with port")
	}
}

func TestGenerateCaddyfile_WritesToFile(t *testing.T) {
	tmpDir := t.TempDir()
	// We need to set networkName to a value that resolves to our temp dir
	// The naming.ProxyDir uses a specific path, so we test generateCaddyfile indirectly
	m := NewManager("/certs")
	m.networkName = "test-network"
	m.projectName = "test"
	m.routes["api"] = interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api:3000",
	}

	path, err := m.generateCaddyfile()
	if err != nil {
		t.Fatalf("generateCaddyfile: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "api.localhost") {
		t.Error("Caddyfile should contain api.localhost")
	}

	// Clean up the generated dir
	os.RemoveAll(filepath.Dir(path))

	_ = tmpDir // used for test isolation only
}

func TestHasExistingCerts_BothExist(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	certsDir := filepath.Join(home, ".raioz", "certs")
	os.MkdirAll(certsDir, 0o755)
	os.WriteFile(filepath.Join(certsDir, certFileName), []byte("cert"), 0o644)
	os.WriteFile(filepath.Join(certsDir, keyFileName), []byte("key"), 0o644)

	if !HasExistingCerts() {
		t.Error("expected true when both cert files exist")
	}
}

func TestHasExistingCerts_MissingCert(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	certsDir := filepath.Join(home, ".raioz", "certs")
	os.MkdirAll(certsDir, 0o755)
	// Only write key, not cert
	os.WriteFile(filepath.Join(certsDir, keyFileName), []byte("key"), 0o644)

	if HasExistingCerts() {
		t.Error("expected false when cert file is missing")
	}
}

func TestHasExistingCerts_MissingKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	certsDir := filepath.Join(home, ".raioz", "certs")
	os.MkdirAll(certsDir, 0o755)
	os.WriteFile(filepath.Join(certsDir, certFileName), []byte("cert"), 0o644)

	if HasExistingCerts() {
		t.Error("expected false when key file is missing")
	}
}

func TestHasExistingCerts_NoCertsDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if HasExistingCerts() {
		t.Error("expected false when certs dir does not exist")
	}
}

func TestCaddyfileGeneration_GlobalOptions(t *testing.T) {
	m := NewManager("/certs")
	m.routes["api"] = interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api:3000",
	}

	// generateCaddyfile includes global options block
	m.networkName = "test-net"
	m.projectName = "test"
	path, err := m.generateCaddyfile()
	if err != nil {
		t.Fatalf("generateCaddyfile: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)

	if !strings.Contains(content, "auto_https off") {
		t.Error("expected auto_https off in global options for mkcert mode (prevents ACME fallback — BUG-12)")
	}

	os.RemoveAll(filepath.Dir(path))
}

func TestEnsureCerts_NoMkcert(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Ensure PATH does not contain mkcert
	t.Setenv("PATH", "/nonexistent")

	got, err := EnsureCerts("localhost")
	if err != nil {
		t.Fatalf("EnsureCerts error: %v", err)
	}
	// Without mkcert, returns empty string (not an error)
	if got != "" {
		t.Errorf("expected empty dir without mkcert, got %q", got)
	}
}

func TestEnsureCerts_EmptyDomain(t *testing.T) {
	// Pre-create a valid cert under the default domain's namespaced folder.
	home := t.TempDir()
	t.Setenv("HOME", home)
	domainDir := filepath.Join(home, ".raioz", "certs", "localhost")
	os.MkdirAll(domainDir, 0o755)
	writeSelfSignedCert(t, filepath.Join(domainDir, certFileName),
		[]string{"localhost", "*.localhost"})
	os.WriteFile(filepath.Join(domainDir, keyFileName), []byte("k"), 0o644)

	got, err := EnsureCerts("") // empty domain defaults to "localhost"
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got != domainDir {
		t.Errorf("got %q, want %q", got, domainDir)
	}
}

func TestCertsDir_ReturnsAbsolute(t *testing.T) {
	dir := CertsDir()
	if dir == "" {
		t.Skip("no HOME set")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("expected absolute path, got %q", dir)
	}
	if !strings.HasSuffix(dir, ".raioz/certs") {
		t.Errorf("expected path ending in .raioz/certs, got %q", dir)
	}
}

func TestCaddyfileGeneration_NoGlobalOptionsWithoutCerts(t *testing.T) {
	m := NewManager("") // no certs
	m.routes["api"] = interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "api:3000",
	}

	m.networkName = "test-net"
	m.projectName = "test"
	path, err := m.generateCaddyfile()
	if err != nil {
		t.Fatalf("generateCaddyfile: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)

	// mkcert mode without a certsDir still turns auto_https off — Caddy
	// would otherwise attempt ACME, which is the exact scenario BUG-12
	// guards against.
	if !strings.Contains(content, "auto_https off") {
		t.Error("mkcert mode must disable auto_https even when certsDir is empty")
	}

	os.RemoveAll(filepath.Dir(path))
}
