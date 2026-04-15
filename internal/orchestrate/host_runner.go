package orchestrate

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"raioz/internal/domain/interfaces"
	"raioz/internal/logging"
	"raioz/internal/naming"
)

// HostRunner handles services that run directly on the host (npm, go, make, python, rust).
type HostRunner struct {
	// processes tracks running host process PIDs by service name
	processes map[string]int
}

// Start runs the detected start command in the service directory.
func (r *HostRunner) Start(ctx context.Context, svc interfaces.ServiceContext) error {
	if r.processes == nil {
		r.processes = make(map[string]int)
	}

	command := svc.Detection.DevCommand
	if command == "" {
		command = svc.Detection.StartCommand
	}
	if command == "" {
		return fmt.Errorf(
			"no start command detected for '%s'. "+
				"Add a dev script to package.json, "+
				"a dev target to Makefile, "+
				"or specify the command in raioz.yaml",
			svc.Name,
		)
	}

	logging.InfoWithContext(ctx, "Starting host service",
		"service", svc.Name, "command", command, "path", svc.Path)

	parts := strings.Fields(command)
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = svc.Path

	// Merge env vars with current environment
	cmd.Env = os.Environ()
	for k, v := range svc.EnvVars {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	// Redirect output to log file (persists after raioz up exits)
	logDir := naming.LogDir(svc.ProjectName)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log dir: %w", err)
	}

	logFile, err := os.Create(naming.LogFile(svc.ProjectName, svc.Name))
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Use SysProcAttr to detach so the child inherits the file descriptor
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Start in background
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start service '%s': %w", svc.Name, err)
	}

	r.processes[svc.Name] = cmd.Process.Pid
	// Don't close logFile — the child process owns the fd now
	// Go's finalizer will close it when the process is collected

	// Reap the child in the background. Without this, killing the process
	// leaves a zombie in the process table: kill(pid, 0) still reports
	// success for zombies, which would make Stop()'s polling loop wait for
	// the full 5s deadline on every tear-down. The goroutine exits as soon
	// as the process actually dies — whether naturally or via Stop().
	go func() {
		_ = cmd.Wait()
	}()

	logging.InfoWithContext(ctx, "Host service started",
		"service", svc.Name, "pid", cmd.Process.Pid)

	return nil
}

// Stop tears down the host service.
//
// When the service declares a custom stop command (`stop:` in raioz.yaml),
// that command is executed on the host with the same env vars as Start. This
// is required for commands like `make start` that spawn docker containers —
// killing the parent PID would orphan the containers.
//
// Otherwise, falls back to SIGTERM-then-SIGKILL of the tracked PID.
func (r *HostRunner) Stop(ctx context.Context, svc interfaces.ServiceContext) error {
	// Custom stop command path
	if svc.StopCommand != "" {
		logging.InfoWithContext(ctx, "Running custom stop command",
			"service", svc.Name, "command", svc.StopCommand, "path", svc.Path)

		parts := strings.Fields(svc.StopCommand)
		cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
		cmd.Dir = svc.Path

		cmd.Env = os.Environ()
		for k, v := range svc.EnvVars {
			cmd.Env = append(cmd.Env, k+"="+v)
		}

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("stop command failed for service '%s': %w", svc.Name, err)
		}

		// Also drop the PID bookkeeping if we have one, so subsequent status
		// checks don't point at a dead process.
		if r.processes != nil {
			delete(r.processes, svc.Name)
		}
		return nil
	}

	// PID-based fallback. We kill the whole process group (Setpgid is set in
	// Start), not just the parent PID, because the parent is typically a
	// `sh -c` wrapper whose grandchild (node, go run, etc.) is the real
	// server holding the port. Signalling the group ensures the grandchild
	// actually receives the signal; otherwise the subsequent Start() races
	// the still-alive grandchild and loses with EADDRINUSE.
	if r.processes == nil {
		return nil
	}

	pid, ok := r.processes[svc.Name]
	if !ok || pid == 0 {
		return nil
	}
	defer delete(r.processes, svc.Name)

	// SIGTERM to the entire group. If the process is already gone, Kill
	// returns ESRCH — treat as success.
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		return nil
	}

	// Poll for actual death up to 5s. We use Signal(0) on the group leader
	// PID, which returns ESRCH once the process is reaped. This is what
	// makes Stop() a real barrier: callers (including Restart) can trust
	// that when it returns, the port the service was holding is free.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err != nil {
			return nil // process is gone
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Still alive after 5s → SIGKILL the group. Best-effort.
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	return nil
}

// Restart stops and starts the service.
func (r *HostRunner) Restart(ctx context.Context, svc interfaces.ServiceContext) error {
	// Best-effort: Stop errors are non-fatal because Start below will
	// surface a real problem (port conflict, spawn failure) anyway.
	_ = r.Stop(ctx, svc)
	return r.Start(ctx, svc)
}

// Status checks if the host process is still running.
func (r *HostRunner) Status(_ context.Context, svc interfaces.ServiceContext) (string, error) {
	if r.processes == nil {
		return "stopped", nil
	}

	pid, ok := r.processes[svc.Name]
	if !ok || pid == 0 {
		return "stopped", nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return "stopped", nil
	}

	if err := process.Signal(syscall.Signal(0)); err != nil {
		return "stopped", nil
	}

	return "running", nil
}

// Logs tails the log file for the host service.
func (r *HostRunner) Logs(ctx context.Context, svc interfaces.ServiceContext, follow bool, tail int) error {
	logPath := naming.LogFile(svc.ProjectName, svc.Name)

	args := []string{}
	if tail > 0 {
		args = append(args, "-n", fmt.Sprintf("%d", tail))
	}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, logPath)

	cmd := exec.CommandContext(ctx, "tail", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// GetPID returns the PID of a running host service, or 0 if not tracked.
func (r *HostRunner) GetPID(serviceName string) int {
	if r.processes == nil {
		return 0
	}
	return r.processes[serviceName]
}

// SetPID records a PID for a service (for restoring state).
func (r *HostRunner) SetPID(serviceName string, pid int) {
	if r.processes == nil {
		r.processes = make(map[string]int)
	}
	r.processes[serviceName] = pid
}
