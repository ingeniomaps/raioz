package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"raioz/internal/audit"
	"raioz/internal/config"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/output"
	"raioz/internal/runtime"
	"raioz/internal/state"
)

// StatusYAML shows status for a YAML orchestrated project. When `filter` is
// non-empty, only services / dependencies in that list are reported and any
// unknown name returns an error so the user notices the typo.
func (uc *StatusUseCase) StatusYAML(ctx context.Context, proj *YAMLProject, filter []string) error {
	if err := validateStatusFilter(proj, filter); err != nil {
		return err
	}
	want := filterSet(filter)

	fmt.Println()
	output.PrintSectionHeader(proj.ProjectName)

	// Dependencies
	visibleInfra := countMatching(proj.Deps.Infra, want)
	if visibleInfra > 0 {
		output.PrintSubsection(fmt.Sprintf("Dependencies (%d)", visibleInfra))
		for name, entry := range proj.Deps.Infra {
			if !inFilter(want, name) {
				continue
			}
			status := proj.ContainerStatus(ctx, name)
			cpu, mem := proj.ContainerStats(ctx, name)
			image := ""
			if entry.Inline != nil {
				image = entry.Inline.Image
				if entry.Inline.Tag != "" {
					image += ":" + entry.Inline.Tag
				}
			}
			fmt.Printf("    %-18s %-10s %-8s %-10s %s\n", name, status, cpu, mem, image)
		}
	}

	// Services
	visibleSvc := countMatchingSvc(proj.Deps.Services, want)
	if visibleSvc > 0 {
		output.PrintSubsection(fmt.Sprintf("Services (%d)", visibleSvc))

		projectDir, _ := filepath.Abs(filepath.Dir(proj.ConfigPath))
		localState, _ := state.LoadLocalState(projectDir)

		for name, svc := range proj.Deps.Services {
			if !inFilter(want, name) {
				continue
			}
			// Honor yaml overrides (command:, compose:) before scanning disk.
			result := config.ResolveServiceDetection(svc, svc.Source.Path)
			runtime := string(result.Runtime)
			if runtime == "" {
				runtime = "unknown"
			}

			// Priority 0: when the user declared `proxy.target`, THAT
			// container is the source of truth — bypass the PID/compose
			// heuristics that go false-negative for launchers that exit
			// 0 after `docker run -d`.
			status := "stopped"
			pidInfo := ""
			if svc.ProxyOverride != nil && svc.ProxyOverride.Target != "" {
				if state := dockerInspectStatus(ctx, svc.ProxyOverride.Target); state != "" {
					if state == "running" {
						status = "running"
					} else {
						status = "stopped"
					}
					goto print
				}
			}

			// Fallback: process alive via saved PID.
			if localState != nil {
				if pid, ok := localState.HostPIDs[name]; ok && pid > 0 {
					if isHostProcessAlive(pid) {
						status = "running"
						pidInfo = fmt.Sprintf("pid:%d", pid)
					}
				}
			}
		print:

			// Check if it has a dev override
			devLabel := ""
			if localState != nil && localState.IsDevOverridden(name) {
				devLabel = " (dev)"
			}

			fmt.Printf("    %-18s %-10s %-10s %-10s%s\n", name, runtime, status, pidInfo, devLabel)
		}
	}

	// Proxy
	if proj.Deps.Proxy && uc.deps.ProxyManager != nil {
		running, _ := uc.deps.ProxyManager.Status(ctx)
		if running {
			output.PrintInfo(i18n.T("output.proxy_running"))
		}
	}

	fmt.Println()
	return nil
}

// LogsYAML shows logs for a YAML orchestrated project.
// Supports both Docker containers (dependencies) and host processes (services).
func LogsYAML(ctx context.Context, proj *YAMLProject, services []string, follow bool, tail int) error {
	// If no services specified, show all
	if len(services) == 0 {
		for name := range proj.Deps.Infra {
			services = append(services, name)
		}
		for name := range proj.Deps.Services {
			services = append(services, name)
		}
	}

	if len(services) == 0 {
		output.PrintInfo(i18n.T("output.no_services_found"))
		return nil
	}

	// Separate host services from Docker containers
	var dockerContainers []string
	var hostLogFiles []string

	for _, name := range services {
		if _, isService := proj.Deps.Services[name]; isService {
			// Host service — read from log file
			logPath := naming.LogFile(proj.ProjectName, name)
			hostLogFiles = append(hostLogFiles, logPath)
		} else {
			// Docker container (dependency)
			dockerContainers = append(dockerContainers, naming.Container(proj.ProjectName, name))
		}
	}

	// Show host service logs
	for _, logPath := range hostLogFiles {
		if err := showHostLogs(ctx, logPath, follow, tail); err != nil {
			// Log file may not exist yet — not fatal
			output.PrintWarning(fmt.Sprintf("No logs found: %s", logPath))
		}
	}

	// Show Docker container logs
	if len(dockerContainers) > 0 {
		args := []string{"logs"}
		if follow {
			args = append(args, "-f")
		}
		if tail > 0 {
			args = append(args, "--tail", fmt.Sprintf("%d", tail))
		}
		args = append(args, dockerContainers...)

		cmd := exec.CommandContext(ctx, runtime.Binary(), args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("docker logs: %w", err)
		}
	}

	return nil
}

// showHostLogs displays logs from a host service log file.
func showHostLogs(ctx context.Context, logPath string, follow bool, tail int) error {
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return fmt.Errorf("stat log file %q: %w", logPath, err)
	}

	if follow {
		cmd := exec.CommandContext(ctx, "tail", "-f", logPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("tail -f %q: %w", logPath, err)
		}
		return nil
	}

	tailLines := "50"
	if tail > 0 {
		tailLines = fmt.Sprintf("%d", tail)
	}

	cmd := exec.CommandContext(ctx, "tail", "-n", tailLines, logPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tail %q: %w", logPath, err)
	}
	return nil
}

