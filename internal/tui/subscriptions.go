package tui

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"raioz/internal/host"
	"raioz/internal/naming"
	"raioz/internal/runtime"
	"raioz/internal/state"

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

		return m.queryStats(ctx)
	}
}

// queryStats reads the live state of every row.
//
// Containers are found by raioz label, not by a guessed name: one
// `docker ps -a` answers for all of them, whatever each is called, and a
// row with no container simply has no line. (Inspecting a fixed list of
// names made docker exit non-zero as soon as one did not exist — which is
// every host service — and the whole answer was thrown away.)
//
// A host service has no container; it is running when its recorded PID is.
func (m Model) queryStats(ctx context.Context) StatsMsg {
	stats := make(map[string]ServiceStats)
	if len(m.services) == 0 {
		return StatsMsg{Stats: stats}
	}

	hostPIDs := map[string]int{}
	if m.config.ProjectDir != "" {
		if ls, err := state.LoadLocalState(m.config.ProjectDir); err == nil && ls != nil {
			hostPIDs = ls.HostPIDs
		}
	}
	for _, svc := range m.services {
		st := ServiceStats{Status: "stopped", CPU: "-", Memory: "-"}
		if svc.Host {
			if pid := hostPIDs[svc.Name]; pid > 0 && host.IsProcessAlive(pid) {
				st.Status = "running"
			}
		}
		stats[svc.Name] = st
	}

	format := "{{.Names}}|" +
		"{{.Label \"" + naming.LabelProject + "\"}}|" +
		"{{.Label \"" + naming.LabelWorkspace + "\"}}|" +
		"{{.Label \"" + naming.LabelService + "\"}}|" +
		"{{.State}}|{{.Status}}"
	out, _ := exec.CommandContext(ctx, runtime.Binary(), "ps", "-a",
		"--filter", "label="+naming.LabelManaged+"=true",
		"--format", format).Output()

	containerToSvc := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		name, service, st, ok := m.parseContainerLine(line)
		if !ok {
			continue
		}
		if _, known := stats[service]; !known {
			continue
		}
		st.CPU, st.Memory = "-", "-"
		stats[service] = st
		if st.Status == "running" {
			containerToSvc[name] = service
		}
	}

	if len(containerToSvc) == 0 {
		return StatsMsg{Stats: stats}
	}
	statsArgs := []string{"stats", "--no-stream", "--format", "{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}"}
	for name := range containerToSvc {
		statsArgs = append(statsArgs, name)
	}
	if out, err := exec.CommandContext(ctx, runtime.Binary(), statsArgs...).Output(); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			sp := strings.Split(line, "\t")
			if len(sp) < 3 {
				continue
			}
			svc, ok := containerToSvc[strings.TrimPrefix(sp[0], "/")]
			if !ok {
				continue
			}
			s := stats[svc]
			s.CPU, s.Memory = sp[1], sp[2]
			stats[svc] = s
		}
	}

	return StatsMsg{Stats: stats}
}

// parseContainerLine reads one `docker ps` line and reports the container,
// the service it runs and its state — when it belongs to this project: it
// carries the project's label, or it is a workspace-shared dependency
// (no project label, same workspace).
func (m Model) parseContainerLine(line string) (container, service string, st ServiceStats, ok bool) {
	f := strings.SplitN(line, "|", 6)
	if len(f) < 6 {
		return "", "", ServiceStats{}, false
	}
	project, workspace := f[1], f[2]
	mine := project == m.config.Project ||
		(project == "" && workspace != "" && workspace == m.config.Workspace)
	if !mine || f[3] == "" {
		return "", "", ServiceStats{}, false
	}
	st = ServiceStats{Status: f[4], Container: f[0]}
	switch {
	case strings.Contains(f[5], "(healthy)"):
		st.Health = "healthy"
	case strings.Contains(f[5], "(unhealthy)"):
		st.Health = "unhealthy"
	}
	return f[0], f[3], st, true
}

// logTailLines is how much of a service's log the dashboard shows.
const logTailLines = 200

// pollLogs fetches the log tail of the selected service: the file the host
// runner writes for a host service, `docker logs` for a container.
func (m Model) pollLogs() tea.Cmd {
	row, ok := m.selectedRow()
	if !ok {
		return nil
	}
	project := m.config.Project
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.config.Ctx, 5*time.Second)
		defer cancel()

		var raw []byte
		switch {
		case row.Host:
			raw, _ = os.ReadFile(naming.LogFile(project, row.Name))
		case row.Container != "":
			raw, _ = exec.CommandContext(ctx, runtime.Binary(), "logs",
				"--tail", strconv.Itoa(logTailLines), row.Container).CombinedOutput()
		}
		return LogsMsg{Service: row.Name, Lines: tailLines(string(raw), logTailLines)}
	}
}

// tailLines returns the last n non-empty lines of text.
func tailLines(text string, n int) []string {
	var lines []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
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
