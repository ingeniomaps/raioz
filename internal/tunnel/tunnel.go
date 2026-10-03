// Package tunnel exposes local services to the internet via cloudflared or bore.
package tunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"raioz/internal/host"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/naming"
)

// Info represents an active tunnel.
type Info struct {
	ServiceName string    `json:"serviceName"`
	LocalPort   int       `json:"localPort"`
	PublicURL   string    `json:"publicURL"`
	Backend     string    `json:"backend"` // "cloudflared" or "bore"
	PID         int       `json:"pid"`
	StartedAt   time.Time `json:"startedAt"`
}

// Manager handles tunnel lifecycle.
type Manager struct {
	registryPath string
	// legacyPath is where raioz kept the registry before ADR-022; read
	// when registryPath does not exist yet, never written.
	legacyPath string
}

// NewManager creates a tunnel Manager.
func NewManager() *Manager {
	m := &Manager{registryPath: filepath.Join(naming.RaiozStateDir(), "tunnels.json")}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		m.legacyPath = filepath.Join(home, ".raioz", "tunnels.json")
	}
	return m
}

// Start creates a tunnel for a local port using the best available backend.
func (m *Manager) Start(ctx context.Context, serviceName string, localPort int) (*Info, error) {
	backend, err := detectBackend()
	if err != nil {
		return nil, err
	}

	var info *Info
	switch backend {
	case "cloudflared":
		info, err = m.startCloudflared(ctx, serviceName, localPort)
	case "bore":
		info, err = m.startBore(ctx, serviceName, localPort)
	default:
		return nil, fmt.Errorf("no tunnel backend available")
	}
	if err != nil {
		return nil, err
	}

	// Save to registry
	m.save(info)
	return info, nil
}

// Stop kills the tunnel process for a service.
func (m *Manager) Stop(serviceName string) error {
	tunnels := m.loadAll()
	for i, t := range tunnels {
		if t.ServiceName == serviceName {
			stopTunnelProcess(t.PID)
			tunnels = append(tunnels[:i], tunnels[i+1:]...)
			m.saveAll(tunnels)
			return nil
		}
	}
	return fmt.Errorf("%s", i18n.T("error.tunnel_not_active", serviceName))
}

// StopAll kills all tunnels.
func (m *Manager) StopAll() {
	for _, t := range m.loadAll() {
		stopTunnelProcess(t.PID)
	}
	_ = os.Remove(m.registryPath)
	if m.legacyPath != "" {
		_ = os.Remove(m.legacyPath)
	}
}

// List returns all active tunnels, cleaning up dead ones.
func (m *Manager) List() []Info {
	tunnels := m.loadAll()
	var alive []Info
	for _, t := range tunnels {
		// host.IsProcessAlive, not proc.Signal(nil): a nil signal is an
		// "unsupported signal type" error on every platform, which read
		// every tunnel as dead and emptied the registry on each list.
		if t.PID > 0 && host.IsProcessAlive(t.PID) {
			alive = append(alive, t)
		}
	}
	if len(alive) != len(tunnels) {
		m.saveAll(alive)
	}
	return alive
}

var cloudflaredURLRegex = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

// tunnelURLTimeout bounds the wait for cloudflared to announce its URL.
const tunnelURLTimeout = 30 * time.Second

func (m *Manager) startCloudflared(_ context.Context, serviceName string, port int) (*Info, error) {
	cmd, logPath, err := startDetached(serviceName,
		"cloudflared", "tunnel", "--url", fmt.Sprintf("http://localhost:%d", port))
	if err != nil {
		return nil, fmt.Errorf("failed to start cloudflared: %w", err)
	}

	// cloudflared prints the URL in its log; read it from the file the
	// process writes to rather than from a pipe that dies with raioz.
	deadline := time.Now().Add(tunnelURLTimeout)
	for time.Now().Before(deadline) {
		if data, readErr := os.ReadFile(logPath); readErr == nil {
			if url := cloudflaredURLRegex.FindString(string(data)); url != "" {
				return &Info{
					ServiceName: serviceName,
					LocalPort:   port,
					PublicURL:   url,
					Backend:     "cloudflared",
					PID:         cmd.Process.Pid,
					StartedAt:   time.Now(),
				}, nil
			}
		}
		if !host.IsProcessAlive(cmd.Process.Pid) {
			return nil, fmt.Errorf("cloudflared exited before returning a URL; see %s", logPath)
		}
		time.Sleep(200 * time.Millisecond)
	}
	stopTunnelProcess(cmd.Process.Pid)
	return nil, fmt.Errorf("cloudflared did not return a URL within %s; see %s", tunnelURLTimeout, logPath)
}

