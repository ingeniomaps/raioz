package tui

import (
	"context"
	"errors"
	"os/exec"
	"time"

	"raioz/internal/i18n"
	"raioz/internal/runtime"

	tea "github.com/charmbracelet/bubbletea"
)

// restartServiceCmd restarts a service.
func (m Model) restartServiceCmd(serviceName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.config.Ctx, 30*time.Second)
		defer cancel()

		container, err := m.containerOf(serviceName)
		if err == nil {
			err = exec.CommandContext(ctx, runtime.Binary(), "restart", container).Run()
		}
		return ActionResultMsg{
			Service: serviceName,
			Action:  "restart",
			Err:     err,
		}
	}
}

// stopServiceCmd stops a service.
func (m Model) stopServiceCmd(serviceName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.config.Ctx, 30*time.Second)
		defer cancel()

		container, err := m.containerOf(serviceName)
		if err == nil {
			err = exec.CommandContext(ctx, runtime.Binary(), "stop", container).Run()
		}
		return ActionResultMsg{
			Service: serviceName,
			Action:  "stop",
			Err:     err,
		}
	}
}

// execInServiceCmd opens an interactive shell in a container.
func (m Model) execInServiceCmd(serviceName string) tea.Cmd {
	container, err := m.containerOf(serviceName)
	if err != nil {
		return func() tea.Msg {
			return ActionResultMsg{Service: serviceName, Action: "exec", Err: err}
		}
	}
	c := exec.Command(runtime.Binary(), "exec", "-it", container, "sh")
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
		return i18n.T("dashboard.action_failed", msg.Action, msg.Service, msg.Err)
	}
	return i18n.T("dashboard.action_done", msg.Action, msg.Service)
}

// containerOf returns the live container of a row. The dashboard acts on
// containers only (ADR-044); a host service, or a row with nothing
// running, has none to act on.
func (m Model) containerOf(serviceName string) (string, error) {
	row, ok := m.row(serviceName)
	if !ok || row.Container == "" {
		return "", errors.New(i18n.T("dashboard.host_service_no_container"))
	}
	return row.Container, nil
}
