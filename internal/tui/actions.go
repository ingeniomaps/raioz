package tui

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"raioz/internal/naming"
	"raioz/internal/runtime"

	tea "github.com/charmbracelet/bubbletea"
)

// restartServiceCmd restarts a service.
func (m Model) restartServiceCmd(serviceName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.config.Ctx, 30*time.Second)
		defer cancel()

		var err error
		if m.config.YAMLMode {
			container := naming.Container(m.config.Project, serviceName)
			cmd := exec.CommandContext(ctx, runtime.Binary(), "restart", container)
			err = cmd.Run()
		} else {
			err = m.config.Docker.RestartServicesWithContext(
				ctx, m.config.ComposePath, []string{serviceName},
			)
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

		var err error
		if m.config.YAMLMode {
			container := naming.Container(m.config.Project, serviceName)
			cmd := exec.CommandContext(ctx, runtime.Binary(), "stop", container)
			err = cmd.Run()
		} else {
			err = m.config.Docker.StopServiceWithContext(
				ctx, m.config.ComposePath, serviceName,
			)
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
	var c *exec.Cmd
	if m.config.YAMLMode {
		container := naming.Container(m.config.Project, serviceName)
		c = exec.Command(runtime.Binary(), "exec", "-it", container, "sh")
	} else {
		c = exec.Command(
			runtime.Binary(), "compose", "-f", m.config.ComposePath,
			"exec", serviceName, "sh",
		)
	}
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
