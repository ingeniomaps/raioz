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
	// Host marks a service raioz runs as a host process: there is no
	// container, its status comes from the recorded PID and its logs from
	// the file the host runner writes.
	Host bool
	// Container is the live container behind the row, as resolved on the
	// last poll. Empty for host services and for anything not running.
	Container string
}

// Config holds everything the TUI needs to start.
//
// There is one mode. The dashboard used to branch on YAMLMode and reach
// containers through a compose file, but no caller ever set ComposePath
// and AutoDetect reports SourceFormatYAML, so the compose half ran with
// an empty path and produced nothing. Containers are addressed by
// naming.Container() throughout.
type Config struct {
	Project   string
	Workspace string
	Services  []ServiceRow
	Proxy     interfaces.ProxyManager
	Ctx       context.Context
	// ProjectDir is where the project's local state lives; host service
	// PIDs are read from it.
	ProjectDir string
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
	// Poll once right away: waiting for the first tick left every row on
	// "unknown" for a full interval.
	return tea.Batch(
		m.pollStats(),
		m.pollLogs(),
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
	limit := maxLogLinesInactive
	if service == m.SelectedService() {
		limit = maxLogLines
	}
	lines = append(lines, line)
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
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
			m.services[i].Container = s.Container
			if s.Uptime != "" {
				m.services[i].Uptime = s.Uptime
			}
		}
	}
}

// selectedRow returns the selected service row, false when there is none.
func (m Model) selectedRow() (ServiceRow, bool) {
	if m.selected < 0 || m.selected >= len(m.services) {
		return ServiceRow{}, false
	}
	return m.services[m.selected], true
}

// row returns the row of the named service.
func (m Model) row(name string) (ServiceRow, bool) {
	for _, svc := range m.services {
		if svc.Name == name {
			return svc, true
		}
	}
	return ServiceRow{}, false
}
