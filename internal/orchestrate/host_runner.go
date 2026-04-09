package orchestrate

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"raioz/internal/domain/interfaces"
	"raioz/internal/logging"
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
		return fmt.Errorf("no start command detected for '%s'. Add a dev script to package.json, a dev target to Makefile, or specify the command in raioz.yaml", svc.Name)
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

	// Log output to files
	logDir := filepath.Join(os.TempDir(), "raioz-orchestrate", "logs")
	os.MkdirAll(logDir, 0755)

	logFile, err := os.Create(filepath.Join(logDir, svc.Name+".log"))
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	cmd.Stdout = io.MultiWriter(os.Stdout, logFile)
	cmd.Stderr = io.MultiWriter(os.Stderr, logFile)

	// Start in background
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start service '%s': %w", svc.Name, err)
	}

	r.processes[svc.Name] = cmd.Process.Pid
	logFile.Close()

	logging.InfoWithContext(ctx, "Host service started",
		"service", svc.Name, "pid", cmd.Process.Pid)

	return nil
}

// Stop kills the host process.
func (r *HostRunner) Stop(ctx context.Context, svc interfaces.ServiceContext) error {
	if r.processes == nil {
		return nil
	}

	pid, ok := r.processes[svc.Name]
	if !ok || pid == 0 {
		return nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		delete(r.processes, svc.Name)
		return nil
	}

	// Try SIGTERM first
	if err := process.Signal(syscall.SIGTERM); err != nil {
		delete(r.processes, svc.Name)
		return nil
	}

	// Wait up to 5 seconds for graceful shutdown
	done := make(chan error, 1)
	go func() { _, err := process.Wait(); done <- err }()

	select {
	case <-time.After(5 * time.Second):
		process.Kill()
	case <-done:
	}

	delete(r.processes, svc.Name)
	return nil
}

// Restart stops and starts the service.
func (r *HostRunner) Restart(ctx context.Context, svc interfaces.ServiceContext) error {
	r.Stop(ctx, svc)
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
	logPath := filepath.Join(os.TempDir(), "raioz", "logs", svc.Name+".log")

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
