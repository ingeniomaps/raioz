package tui

import (
	"context"

	"raioz/internal/domain/interfaces"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	maxLogLines         = 500
	maxLogLinesInactive = 50
	statsInterval       = 2 // seconds
)

// ViewMode controls what the TUI displays.
type ViewMode int

const (
	ViewNormal ViewMode = iota
	ViewLogsExpanded
)

// ServiceRow is one row in the services table.
type ServiceRow struct {
	Name    string
	Runtime string
	Status  string
	CPU     string
	Memory  string
	URL     string
	Uptime  string
}

// Config holds everything the TUI needs to start.
type Config struct {
	Project     string
	Workspace   string
	Services    []ServiceRow
	ComposePath string
	Docker      interfaces.DockerRunner
	Proxy       interfaces.ProxyManager
	Ctx         context.Context
	// YAMLMode indicates the project uses YAML orchestration (no compose file).
	// When true, container operations use naming.Container() directly.
	YAMLMode bool
}

// Model is the Bubble Tea model for the dashboard.
type Model struct {
	config    Config
	services  []ServiceRow
	selected  int
	logs      map[string][]string
	view      ViewMode
	width     int
	height    int
	statusMsg string // transient status message (e.g., "Restarting api...")
	proxyUp   bool
	quitting  bool
}

// New creates a new dashboard Model from config.
func New(cfg Config) Model {
	logs := make(map[string][]string)
	for _, svc := range cfg.Services {
		logs[svc.Name] = nil
	}

	return Model{
		config:   cfg,
		services: cfg.Services,
		logs:     logs,
	}
}

// Init starts background subscriptions.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		m.checkProxyCmd(),
	)
}

// SelectedService returns the name of the currently selected service.
func (m Model) SelectedService() string {
	if m.selected < 0 || m.selected >= len(m.services) {
		return ""
	}
	return m.services[m.selected].Name
}

func (m *Model) addLogLine(service, line string) {
	lines := m.logs[service]
	max := maxLogLinesInactive
	if service == m.SelectedService() {
		max = maxLogLines
	}
	lines = append(lines, line)
	if len(lines) > max {
		lines = lines[len(lines)-max:]
	}
	m.logs[service] = lines
}

func (m *Model) selectNext() {
	if m.selected < len(m.services)-1 {
		m.selected++
	}
}

func (m *Model) selectPrev() {
	if m.selected > 0 {
		m.selected--
	}
}

func (m *Model) updateStats(stats map[string]ServiceStats) {
	for i, svc := range m.services {
		if s, ok := stats[svc.Name]; ok {
			m.services[i].CPU = s.CPU
			m.services[i].Memory = s.Memory
			if s.Status != "" {
				m.services[i].Status = s.Status
			}
			if s.Uptime != "" {
				m.services[i].Uptime = s.Uptime
			}
		}
	}
}