func (m *Manager) startBore(_ context.Context, serviceName string, port int) (*Info, error) {
	cmd, _, err := startDetached(serviceName, "bore", "local", fmt.Sprintf("%d", port), "--to", "bore.pub")
	if err != nil {
		return nil, fmt.Errorf("failed to start bore: %w", err)
	}

	// Bore doesn't give us the URL easily, construct it
	return &Info{
		ServiceName: serviceName,
		LocalPort:   port,
		PublicURL:   fmt.Sprintf("bore.pub (port forwarded from %d)", port),
		Backend:     "bore",
		PID:         cmd.Process.Pid,
		StartedAt:   time.Now(),
	}, nil
}

// tunnelLogPath is where a tunnel's backend writes its output.
func tunnelLogPath(serviceName string) string {
	return filepath.Join(naming.RaiozStateDir(), "logs", "tunnels", serviceName+".log")
}

// startDetached starts a tunnel backend that outlives the raioz command
// that launched it. `raioz tunnel` returns as soon as it has the URL, so the
// backend cannot hang off the command's context (cancelled on exit) nor
// write to a pipe raioz holds (closed on exit — the next log line kills
// it): it gets its own process group and a log file.
func startDetached(serviceName, name string, args ...string) (*exec.Cmd, string, error) {
	logPath := tunnelLogPath(serviceName)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, "", fmt.Errorf("create tunnel log dir: %w", err)
	}
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, "", fmt.Errorf("create tunnel log: %w", err)
	}
	defer logFile.Close()

	cmd := exec.Command(name, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	host.SetNewProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		return nil, "", fmt.Errorf("start %s: %w", name, err)
	}
	// Reap it if it dies while raioz is still here; once raioz exits it is
	// init's child.
	go func() { _ = cmd.Wait() }()
	return cmd, logPath, nil
}

// stopTunnelProcess ends a tunnel backend: a graceful signal to its group,
// then a forced one if it is still there a few seconds later.
func stopTunnelProcess(pid int) {
	if pid <= 0 || !host.IsProcessAlive(pid) {
		return
	}
	_ = host.KillProcessTree(pid)
	for i := 0; i < 30 && host.IsProcessAlive(pid); i++ {
		time.Sleep(100 * time.Millisecond)
	}
	if host.IsProcessAlive(pid) {
		_ = host.ForceKillProcessTree(pid)
	}
}

func detectBackend() (string, error) {
	if _, err := exec.LookPath("cloudflared"); err == nil {
		return "cloudflared", nil
	}
	if _, err := exec.LookPath("bore"); err == nil {
		return "bore", nil
	}
	return "", fmt.Errorf("%s", i18n.T("error.tunnel_no_backend"))
}

func (m *Manager) save(info *Info) {
	all := m.loadAll()
	// Replace if exists
	found := false
	for i, t := range all {
		if t.ServiceName == info.ServiceName {
			all[i] = *info
			found = true
			break
		}
	}
	if !found {
		all = append(all, *info)
	}
	m.saveAll(all)
}

func (m *Manager) loadAll() []Info {
	data, err := os.ReadFile(m.registryPath)
	if err != nil && m.legacyPath != "" {
		// Tunnels an older raioz started are still running; find them.
		data, err = os.ReadFile(m.legacyPath)
	}
	if err != nil {
		return nil
	}
	var tunnels []Info
	if err := json.Unmarshal(data, &tunnels); err != nil {
		logging.Warn("Tunnel registry is corrupt, ignoring",
			"path", m.registryPath, "error", err)
		return nil
	}
	return tunnels
}

func (m *Manager) saveAll(tunnels []Info) {
	// Best-effort persistence: tunnel registry is a cache. On write
	// failure, subsequent loadAll returns an empty list and we start
	// fresh — no user-visible break beyond losing the tunnel history.
	if err := os.MkdirAll(filepath.Dir(m.registryPath), 0755); err != nil {
		return
	}
	data, err := json.MarshalIndent(tunnels, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(m.registryPath, data, 0600)
}
