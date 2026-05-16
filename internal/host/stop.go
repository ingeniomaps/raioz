package host

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	exectimeout "raioz/internal/exec"
	"raioz/internal/logging"
)

// stopShutdownDeadline bounds how long Stop waits for the tracked PID
// to disappear after KillProcessTree fires before escalating to
// ForceKillProcessTree. Long enough for a well-behaved server to flush;
// short enough that callers waiting on the port don't hang.
const stopShutdownDeadline = 5 * time.Second

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

	if pid <= 0 {
		return nil
	}
	return killTrackedProcess(ctx, pid)
}

// killTrackedProcess terminates a host service started by raioz.
// HostRunner.Start always calls SetNewProcessGroup, so the tracked PID
// is the leader of its own group and KillProcessTree reaches every
// descendant via `kill -PGID`. A direct SIGTERM to the PID covers the
// legacy / test case where Setpgid wasn't applied (kill(-pid) is a
// no-op when no group with that PGID exists). Polls until the leader
// is reaped or stopShutdownDeadline expires, then escalates to
// ForceKillProcessTree + direct os.Process.Kill as last resort.
//
// Before the v0.8.3 fix this function sent SIGTERM only to the lone
// PID and let process.Wait() handle the barrier — but Wait returns
// ECHILD immediately on a non-child PID, so the deadline was never
// reached AND grandchildren like next-server / vite survived holding
// the listening port.
func killTrackedProcess(ctx context.Context, pid int) error {
	_ = KillProcessTree(pid)
	proc, _ := os.FindProcess(pid)
	if proc != nil {
		_ = proc.Signal(syscall.SIGTERM)
		// Reap in the background. If we're the parent (typical in
		// tests) this clears the zombie so IsProcessAlive below sees
		// the PID truly gone. If not the parent, Wait returns ECHILD
		// immediately (or blocks via pidfd until the process exits)
		// — IsProcessAlive drives the deadline unchanged.
		go func() { _, _ = proc.Wait() }()
	}

	deadline := time.Now().Add(stopShutdownDeadline)
	for IsProcessAlive(pid) {
		if !time.Now().Before(deadline) {
			_ = ForceKillProcessTree(pid)
			if proc != nil {
				_ = proc.Kill()
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return nil
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
