package tui

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// restartServiceCmd restarts a service via docker compose.
func (m Model) restartServiceCmd(serviceName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.config.Ctx, 30*time.Second)
		defer cancel()

		err := m.config.Docker.RestartServicesWithContext(ctx, m.config.ComposePath, []string{serviceName})
		return ActionResultMsg{
			Service: serviceName,
			Action:  "restart",
			Err:     err,
		}
	}
}

// stopServiceCmd stops a service via docker compose.
func (m Model) stopServiceCmd(serviceName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.config.Ctx, 30*time.Second)
		defer cancel()

		err := m.config.Docker.StopServiceWithContext(ctx, m.config.ComposePath, serviceName)
		return ActionResultMsg{
			Service: serviceName,
			Action:  "stop",
			Err:     err,
		}
	}
}

// execInServiceCmd opens an interactive shell in a container.
// Uses tea.ExecProcess to suspend the TUI during the shell session.
func (m Model) execInServiceCmd(serviceName string) tea.Cmd {
	c := exec.Command("docker", "compose", "-f", m.config.ComposePath,
		"exec", serviceName, "sh")
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return ActionResultMsg{
			Service: serviceName,
			Action:  "exec",
			Err:     err,
		}
	})
}

// formatActionResult returns a human-readable message for an action result.
func formatActionResult(msg ActionResultMsg) string {
	if msg.Err != nil {
		return fmt.Sprintf("%s %s failed: %s", msg.Action, msg.Service, msg.Err)
	}
	return fmt.Sprintf("%s %s: done", msg.Action, msg.Service)
}
