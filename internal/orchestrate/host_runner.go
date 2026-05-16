package orchestrate

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/host"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/output"
)

// register routes every script/native-runtime dispatch (npm, go, make,
// python, …) to HostRunner. Each runtime here corresponds to one
// runner-on-the-host execution path; adding a new host language means
// adding it to this list AND to models.AllRuntimes().
func init() {
	hostRuntimes := []models.Runtime{
		models.RuntimeNPM, models.RuntimeGo, models.RuntimeMake,
		models.RuntimeJust, models.RuntimeTask,
		models.RuntimePython, models.RuntimeRust, models.RuntimePHP,
		models.RuntimeJava, models.RuntimeDotnet, models.RuntimeRuby,
		models.RuntimeElixir, models.RuntimeDart, models.RuntimeSwift,
		models.RuntimeScala, models.RuntimeClojure, models.RuntimeZig,
		models.RuntimeGleam, models.RuntimeHaskell, models.RuntimeDeno,
		models.RuntimeBun,
	}
	for _, rt := range hostRuntimes {
		register(rt, func(d *Dispatcher) runner { return d.host })
	}
}

// HostRunner handles services that run directly on the host (npm, go, make, python, rust).
type HostRunner struct {
	// mu guards processes + launchers. All access goes through the
	// helpers below so the lock pattern stays uniform (ADR-028). Lazy
	// init: a fresh HostRunner has nil maps; the first helper that
	// touches them initializes under the write lock.
	mu        sync.Mutex
	processes map[string]int
	// Services that triggered the launcher pattern at Start time. Stop
	// drains an in-progress build before invoking stop: (ADR-025).
	launchers map[string]bool
}

// recordPID stores svcName→pid under the write lock. Lazy-inits the
// underlying map so a freshly constructed HostRunner doesn't need an
// explicit setup phase.
func (r *HostRunner) recordPID(svcName string, pid int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.processes == nil {
		r.processes = make(map[string]int)
	}
	r.processes[svcName] = pid
}

// markLauncher records that svcName was started via the launcher
// pattern (ADR-025 clean-exit-in-settle-window path).
func (r *HostRunner) markLauncher(svcName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.launchers == nil {
		r.launchers = make(map[string]bool)
	}
	r.launchers[svcName] = true
}

// isLauncher reports whether Stop should run the launcher drain
// (ADR-025) before invoking stop:.
func (r *HostRunner) isLauncher(svcName string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.launchers[svcName]
}

// takePID atomically reads and removes the recorded PID for svcName,
// returning (0, false) when no PID was tracked. Used by Stop to
// retrieve-and-clear in one critical section so a concurrent Start
// of the same service can't observe a half-cleaned state.
func (r *HostRunner) takePID(svcName string) (int, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pid, ok := r.processes[svcName]
	if ok {
		delete(r.processes, svcName)
	}
	return pid, ok
}

// peekPID is a read-only PID lookup. Used by Status and GetPID where
// the caller does not want side effects.
func (r *HostRunner) peekPID(svcName string) (int, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pid, ok := r.processes[svcName]
	return pid, ok
}

