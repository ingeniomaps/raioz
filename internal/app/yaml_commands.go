package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"raioz/internal/detect"
	"raioz/internal/output"
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
			runtime := "unknown"
			if svc.Source.Path != "" {
				result := detect.Detect(svc.Source.Path)
				runtime = string(result.Runtime)
			}
			status := "host"

			// Check if it has a dev override
			devLabel := ""
			if localState != nil && localState.IsDevOverridden(name) {
				devLabel = " (dev)"
			}

			fmt.Printf("    %-18s %-10s %-10s%s\n", name, runtime, status, devLabel)
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
func LogsYAML(ctx context.Context, proj *YAMLProject, services []string, follow bool, tail int) error {
	if len(services) == 0 {
		// Show logs for all containers
		containers := proj.ListRunningContainers(ctx)
		if len(containers) == 0 {
			output.PrintInfo("No running containers")
			return nil
		}
		services = containers
	} else {
		// Convert service names to container names
		var containers []string
		for _, name := range services {
			containers = append(containers, fmt.Sprintf("raioz-%s-%s", proj.ProjectName, name))
		}
		services = containers
	}

	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	if tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", tail))
	}
	args = append(args, services...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RestartYAML restarts services in a YAML orchestrated project.
func RestartYAML(ctx context.Context, proj *YAMLProject, services []string) error {
	if len(services) == 0 {
		output.PrintWarning("No services specified. Use service names or --all")
		return nil
	}

	for _, name := range services {
		containerName := fmt.Sprintf("raioz-%s-%s", proj.ProjectName, name)
		output.PrintProgress("Restarting " + name + "...")
		cmd := exec.CommandContext(ctx, "docker", "restart", containerName)
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
			return cmd.Run()
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

	cmd := exec.CommandContext(ctx, "docker", args...)
	if isTTY {
		cmd.Stdin = os.Stdin
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// CheckYAML validates a YAML project config.
func CheckYAML(proj *YAMLProject) error {
	fmt.Println()
	output.PrintSectionHeader("Config check: " + proj.ProjectName)

	issues := 0

	// Check service paths exist
	for name, svc := range proj.Deps.Services {
		if svc.Source.Path != "" {
			result := detect.Detect(svc.Source.Path)
			if result.Runtime == detect.RuntimeUnknown {
				output.PrintWarning(fmt.Sprintf("%s: no runtime detected at %s", name, svc.Source.Path))
				issues++
			} else {
				output.PrintSuccess(fmt.Sprintf("%s: %s", name, result.Runtime))
			}
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

	fmt.Println()
	if issues == 0 {
		output.PrintSuccess("All checks passed")
	} else {
		output.PrintWarning(fmt.Sprintf("%d issue(s) found", issues))
	}
	return nil
}