// RestartYAML restarts services in a YAML orchestrated project. Host
// services (declared with `command:`) go through HostRunner so the
// settle-window + launcher-pattern logic applies; docker services
// delegate to `docker restart <container>`. ADR-025.
func (uc *RestartUseCase) RestartYAML(
	ctx context.Context, proj *YAMLProject, opts RestartOptions,
) (err error) {
	services := opts.Services
	if len(services) == 0 {
		if !opts.All {
			output.PrintWarning(
				"No services specified. Use service names or --all")
			return nil
		}
		services = collectYAMLServiceNames(proj)
		if opts.IncludeInfra {
			services = append(services, collectYAMLDepNames(proj)...)
		}
		if len(services) == 0 {
			output.PrintWarning(i18n.T("warning.no_services_to_restart"))
			return nil
		}
	}

	// Lifecycle audit. Restart can be partial-success at the
	// per-service level (printed); the lifecycle pair records the
	// outer Execute outcome only.
	startTime := time.Now()
	if auditErr := audit.LogLifecycleStart(
		ctx, "restart", proj.ProjectName, proj.Deps.Workspace,
	); auditErr != nil {
		logging.DebugWithContext(ctx, "audit LogLifecycleStart failed",
			"error", auditErr.Error())
	}
	defer func() {
		status := "success"
		if err != nil {
			status = "failure"
		}
		if auditErr := audit.LogLifecycleComplete(
			ctx, "restart", proj.ProjectName, proj.Deps.Workspace,
			status, time.Since(startTime), err,
		); auditErr != nil {
			logging.DebugWithContext(ctx, "audit LogLifecycleComplete failed",
				"error", auditErr.Error())
		}
	}()

	for _, name := range services {
		if isYAMLHostService(proj, name) {
			if err := uc.restartHostService(ctx, proj, name); err != nil {
				output.PrintProgressError(name + ": " + err.Error())
			} else {
				output.PrintProgressDone(name)
			}
			continue
		}

		containerName := naming.Container(proj.ProjectName, name)
		output.PrintProgress(i18n.T("output.restarting_service", name))
		cmd := exec.CommandContext(ctx, runtime.Binary(), "restart", containerName)
		if out, err := cmd.CombinedOutput(); err != nil {
			output.PrintProgressError(name + ": " + strings.TrimSpace(string(out)))
		} else {
			output.PrintProgressDone(name)
		}
	}
	return nil
}

// collectYAMLServiceNames returns the service names of a YAML project in a
// deterministic order. Sorted so `restart --all` output (and tests) don't
// depend on Go's randomized map iteration.
func collectYAMLServiceNames(proj *YAMLProject) []string {
	if proj == nil || proj.Deps == nil {
		return nil
	}
	names := make([]string, 0, len(proj.Deps.Services))
	for name := range proj.Deps.Services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// collectYAMLDepNames mirrors collectYAMLServiceNames for the infra map,
// used when --include-infra opts dependencies back into restart --all.
func collectYAMLDepNames(proj *YAMLProject) []string {
	if proj == nil || proj.Deps == nil {
		return nil
	}
	names := make([]string, 0, len(proj.Deps.Infra))
	for name := range proj.Deps.Infra {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// isYAMLHostService reports whether the named entry in a YAML project runs
// as a host process — i.e. has a `command:` or `commands:` block, no Docker.
// Used to pick the right restart path. Returns false for unknown names so
// the docker fallback can produce its own (admittedly ugly) error.
func isYAMLHostService(proj *YAMLProject, name string) bool {
	svc, ok := proj.Deps.Services[name]
	if !ok {
		return false
	}
	if svc.Docker != nil {
		return false
	}
	return svc.Source.Command != "" || svc.Commands != nil
}

// ExecYAML runs a command in a container of a YAML orchestrated project.
func ExecYAML(ctx context.Context, proj *YAMLProject, serviceName string, command []string, interactive bool) error {
	containerName := fmt.Sprintf("raioz-%s-%s", proj.ProjectName, serviceName)

	// Check if it's a Docker container or host service
	status := proj.ContainerStatus(ctx, serviceName)
	if status == "stopped" {
		// Might be a host service — exec in the directory
		if svc, ok := proj.Deps.Services[serviceName]; ok && svc.Source.Path != "" {
			output.PrintInfo(i18n.T("output.exec_in_dir", svc.Source.Path))
			args := command
			if len(args) == 0 {
				args = []string{"sh"}
			}
			cmd := exec.CommandContext(ctx, args[0], args[1:]...)
			cmd.Dir = svc.Source.Path
			cmd.Stdin = nil // Will be set by cobra for interactive
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("exec in service dir: %w", err)
			}
			return nil
		}
		return fmt.Errorf("service '%s' is not running", serviceName)
	}

	isTTY := false
	if fileInfo, err := os.Stdin.Stat(); err == nil {
		isTTY = fileInfo.Mode()&os.ModeCharDevice != 0
	}

	args := []string{"exec"}
	if interactive && isTTY {
		args = append(args, "-it")
	}
	args = append(args, containerName)
	if len(command) == 0 {
		args = append(args, "sh")
	} else {
		args = append(args, command...)
	}

	cmd := exec.CommandContext(ctx, runtime.Binary(), args...)
	if isTTY {
		cmd.Stdin = os.Stdin
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker exec: %w", err)
	}
	return nil
}

// isHostProcessAlive checks if a process with the given PID is running.
func isHostProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// CheckYAML validates a YAML project config.
