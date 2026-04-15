package tui

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"raioz/internal/naming"
	"raioz/internal/runtime"

	tea "github.com/charmbracelet/bubbletea"
)

// tickCmd returns a command that sends a TickMsg after the stats interval.
func tickCmd() tea.Cmd {
	return tea.Tick(
		time.Duration(statsInterval)*time.Second,
		func(t time.Time) tea.Msg { return TickMsg(t) },
	)
}

// pollStats queries Docker for current CPU/memory of all services.
func (m Model) pollStats() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.config.Ctx, 5*time.Second)
		defer cancel()

		if m.config.YAMLMode {
			return m.pollStatsYAML(ctx)
		}

		info, err := m.config.Docker.GetServicesInfoWithContext(
			ctx, m.config.ComposePath, nil, m.config.Project, nil, nil,
		)
		if err != nil {
			return StatsMsg{Stats: map[string]ServiceStats{}}
		}

		stats := make(map[string]ServiceStats)
		for name, si := range info {
			stats[name] = ServiceStats{
				CPU:    si.CPU,
				Memory: si.Memory,
				Status: si.Status,
				Health: si.Health,
				Uptime: si.Uptime,
			}
		}
		return StatsMsg{Stats: stats}
	}
}

// pollStatsYAML queries stats per-container using naming.Container().
func (m Model) pollStatsYAML(ctx context.Context) StatsMsg {
	stats := make(map[string]ServiceStats)

	for _, svc := range m.services {
		container := naming.Container(m.config.Project, svc.Name)

		// Get status
		cmd := exec.CommandContext(
			ctx, runtime.Binary(),
			"inspect", "--format",
			"{{.State.Status}}|{{.State.Health.Status}}",
			container,
		)
		out, err := cmd.Output()
		if err != nil {
			stats[svc.Name] = ServiceStats{Status: "stopped"}
			continue
		}
		parts := strings.SplitN(strings.TrimSpace(string(out)), "|", 2)
		status := parts[0]
		health := ""
		if len(parts) > 1 && parts[1] != "" {
			health = parts[1]
		}

		// Get CPU/mem
		cmd2 := exec.CommandContext(
			ctx, runtime.Binary(),
			"stats", "--no-stream",
			"--format", "{{.CPUPerc}}\t{{.MemUsage}}",
			container,
		)
		cpu, mem := "-", "-"
		if out2, err2 := cmd2.Output(); err2 == nil {
			sp := strings.Split(strings.TrimSpace(string(out2)), "\t")
			if len(sp) >= 2 {
				cpu, mem = sp[0], sp[1]
			}
		}

		stats[svc.Name] = ServiceStats{
			Status: status,
			Health: health,
			CPU:    cpu,
			Memory: mem,
		}
	}

	return StatsMsg{Stats: stats}
}

// checkProxyCmd checks if the proxy is running.
func (m Model) checkProxyCmd() tea.Cmd {
	return func() tea.Msg {
		if m.config.Proxy == nil {
			return nil
		}
		running, _ := m.config.Proxy.Status(m.config.Ctx)
		return proxyStatusMsg(running)
	}
}

type proxyStatusMsg bool
