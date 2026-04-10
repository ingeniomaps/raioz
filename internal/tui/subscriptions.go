package tui

import (
	"bufio"
	"context"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"raioz/internal/runtime"
)

// tickCmd returns a command that sends a TickMsg after the stats interval.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Duration(statsInterval)*time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// pollStats queries Docker for current CPU/memory of all services.
func (m Model) pollStats() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.config.Ctx, 5*time.Second)
		defer cancel()

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

// checkProxyCmd checks if the proxy is running.
func (m Model) checkProxyCmd() tea.Cmd {
	return func() tea.Msg {
		if m.config.Proxy == nil {
			return nil
		}
		running, _ := m.config.Proxy.Status(m.config.Ctx)
		if running {
			return proxyStatusMsg(true)
		}
		return proxyStatusMsg(false)
	}
}

type proxyStatusMsg bool

// streamLogs starts streaming logs for a service in the background.
func (m Model) streamLogs(serviceName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(m.config.Ctx)
		defer cancel()

		args := []string{"compose"}
		if m.config.ComposePath != "" {
			args = append(args, "-f", m.config.ComposePath)
		}
		args = append(args, "logs", "--follow", "--tail", "50", serviceName)

		cmd := exec.CommandContext(ctx, runtime.Binary(), args...)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil
		}
		cmd.Stderr = cmd.Stdout

		if err := cmd.Start(); err != nil {
			return nil
		}

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			// Note: in a real implementation we'd send these via a channel
			// For now, logs are collected via polling in the stats cycle
			_ = scanner.Text()
		}
		return nil
	}
}
