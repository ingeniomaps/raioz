// Package tunnel exposes local services to the internet via cloudflared or bore.
package tunnel

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

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
			if t.PID > 0 {
				if proc, err := os.FindProcess(t.PID); err == nil {
					_ = proc.Kill()
				}
			}
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
		if t.PID > 0 {
			if proc, err := os.FindProcess(t.PID); err == nil {
				_ = proc.Kill()
			}
		}
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
		if t.PID > 0 {
			if proc, err := os.FindProcess(t.PID); err == nil {
				if proc.Signal(nil) == nil {
					alive = append(alive, t)
					continue
				}
			}
		}
	}
	if len(alive) != len(tunnels) {
		m.saveAll(alive)
	}
	return alive
}

var cloudflaredURLRegex = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

func (m *Manager) startCloudflared(ctx context.Context, serviceName string, port int) (*Info, error) {
	cmd := exec.CommandContext(ctx, "cloudflared", "tunnel", "--url",
		fmt.Sprintf("http://localhost:%d", port))

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("cloudflared stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start cloudflared: %w", err)
	}

	// Parse URL from stderr (cloudflared prints it there)
	urlCh := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if match := cloudflaredURLRegex.FindString(line); match != "" {
				urlCh <- match
				return
			}
		}
	}()

	select {
	case url := <-urlCh:
		return &Info{
			ServiceName: serviceName,
			LocalPort:   port,
			PublicURL:   url,
			Backend:     "cloudflared",
			PID:         cmd.Process.Pid,
			StartedAt:   time.Now(),
		}, nil
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("cloudflared did not return a URL within 15 seconds")
	}
}

func (m *Manager) startBore(_ context.Context, serviceName string, port int) (*Info, error) {
	cmd := exec.Command("bore", "local", fmt.Sprintf("%d", port), "--to", "bore.pub")
	if err := cmd.Start(); err != nil {
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
