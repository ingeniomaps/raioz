package tunnel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newTestManager returns a Manager whose registry lives inside t.TempDir().
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	return &Manager{
		registryPath: filepath.Join(dir, "tunnels.json"),
	}
}

func TestManager_SaveAndLoadAll(t *testing.T) {
	m := newTestManager(t)

	info := &Info{
		ServiceName: "api",
		LocalPort:   8080,
		PublicURL:   "https://abc.trycloudflare.com",
		Backend:     "cloudflared",
		PID:         0, // avoid process lookup in other tests
		StartedAt:   time.Now(),
	}
	m.save(info)

	tunnels := m.loadAll()
	if len(tunnels) != 1 {
		t.Fatalf("expected 1 tunnel, got %d", len(tunnels))
	}
	if tunnels[0].ServiceName != "api" {
		t.Errorf("expected 'api', got %q", tunnels[0].ServiceName)
	}
	if tunnels[0].PublicURL != info.PublicURL {
		t.Errorf("URL mismatch: %q", tunnels[0].PublicURL)
	}
}

func TestManager_SaveReplacesExisting(t *testing.T) {
	m := newTestManager(t)

	m.save(&Info{ServiceName: "api", LocalPort: 8080, Backend: "bore"})
	m.save(&Info{ServiceName: "api", LocalPort: 9090, Backend: "cloudflared"})
	m.save(&Info{ServiceName: "web", LocalPort: 3000, Backend: "bore"})

	tunnels := m.loadAll()
	if len(tunnels) != 2 {
		t.Fatalf("expected 2 tunnels, got %d", len(tunnels))
	}

	// Find api and check it was replaced
	var api *Info
	for i := range tunnels {
		if tunnels[i].ServiceName == "api" {
			api = &tunnels[i]
		}
	}
	if api == nil {
		t.Fatal("expected api tunnel to exist")
	}
	if api.LocalPort != 9090 || api.Backend != "cloudflared" {
		t.Errorf("expected replaced tunnel, got port=%d backend=%s",
			api.LocalPort, api.Backend)
	}
}

func TestManager_LoadAllMissingFile(t *testing.T) {
	m := &Manager{registryPath: filepath.Join(t.TempDir(), "missing.json")}
	if tunnels := m.loadAll(); tunnels != nil {
		t.Errorf("expected nil, got %+v", tunnels)
	}
}

func TestManager_LoadAllInvalidJSON(t *testing.T) {
	m := newTestManager(t)
	// Write garbage
	if err := os.MkdirAll(filepath.Dir(m.registryPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m.registryPath, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}

	// loadAll silently returns empty on unmarshal failure
	tunnels := m.loadAll()
	if len(tunnels) != 0 {
		t.Errorf("expected empty, got %d entries", len(tunnels))
	}
}

func TestManager_SaveAllCreatesDirectory(t *testing.T) {
	baseDir := t.TempDir()
	// Nested path that does not exist yet
	m := &Manager{
		registryPath: filepath.Join(baseDir, "nested", "deep", "tunnels.json"),
	}

	m.saveAll([]Info{{ServiceName: "api", LocalPort: 8080}})

	data, err := os.ReadFile(m.registryPath)
	if err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}

	var tunnels []Info
	if err := json.Unmarshal(data, &tunnels); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(tunnels) != 1 || tunnels[0].ServiceName != "api" {
		t.Errorf("unexpected content: %+v", tunnels)
	}
}

func TestManager_Stop_NonexistentService(t *testing.T) {
	m := newTestManager(t)

	err := m.Stop("ghost")
	if err == nil {
		t.Error("expected error for non-existent service")
	}
}

func TestManager_Stop_RemovesFromRegistry(t *testing.T) {
	m := newTestManager(t)

	// Save with PID=0 so no real process is killed
	m.save(&Info{ServiceName: "api", LocalPort: 8080, PID: 0})
	m.save(&Info{ServiceName: "web", LocalPort: 3000, PID: 0})

	if err := m.Stop("api"); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	remaining := m.loadAll()
	if len(remaining) != 1 || remaining[0].ServiceName != "web" {
		t.Errorf("expected only 'web' to remain, got %+v", remaining)
	}
}

func TestManager_StopAll_RemovesFile(t *testing.T) {
	m := newTestManager(t)

	m.save(&Info{ServiceName: "api", LocalPort: 8080, PID: 0})
	m.save(&Info{ServiceName: "web", LocalPort: 3000, PID: 0})

	m.StopAll()

	if _, err := os.Stat(m.registryPath); !os.IsNotExist(err) {
		t.Error("expected registry file to be removed")
	}
}

