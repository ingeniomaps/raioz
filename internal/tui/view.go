package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the entire dashboard.
func (m Model) View() string {
	if m.quitting {
		return "Services are still running. Use 'raioz down' to stop.\n"
	}

	if m.width < 60 || m.height < 15 {
		return "Terminal too small. Resize to at least 60x15.\n"
	}

	var sections []string

	sections = append(sections, m.renderHeader())
	sections = append(sections, m.renderTable())
	sections = append(sections, m.renderLogs())
	sections = append(sections, m.renderFooter())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) renderHeader() string {
	title := m.config.Project
	if m.config.Workspace != "" {
		title = m.config.Workspace + " / " + m.config.Project
	}
	return headerStyle.Render(title) + "\n"
}

func (m Model) renderTable() string {
	var b strings.Builder

	// Header row
	header := fmt.Sprintf("  %-18s %-10s %-9s %-6s %-8s %s",
		"SERVICE", "RUNTIME", "STATUS", "CPU", "MEM", "URL")
	b.WriteString(tableHeaderStyle.Render(header))
	b.WriteString("\n")

	// Service rows
	for i, svc := range m.services {
		icon := statusIcon(svc.Status)
		url := svc.URL
		if url == "" {
			url = "-"
		}
		if len(url) > 20 {
			url = url[:20]
		}

		row := fmt.Sprintf("%s %-18s %-10s %-9s %-6s %-8s %s",
			icon, svc.Name, svc.Runtime, svc.Status,
			svc.CPU, svc.Memory, url)

		if i == m.selected {
			b.WriteString(selectedRowStyle.Render(">" + row))
		} else {
			b.WriteString(normalRowStyle.Render(" " + row))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderLogs() string {
	selected := m.SelectedService()
	if selected == "" {
		return ""
	}

	title := logTitleStyle.Render(fmt.Sprintf("─── Logs (%s) ", selected))

	// Calculate available height for logs
	logHeight := m.height - len(m.services) - 8 // header + table + footer + padding
	if m.view == ViewLogsExpanded {
		logHeight = m.height - 4
	}
	if logHeight < 3 {
		logHeight = 3
	}

	lines := m.logs[selected]
	if len(lines) == 0 {
		lines = []string{"  (no logs yet)"}
	}

	// Show last N lines that fit
	start := 0
	if len(lines) > logHeight {
		start = len(lines) - logHeight
	}
	visible := lines[start:]

	// Truncate long lines
	maxWidth := m.width - 4
	if maxWidth < 20 {
		maxWidth = 20
	}
	var trimmed []string
	for _, line := range visible {
		if len(line) > maxWidth {
			line = line[:maxWidth-3] + "..."
		}
		trimmed = append(trimmed, line)
	}

	content := strings.Join(trimmed, "\n")
	return logBorderStyle.Render(title + "\n" + content)
}

func (m Model) renderFooter() string {
	var parts []string

	// Proxy status
	if m.config.Proxy != nil {
		if m.proxyUp {
			parts = append(parts, proxyOnStyle.Render("PROXY: Caddy ✓"))
		} else {
			parts = append(parts, proxyOffStyle.Render("PROXY: off"))
		}
	}

	// Status message
	if m.statusMsg != "" {
		parts = append(parts, m.statusMsg)
	}

	// Key hints
	keys := []string{
		footerKeyStyle.Render("[r]") + "estart",
		footerKeyStyle.Render("[s]") + "top",
		footerKeyStyle.Render("[l]") + "ogs",
		footerKeyStyle.Render("[e]") + "xec",
		footerKeyStyle.Render("[q]") + "uit",
	}
	parts = append(parts, strings.Join(keys, "  "))

	return footerStyle.Render(strings.Join(parts, "  │  "))
}
