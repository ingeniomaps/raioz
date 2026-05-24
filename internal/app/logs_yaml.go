package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"raioz/internal/config"
	"raioz/internal/domain/models"
	"raioz/internal/i18n"
	"raioz/internal/naming"
	"raioz/internal/output"
	"raioz/internal/runtime"
)

// LogsYAML shows logs for a YAML orchestrated project. A service can run three
// ways, and each keeps its logs in a different place:
//
//  1. Host process (`command:` or a host runtime like npm/go) — HostRunner
//     writes a log file; we tail it.
//  2. Compose stack (`compose:` or a detected docker-compose.yml) — logs live
//     in Docker under containers compose names itself, so we go through
//     `docker compose -f <files> logs` scoped to the same COMPOSE_PROJECT_NAME
//     used at up time (mirrors stopComposeServices).
//  3. Dockerfile / image service, or a dependency — raioz names the container
//     `raioz-<project>-<name>`, so plain `docker logs` finds it.
//
// The earlier version assumed every `services:` entry was class 1 and printed
// "No logs found: …/svc.log" for the (common) class-2 case.
func LogsYAML(ctx context.Context, proj *YAMLProject, services []string, follow bool, tail int) error {
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

	var dockerContainers []string
	var hostLogFiles []string
	var composeServices []string

	for _, name := range services {
		svc, isService := proj.Deps.Services[name]
		if !isService {
			// Dependency — docker logs <container>.
			dockerContainers = append(dockerContainers, naming.Container(proj.ProjectName, name))
			continue
		}
		// Classify by how the service runs — same runtime check the up-time
		// log streamer uses (internal/app/upcase/log_stream.go).
		det := config.ResolveServiceDetection(svc, svc.Source.Path)
		switch {
		case det.Runtime == models.RuntimeCompose:
			composeServices = append(composeServices, name)
		case det.IsDocker():
			// Dockerfile / image: raioz owns the container name.
			dockerContainers = append(dockerContainers, naming.Container(proj.ProjectName, name))
		default:
			// Host process — the file HostRunner writes.
			hostLogFiles = append(hostLogFiles, naming.LogFile(proj.ProjectName, name))
		}
	}

	for _, logPath := range hostLogFiles {
		if err := showHostLogs(ctx, logPath, follow, tail); err != nil {
			// Log file may not exist yet — not fatal.
			output.PrintWarning(fmt.Sprintf("No logs found: %s", logPath))
		}
	}

	for _, name := range composeServices {
		if err := showComposeServiceLogs(ctx, proj, name, follow, tail); err != nil {
			output.PrintWarning(fmt.Sprintf("Failed to read logs for %s: %v", name, err))
		}
	}

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
