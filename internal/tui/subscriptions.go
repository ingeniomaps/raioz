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

// pollStatsYAML queries stats for all services in two batched docker
// calls (pre-fix this forked N×2 subprocesses per 2s tick,
// which throttled on Docker Desktop macOS once N > ~10). One `docker
// inspect` covers status/health for all containers; one `docker stats
// --no-stream` covers CPU/mem. Services whose containers don't exist
// surface as "stopped" without spamming individual inspects.
func (m Model) pollStatsYAML(ctx context.Context) StatsMsg {
	stats := make(map[string]ServiceStats)
	if len(m.services) == 0 {
		return StatsMsg{Stats: stats}
	}

	// Map container name → service name so the batched output can be
	// distributed back without re-parsing.
	containers := make([]string, 0, len(m.services))
	containerToSvc := make(map[string]string, len(m.services))
	for _, svc := range m.services {
		c := naming.Container(m.config.Project, svc.Name)
		containers = append(containers, c)
		containerToSvc[c] = svc.Name
		stats[svc.Name] = ServiceStats{Status: "stopped"} // default if absent
	}

	// One batched inspect for status + health. Format includes the
	// container name so the output is self-describing.
	inspectArgs := append([]string{
		"inspect", "--format",
		"{{.Name}}|{{.State.Status}}|{{.State.Health.Status}}",
	}, containers...)
	if out, err := exec.CommandContext(
		ctx, runtime.Binary(), inspectArgs...,
	).Output(); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			parts := strings.SplitN(line, "|", 3)
			if len(parts) < 2 {
				continue
			}
			// docker inspect prefixes the container name with "/".
			name := strings.TrimPrefix(parts[0], "/")
			svc, ok := containerToSvc[name]
			if !ok {
				continue
			}
			s := stats[svc]
			s.Status = parts[1]
			if len(parts) > 2 && parts[2] != "" {
				s.Health = parts[2]
			}
			stats[svc] = s
		}
	}

	// One batched `stats --no-stream` for CPU/mem. Skipped containers
	// keep the default "-" values.
	for svc := range stats {
		s := stats[svc]
		if s.CPU == "" {
			s.CPU = "-"
		}
		if s.Memory == "" {
			s.Memory = "-"
		}
		stats[svc] = s
	}
	statsArgs := append([]string{
		"stats", "--no-stream",
		"--format", "{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}",
	}, containers...)
	if out, err := exec.CommandContext(
		ctx, runtime.Binary(), statsArgs...,
	).Output(); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			sp := strings.Split(line, "\t")
			if len(sp) < 3 {
				continue
			}
			name := strings.TrimPrefix(sp[0], "/")
			svc, ok := containerToSvc[name]
			if !ok {
				continue
			}
			s := stats[svc]
			s.CPU = sp[1]
			s.Memory = sp[2]
			stats[svc] = s
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
