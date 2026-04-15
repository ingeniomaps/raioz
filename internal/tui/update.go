package tui

import tea "github.com/charmbracelet/bubbletea"

// Update handles all messages and returns the updated model + commands.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case TickMsg:
		return m, tea.Batch(m.pollStats(), tickCmd())

	case StatsMsg:
		m.updateStats(msg.Stats)
		return m, nil

	case LogMsg:
		m.addLogLine(msg.Service, msg.Line)
		return m, nil

	case ActionResultMsg:
		m.statusMsg = formatActionResult(msg)
		return m, nil

	case proxyStatusMsg:
		m.proxyUp = bool(msg)
		return m, nil
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// In expanded logs mode, Esc returns to normal
	if m.view == ViewLogsExpanded {
		switch msg.String() {
		case "esc", "l":
			m.view = ViewNormal
			return m, nil
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	}

	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "up", "k":
		m.selectPrev()
		return m, nil

	case "down", "j":
		m.selectNext()
		return m, nil

	case "r":
		svc := m.SelectedService()
		if svc != "" {
			m.statusMsg = "Restarting " + svc + "..."
			return m, m.restartServiceCmd(svc)
		}

	case "s":
		svc := m.SelectedService()
		if svc != "" {
			m.statusMsg = "Stopping " + svc + "..."
			return m, m.stopServiceCmd(svc)
		}

	case "l":
		m.view = ViewLogsExpanded
		return m, nil

	case "e":
		svc := m.SelectedService()
		if svc != "" {
			return m, m.execInServiceCmd(svc)
		}
	}

	return m, nil
}
