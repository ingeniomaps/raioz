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
	"strings"
	"time"
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
}

// NewManager creates a tunnel Manager.
func NewManager() *Manager {
	home, _ := os.UserHomeDir()
	return &Manager{
		registryPath: filepath.Join(home, ".raioz", "tunnels.json"),
	}
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
					proc.Kill()
				}
			}
			tunnels = append(tunnels[:i], tunnels[i+1:]...)
			m.saveAll(tunnels)
			return nil
		}
	}
	return fmt.Errorf("no active tunnel for '%s'", serviceName)
}

// StopAll kills all tunnels.
func (m *Manager) StopAll() {
	for _, t := range m.loadAll() {
		if t.PID > 0 {
			if proc, err := os.FindProcess(t.PID); err == nil {
				proc.Kill()
			}
		}
	}
	os.Remove(m.registryPath)
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
		return nil, err
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
		cmd.Process.Kill()
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
	return "", fmt.Errorf("no tunnel backend found. Install cloudflared or bore:\n" +
		"  brew install cloudflare/cloudflare/cloudflared\n" +
		"  cargo install bore-cli")
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
	if err != nil {
		return nil
	}
	var tunnels []Info
	json.Unmarshal(data, &tunnels)
	return tunnels
}

func (m *Manager) saveAll(tunnels []Info) {
	os.MkdirAll(filepath.Dir(m.registryPath), 0755)
	data, _ := json.MarshalIndent(tunnels, "", "  ")
	os.WriteFile(m.registryPath, data, 0600)
}

// DetectBackend returns the name of the available backend, or error.
func DetectBackend() (string, error) {
	return detectBackend()
}

// HasBackend returns true if a tunnel backend is available.
func HasBackend() bool {
	_, err := detectBackend()
	return err == nil
}

// FormatURL returns a clean display URL. Trims protocol if present.
func FormatURL(url string) string {
	return strings.TrimPrefix(url, "https://")
}