func TestManager_List_FiltersDeadPIDs(t *testing.T) {
	m := newTestManager(t)

	// PID=0 is treated as dead (skipped in List loop)
	m.save(&Info{ServiceName: "dead0", LocalPort: 1234, PID: 0})
	m.save(&Info{ServiceName: "dead-neg", LocalPort: 2345, PID: -1})

	alive := m.List()
	if len(alive) != 0 {
		t.Errorf("expected all filtered out, got %d: %+v", len(alive), alive)
	}

	// After filtering, registry should be rewritten with 0 entries
	remaining := m.loadAll()
	if len(remaining) != 0 {
		t.Errorf("expected registry rewritten empty, got %d", len(remaining))
	}
}

func TestManager_Start_NoBackend(t *testing.T) {
	// If neither cloudflared nor bore is installed, Start should fail.
	if HasBackend() {
		t.Skip("tunnel backend is installed; skipping no-backend test")
	}
	m := newTestManager(t)
	_, err := m.Start(nil, "api", 8080)
	if err == nil {
		t.Error("expected error when no backend is available")
	}
}

func TestManager_StopAll_EmptyRegistry(t *testing.T) {
	m := newTestManager(t)
	// No tunnels saved — should not panic
	m.StopAll()
}

func TestManager_StopAll_WithDeadPIDs(t *testing.T) {
	m := newTestManager(t)

	// Save an entry with PID > 0 to exercise the FindProcess/Kill branches.
	// PID 99999 is very unlikely to exist; Kill is a no-op on Linux if
	// the process doesn't exist (error is silently ignored).
	m.save(&Info{ServiceName: "dead", LocalPort: 1234, PID: 99999})

	m.StopAll()

	if _, err := os.Stat(m.registryPath); !os.IsNotExist(err) {
		t.Error("expected registry file to be removed")
	}
}

func TestManager_Stop_WithNonZeroDeadPID(t *testing.T) {
	m := newTestManager(t)
	m.save(&Info{ServiceName: "zombie", LocalPort: 8080, PID: 99999})

	if err := m.Stop("zombie"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if len(m.loadAll()) != 0 {
		t.Error("expected empty registry after Stop")
	}
}

func TestManager_List_RewritesWhenShrinking(t *testing.T) {
	m := newTestManager(t)

	// Save 3 entries with dead PIDs
	m.save(&Info{ServiceName: "a", LocalPort: 1, PID: 0})
	m.save(&Info{ServiceName: "b", LocalPort: 2, PID: 0})
	m.save(&Info{ServiceName: "c", LocalPort: 3, PID: 0})

	alive := m.List()
	if len(alive) != 0 {
		t.Errorf("expected 0 alive, got %d", len(alive))
	}

	// Registry should have been rewritten
	if len(m.loadAll()) != 0 {
		t.Error("expected registry to be rewritten empty")
	}
}

func TestManager_Stop_WithDeadPID(t *testing.T) {
	m := newTestManager(t)

	// PID=0 means no process will be killed, but the entry should still be removed
	m.save(&Info{ServiceName: "api", LocalPort: 8080, PID: 0})

	if err := m.Stop("api"); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if len(m.loadAll()) != 0 {
		t.Error("expected registry to be empty after Stop")
	}
}

func TestDetectBackend_ReturnsKnownBackend(t *testing.T) {
	// We cannot guarantee any backend; just verify the string is one of the
	// allowed values when no error is returned.
	backend, err := detectBackend()
	if err != nil {
		return // no backend installed — acceptable
	}
	if backend != "cloudflared" && backend != "bore" {
		t.Errorf("unexpected backend: %q", backend)
	}
}

func TestManager_List_PIDBranches(t *testing.T) {
	m := newTestManager(t)

	// PID > 0 with a process that does not exist exercises the inner
	// FindProcess / Signal checks (PID=99999 is very unlikely to be live
	// and signalable in a test environment — and since Signal(nil) always
	// returns an error, the entry is still filtered out).
	m.save(&Info{ServiceName: "zombie", LocalPort: 1, PID: 99999})

	alive := m.List()
	if len(alive) != 0 {
		t.Errorf("expected 0 alive, got %d", len(alive))
	}
}

func TestCloudflaredURLRegex(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		{"Your url is: https://abc123.trycloudflare.com | info",
			"https://abc123.trycloudflare.com"},
		{"https://my-test-tunnel.trycloudflare.com",
			"https://my-test-tunnel.trycloudflare.com"},
		{"no url here", ""},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := cloudflaredURLRegex.FindString(tt.line)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatURL_MoreCases(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://foo.com", "foo.com"},
		{"http://foo.com", "http://foo.com"}, // only https stripped
		{"", ""},
		{"plain.example", "plain.example"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := FormatURL(tt.input); got != tt.want {
				t.Errorf("FormatURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestInfo_JSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	orig := Info{
		ServiceName: "api",
		LocalPort:   8080,
		PublicURL:   "https://abc.trycloudflare.com",
		Backend:     "cloudflared",
		PID:         1234,
		StartedAt:   now,
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got Info
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.ServiceName != orig.ServiceName || got.LocalPort != orig.LocalPort {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}
