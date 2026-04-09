package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	colorPrimary   = lipgloss.Color("39")  // blue
	colorSuccess   = lipgloss.Color("42")  // green
	colorWarning   = lipgloss.Color("214") // orange
	colorError     = lipgloss.Color("196") // red
	colorMuted     = lipgloss.Color("241") // gray
	colorHighlight = lipgloss.Color("213") // pink
	colorWhite     = lipgloss.Color("15")

	// Header
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			PaddingLeft(1)

	// Table
	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorMuted).
				PaddingLeft(2)

	selectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorHighlight).
				PaddingLeft(1)

	normalRowStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	// Status indicators
	statusRunning = lipgloss.NewStyle().Foreground(colorSuccess).Render("●")
	statusStopped = lipgloss.NewStyle().Foreground(colorError).Render("○")
	statusUnknown = lipgloss.NewStyle().Foreground(colorWarning).Render("◌")

	// Logs panel
	logBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(colorMuted).
			PaddingLeft(1).
			MarginTop(1)

	logTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorMuted)

	// Footer
	footerStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			PaddingLeft(1).
			MarginTop(1)

	footerKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite)

	// Proxy indicator
	proxyOnStyle  = lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)
	proxyOffStyle = lipgloss.NewStyle().Foreground(colorError)
)

func statusIcon(status string) string {
	switch status {
	case "running", "healthy", "ready":
		return statusRunning
	case "stopped", "exited":
		return statusStopped
	default:
		return statusUnknown
	}
}
