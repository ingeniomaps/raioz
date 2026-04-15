package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"raioz/internal/app/upcase"
	"raioz/internal/config"
	"raioz/internal/detect"
	"raioz/internal/naming"
	"raioz/internal/output"
	"raioz/internal/runtime"
	"raioz/internal/state"
)

// StatusYAML shows status for a YAML orchestrated project.
func (uc *StatusUseCase) StatusYAML(ctx context.Context, proj *YAMLProject) error {
	fmt.Println()
	output.PrintSectionHeader(proj.ProjectName)

	// Dependencies
	if len(proj.Deps.Infra) > 0 {
		output.PrintSubsection(fmt.Sprintf("Dependencies (%d)", len(proj.Deps.Infra)))
		for name, entry := range proj.Deps.Infra {
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
	if len(proj.Deps.Services) > 0 {
		output.PrintSubsection(fmt.Sprintf("Services (%d)", len(proj.Deps.Services)))

		projectDir, _ := filepath.Abs(filepath.Dir(proj.ConfigPath))
		localState, _ := state.LoadLocalState(projectDir)

		for name, svc := range proj.Deps.Services {
			// Honor yaml overrides (command:, compose:) before scanning disk.
			result := config.ResolveServiceDetection(svc, svc.Source.Path)
			runtime := string(result.Runtime)
			if runtime == "" {
				runtime = "unknown"
			}

			// Check if process is alive via saved PID
			status := "stopped"
			pidInfo := ""
			if localState != nil {
				if pid, ok := localState.HostPIDs[name]; ok && pid > 0 {
					if isHostProcessAlive(pid) {
						status = "running"
						pidInfo = fmt.Sprintf("pid:%d", pid)
					}
				}
			}

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
			output.PrintInfo("Proxy: running")
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
		output.PrintInfo("No services found")
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

// RestartYAML restarts services in a YAML orchestrated project.
func RestartYAML(ctx context.Context, proj *YAMLProject, services []string) error {
	if len(services) == 0 {
		output.PrintWarning("No services specified. Use service names or --all")
		return nil
	}

	for _, name := range services {
		containerName := naming.Container(proj.ProjectName, name)
		output.PrintProgress("Restarting " + name + "...")
		cmd := exec.CommandContext(ctx, runtime.Binary(), "restart", containerName)
		if out, err := cmd.CombinedOutput(); err != nil {
			output.PrintProgressError(name + ": " + strings.TrimSpace(string(out)))
		} else {
			output.PrintProgressDone(name)
		}
	}
	return nil
}

// ExecYAML runs a command in a container of a YAML orchestrated project.
func ExecYAML(ctx context.Context, proj *YAMLProject, serviceName string, command []string, interactive bool) error {
	containerName := fmt.Sprintf("raioz-%s-%s", proj.ProjectName, serviceName)

	// Check if it's a Docker container or host service
	status := proj.ContainerStatus(ctx, serviceName)
	if status == "stopped" {
		// Might be a host service — exec in the directory
		if svc, ok := proj.Deps.Services[serviceName]; ok && svc.Source.Path != "" {
			output.PrintInfo("Executing in service directory: " + svc.Source.Path)
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
func CheckYAML(proj *YAMLProject) error {
	fmt.Println()
	output.PrintSectionHeader("Config check: " + proj.ProjectName)

	issues := 0

	// Check service paths exist (honoring yaml `command:`/`compose:` overrides).
	for name, svc := range proj.Deps.Services {
		result := config.ResolveServiceDetection(svc, svc.Source.Path)
		if result.Runtime == detect.RuntimeUnknown {
			if svc.Source.Path != "" {
				output.PrintWarning(fmt.Sprintf("%s: no runtime detected at %s", name, svc.Source.Path))
			} else {
				output.PrintWarning(fmt.Sprintf("%s: no runtime declared (command/compose/path)", name))
			}
			issues++
		} else {
			output.PrintSuccess(fmt.Sprintf("%s: %s", name, result.Runtime))
		}
	}

	// Check dependency images
	for name, entry := range proj.Deps.Infra {
		if entry.Inline != nil && entry.Inline.Image != "" {
			output.PrintSuccess(fmt.Sprintf("%s: %s", name, entry.Inline.Image))
		}
	}

	// Check dependsOn references
	known := make(map[string]bool)
	for name := range proj.Deps.Services {
		known[name] = true
	}
	for name := range proj.Deps.Infra {
		known[name] = true
	}
	for name, svc := range proj.Deps.Services {
		for _, dep := range svc.GetDependsOn() {
			if !known[dep] {
				output.PrintError(fmt.Sprintf("%s depends on '%s' which is not defined", name, dep))
				issues++
			}
		}
	}

	// Proxy requirements (mkcert presence, certs on disk). Matches what
	// `raioz up` enforces so the user never gets a green check followed by
	// a red up on the same machine.
	if err := upcase.CheckProxyRequirements(proj.Deps); err != nil {
		output.PrintError(err.Error())
		issues++
	}

	// Port allocation + host-bind probing. This runs the same allocator the
	// up flow uses: explicit conflicts fail loud, implicit/auto conflicts
	// bump deterministically, external binders (other projects, random
	// containers, local processes) are surfaced as errors pointing at the
	// offending service or dep.
	//
	// `raioz check` runs this read-only — nothing is actually bound, just
	// a transient net.Listen() per candidate port to probe availability.
	detections := upcase.BuildDetectionMap(proj.Deps)
	if _, err := upcase.AllocateHostPorts(proj.Deps, detections); err != nil {
		output.PrintError(err.Error())
		issues++
	}

	fmt.Println()
	if issues == 0 {
		output.PrintSuccess("All checks passed")
		return nil
	}
	// Issues found: return a sentinel error so the CLI wrapper (cli/check.go)
	// can skip the misleading "Configuration is valid" banner, surface a
	// non-zero exit code, and avoid the "no state found" hint that implies
	// everything is fine. The actual issue list has already been printed
	// above — the error here is just the signal.
	output.PrintWarning(fmt.Sprintf("%d issue(s) found", issues))
	return fmt.Errorf("%d check issue(s) found", issues)
}