// Start runs the detected start command in the service directory.
//
// The child is spawned with plain exec.Command (NOT CommandContext) so
// it survives `raioz up` returning: the cobra signal context cancels on
// every clean exit (deferred stop() in cli/root.go), which would
// otherwise SIGKILL slow launchers like `make start` before their
// internal `docker compose up -d` finishes. SetNewProcessGroup keeps
// the child reachable by Stop later (via Kill(-pid)). SIGINT during
// the settle window is still handled — see the select below.
func (r *HostRunner) Start(ctx context.Context, svc interfaces.ServiceContext) error {
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
	// exec.Command (no ctx) by design — see Start's doc comment.
	cmd := exec.Command(parts[0], parts[1:]...)
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

	// Start in a new process group so Stop can kill the whole tree (the
	// parent is usually `sh -c` and the grandchild is the real server).
	// No-op on Windows — taskkill /T handles the tree without a group.
	host.SetNewProcessGroup(cmd)

	// Start in background
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start service '%s': %w", svc.Name, err)
	}

	// Detect processes that fork+exec ok but die immediately
	// (port already bound, missing binary, panic at boot). Same goroutine
	// also handles reaping after the settle window — the channel buffer
	// absorbs the eventual exit so we don't leak a zombie.
	//
	// The drain goroutine ALSO closes logFile once cmd exits. Without
	// that explicit close, the "still alive after the settle window"
	// path leaves the parent's copy of the fd open and a long watch-mode
	// session leaks one handle per Start until GC runs the finalizer.
	// Closing inside the goroutine guarantees release exactly once,
	// when the child actually dies — regardless of which select branch
	// fired above. See ADR-034.
	logPath := naming.LogFile(svc.ProjectName, svc.Name)
	waitCh := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		_ = logFile.Close()
		waitCh <- err
	}()

	if window := host.SettleWindow(); window > 0 {
		select {
		case <-ctx.Done():
			// SIGINT / SIGTERM during the settle window: tear down
			// the child explicitly (it isn't bound to ctx anymore).
			// Kill the whole process group so `sh -c` wrappers and
			// their make/docker-compose grandchildren all go.
			_ = host.KillProcessTree(cmd.Process.Pid)
			return fmt.Errorf(
				"host runner '%s' cancelled during settle window: %w",
				svc.Name, ctx.Err(),
			)
		case exitErr := <-waitCh:
			if exitErr == nil {
				// Clean exit 0 inside the window: the command was a
				// launcher that detached a long-running container (issue
				// 010 shape — `make dev-docker`, `./up.sh`). Continue;
				// the proxy.target / Status priority-0 logic in
				// status_host.go owns observability from here on.
				//
				// When the user has NOT declared `stop:`, raioz won't
				// be able to tear down whatever the launcher started —
				// the captured PID is already dead and we have no
				// label-based handle on the launched processes /
				// containers. Warn loudly so `raioz down` doesn't
				// silently leak resources.
				r.markLauncher(svc.Name)
				if svc.StopCommand == "" {
					output.PrintWarning(fmt.Sprintf(
						"Service '%s' exited 0 within the settle window — likely a "+
							"launcher that detached a container or daemon. Without "+
							"`stop:` declared, `raioz down` cannot clean up. Add a "+
							"`stop:` command (e.g. `make stop`) to raioz.yaml.",
						svc.Name))
					logging.WarnWithContext(ctx, "Launcher pattern without stop: declared",
						"service", svc.Name, "command", command)
				}
				// ADR-025: don't claim ready before the launcher's
				// container materializes. No-op without proxy.target:.
				waitForLauncherContainer(ctx, svc)
				break
			}
			// logFile already closed by the drain goroutine.
			return host.FormatEarlyExitError(svc.Name, window, exitErr, logPath)
		case <-time.After(window):
			// process is still alive — fall through. Drain goroutine
			// keeps running; logFile gets released when cmd exits.
		}
	}

	r.recordPID(svc.Name, cmd.Process.Pid)
	// logFile is owned by the drain goroutine; it closes the parent's
	// copy of the fd when cmd.Wait() returns. See ADR-034.

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
		// ADR-025: drain an in-progress launcher build before stop:.
		if r.isLauncher(svc.Name) {
			drainLauncherBeforeStop(ctx, svc)
		}
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

		// Drop the PID bookkeeping (if any) so subsequent status checks
		// don't point at a dead process.
		_, _ = r.takePID(svc.Name)
		return nil
	}

	// PID-based fallback. We kill the whole process group (Setpgid is set in
	// Start), not just the parent PID, because the parent is typically a
	// `sh -c` wrapper whose grandchild (node, go run, etc.) is the real
	// server holding the port. Signalling the group ensures the grandchild
	// actually receives the signal; otherwise the subsequent Start() races
	// the still-alive grandchild and loses with EADDRINUSE.
	pid, ok := r.takePID(svc.Name)
	if !ok || pid == 0 {
		return nil
	}

	// Graceful tree kill. Ignore errors from the KillProcessTree path —
	// the poll below is the real barrier.
	_ = host.KillProcessTree(pid)

	// Poll for actual death up to 5s so callers (Restart, down) can
	// trust the port is free once Stop returns.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !host.IsProcessAlive(pid) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Still alive after 5s → force kill. Best-effort.
	_ = host.ForceKillProcessTree(pid)
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
	pid, ok := r.peekPID(svc.Name)
	if !ok || pid == 0 {
		return "stopped", nil
	}

	if _, err := os.FindProcess(pid); err != nil {
		return "stopped", nil
	}

	if !host.IsProcessAlive(pid) {
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
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tail %q: %w", logPath, err)
	}
	return nil
}

// GetPID returns the PID of a running host service, or 0 if not tracked.
func (r *HostRunner) GetPID(serviceName string) int {
	pid, _ := r.peekPID(serviceName)
	return pid
}

// SetPID records a PID for a service (for restoring state).
func (r *HostRunner) SetPID(serviceName string, pid int) {
	r.recordPID(serviceName, pid)
}
