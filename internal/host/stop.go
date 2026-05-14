package host

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	exectimeout "raioz/internal/exec"
	"raioz/internal/logging"
)

// StopService stops a running host process by PID
func StopService(ctx context.Context, pid int) error {
	return StopServiceWithCommandAndPath(ctx, pid, "", "")
}

// StopServiceWithCommand stops a running host process, optionally using a custom stop command first
// Deprecated: Use StopServiceWithCommandAndPath instead
func StopServiceWithCommand(ctx context.Context, pid int, stopCommand string) error {
	return StopServiceWithCommandAndPath(ctx, pid, stopCommand, "")
}

// StopServiceWithCommandAndPath stops a running host process, optionally using a custom stop command first
// servicePath is the directory where the stop command should be executed
func StopServiceWithCommandAndPath(ctx context.Context, pid int, stopCommand string, servicePath string) error {
	// If a custom stop command is provided, execute it first
	if stopCommand != "" {
		cmdParts := parseCommand(stopCommand)
		if len(cmdParts) > 0 {
			// Use a longer timeout for stop commands (60 seconds)
			stopCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, 60*time.Second)
			defer cancel()

			var cmd *exec.Cmd
			if len(cmdParts) == 1 {
				cmd = exec.CommandContext(stopCtx, cmdParts[0])
			} else {
				cmd = exec.CommandContext(stopCtx, cmdParts[0], cmdParts[1:]...)
			}

			// Set working directory if service path is provided
			if servicePath != "" {
				cmd.Dir = servicePath
				logging.DebugWithContext(
					ctx, "Executing stop command in service directory",
					"stopCommand", stopCommand,
					"servicePath", servicePath, "pid", pid,
				)
			} else {
				logging.DebugWithContext(
					ctx, "Executing stop command",
					"stopCommand", stopCommand, "pid", pid,
				)
			}

			// Show output in console for stop commands (they are always synchronous)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			// Execute stop command and wait for completion
			// Stop commands like "make stop" should complete before continuing
			if err := cmd.Run(); err != nil {
				// Log but don't fail - fall back to PID kill
				logging.WarnWithContext(
					ctx, "Stop command failed, falling back to PID kill",
					"error", err.Error(), "stopCommand", stopCommand,
				)
			} else {
				// Stop command completed successfully
				logging.InfoWithContext(
					ctx, "Stop command completed successfully",
					"stopCommand", stopCommand,
				)
				// Check if process is still running (only if we have a valid PID)
				if pid > 0 {
					if running, _ := IsServiceRunning(pid); !running {
						// Process already stopped by the command
						return nil
					}
				} else {
					// No PID to track, command completed successfully
					return nil
				}
			}
		}
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	// Try graceful shutdown first (SIGTERM)
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// Process might already be dead
		if err.Error() == "os: process already finished" {
			return nil
		}
		return fmt.Errorf("failed to send SIGTERM to process %d: %w", pid, err)
	}

	// Wait a bit for graceful shutdown
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := process.Wait()
		done <- err
	}()

	select {
	case <-timeoutCtx.Done():
		// Timeout: force kill
		if err := process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process %d: %w", pid, err)
		}
		return nil
	case err := <-done:
		// "no child processes" / "wait: no child processes" means the
		// child was already reaped — typically by the settle-window
		// goroutine in StartService. The process is gone,
		// which is exactly what we wanted.
		if err != nil && strings.Contains(err.Error(), "no child process") {
			return nil
		}
		return err
	}
}

// IsServiceRunning checks if a process is still running
func IsServiceRunning(pid int) (bool, error) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, fmt.Errorf("find process %d: %w", pid, err)
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	if err == nil {
		return true, nil
	}
	if err.Error() == "os: process already finished" {
		return false, nil
	}
	return false, fmt.Errorf("probe process %d: %w", pid, err)
}
