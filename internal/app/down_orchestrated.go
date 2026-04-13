package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"raioz/internal/config"
	"raioz/internal/detect"
	"raioz/internal/docker"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/orchestrate"
	"raioz/internal/output"
	"raioz/internal/runtime"
	"raioz/internal/state"
)

// downOrchestrated handles raioz down for YAML-based (orchestrated) projects.
// Instead of using a generated compose file, it stops containers by name
// and kills host processes tracked in .raioz.state.json.
func (uc *DownUseCase) downOrchestrated(ctx context.Context, opts DownOptions) error {
	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = "raioz.yaml"
	}

	deps, _, err := uc.deps.ConfigLoader.LoadDeps(configPath)
	if err != nil || deps == nil {
		return nil // Cannot load config — fall through to legacy down
	}

	if deps.SchemaVersion != "2.0" {
		return nil // Not a YAML project — fall through to legacy down
	}

	// Set naming prefix from workspace
	naming.SetPrefix(deps.Workspace)

	projectDir, _ := filepath.Abs(filepath.Dir(configPath))
	projectName := deps.Project.Name

	output.PrintProgress("Stopping project " + projectName + "...")

	// Run custom per-service stop commands first (declared in raioz.yaml as `stop:`).
	// These take precedence over PID/prefix-based cleanup because they know how to
	// tear down whatever the `command:` started — e.g. `make start` spawning its
	// own docker compose project whose containers are outside the raioz naming
	// convention.
	runCustomStopCommands(ctx, deps, projectDir)

	// Tear down compose-based services (yaml `compose:` or auto-detected) by
	// invoking `docker compose down` with the same project name scope that
	// ComposeRunner used to bring them up. Without this, the prefix-based sweep
	// below misses them because docker compose names containers `<dir>-<svc>-N`
	// rather than `raioz-<project>-<svc>`.
	stopComposeServices(ctx, deps)

	// Stop host processes from local state
	localState, _ := state.LoadLocalState(projectDir)
	if localState != nil {
		for name, pid := range localState.HostPIDs {
			if pid > 0 {
				// Skip services that already ran a custom stop command.
				if svc, ok := deps.Services[name]; ok && svc.Commands != nil && svc.Commands.Down != "" {
					continue
				}
				logging.InfoWithContext(ctx, "Stopping host process",
					"service", name, "pid", pid)
				killProcessGroup(pid)
			}
		}
		// Clear PIDs from state
		localState.HostPIDs = make(map[string]int)
		state.SaveLocalState(projectDir, localState)
	}

	// Stop raioz containers by name pattern
	containerPrefix := naming.ContainerPrefix(projectName)
	stopContainersByPrefix(ctx, containerPrefix)

	// Stop dependency compose projects
	stopDependencyComposeProjects(ctx, deps, projectName)

	// Stop proxy
	uc.stopProxy(ctx, opts)

	// Clean local state
	if localState != nil {
		localState.HostPIDs = make(map[string]int)
		state.SaveLocalState(projectDir, localState)
	}

	// Clean network
	networkName := deps.Network.GetName()
	if networkName != "" {
		inUse, _ := uc.deps.DockerRunner.IsNetworkInUseWithContext(ctx, networkName)
		if !inUse {
			exec.CommandContext(ctx, runtime.Binary(), "network", "rm", networkName).Run()
			logging.InfoWithContext(ctx, "Network removed", "network", networkName)
		}
	}

	output.PrintSuccess("Project '" + projectName + "' stopped")
	return nil
}

// stopContainersByPrefix stops and removes all containers matching a name prefix.
func stopContainersByPrefix(ctx context.Context, prefix string) {
	// List containers matching prefix
	cmd := exec.CommandContext(ctx, runtime.Binary(), "ps", "-a",
		"--filter", "name="+prefix,
		"--format", "{{.Names}}")
	out, err := cmd.Output()
	if err != nil {
		return
	}

	names := strings.TrimSpace(string(out))
	if names == "" {
		return
	}

	for _, name := range strings.Split(names, "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		logging.Info("Stopping container", "name", name)
		exec.CommandContext(ctx, runtime.Binary(), "stop", name).Run()
		exec.CommandContext(ctx, runtime.Binary(), "rm", "-f", name).Run()
	}
}

// stopDependencyComposeProjects stops compose projects created by image_runner.
func stopDependencyComposeProjects(ctx context.Context, deps *config.Deps, projectName string) {
	for name := range deps.Infra {
		composePath := naming.DepComposePath(projectName, name)
		cmd := exec.CommandContext(ctx, runtime.Binary(), "compose", "-f", composePath, "down")
		cmd.Run() // Ignore errors — file might not exist
	}
}

// killProcessGroup sends SIGTERM to the process group, then SIGKILL.
// Killing the group ensures child processes (e.g., go run's compiled binary) are also stopped.
func killProcessGroup(pid int) {
	pgid, err := syscall.Getpgid(pid)
	if err == nil && pgid > 0 {
		syscall.Kill(-pgid, syscall.SIGTERM)
	}
	// Fallback: kill PID directly
	exec.Command("kill", fmt.Sprintf("%d", pid)).Run()
}

// stopComposeServices tears down compose-based yaml services by invoking
// `docker compose -f <files> down` under the same COMPOSE_PROJECT_NAME scope
// used at `up` time. Required because the default prefix-based cleanup only
// matches containers named `raioz-<project>-<svc>`, and docker compose names
// its own containers `<dir>-<svc>-N` instead.
func stopComposeServices(ctx context.Context, deps *config.Deps) {
	for name, svc := range deps.Services {
		// Skip services with a custom command — they have their own stop flow.
		if svc.Source.Command != "" {
			continue
		}

		// Resolve the compose files the service was brought up with.
		files := svc.Source.ComposeFiles
		if len(files) == 0 {
			// Not yaml-declared: auto-detect from the service path (same logic
			// ComposeRunner.Start used). If there is no compose file, skip.
			if svc.Source.Path == "" {
				continue
			}
			dr := detect.Detect(svc.Source.Path)
			if dr.Runtime != detect.RuntimeCompose || len(dr.ComposeFiles) == 0 {
				continue
			}
			files = dr.ComposeFiles
		}

		// Include the network overlay raioz wrote next to the first compose file.
		overlay := filepath.Join(filepath.Dir(files[0]), ".raioz-overlay.yml")
		callFiles := append([]string{}, files...)
		if _, err := os.Stat(overlay); err == nil {
			callFiles = append(callFiles, overlay)
		}

		spec := docker.JoinComposePaths(callFiles)
		scopedCtx := docker.WithComposeProjectName(
			ctx, orchestrate.ComposeProjectName(deps.Project.Name, name),
		)

		logging.InfoWithContext(ctx, "Stopping compose service",
			"service", name, "files", files,
			"project", orchestrate.ComposeProjectName(deps.Project.Name, name))

		if err := docker.DownWithContext(scopedCtx, spec); err != nil {
			logging.WarnWithContext(ctx, "Compose service down failed",
				"service", name, "error", err.Error())
			output.PrintWarning(fmt.Sprintf("Failed to stop compose service %s: %v", name, err))
		}
	}
}

// runCustomStopCommands executes each service's `stop:` command from raioz.yaml,
// running in the project directory with the service's configured env vars merged
// in. Failures are logged but do not abort the overall down flow.
func runCustomStopCommands(ctx context.Context, deps *config.Deps, projectDir string) {
	for name, svc := range deps.Services {
		if svc.Commands == nil || svc.Commands.Down == "" {
			continue
		}

		stopCmd := svc.Commands.Down
		logging.InfoWithContext(ctx, "Running custom stop command",
			"service", name, "command", stopCmd)
		output.PrintInfo("Stopping " + name + " via: " + stopCmd)

		parts := strings.Fields(stopCmd)
		if len(parts) == 0 {
			continue
		}
		cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
		cmd.Dir = projectDir

		// Merge env files (best-effort) so commands that read from .env work.
		if svc.Env != nil {
			for _, f := range svc.Env.GetFilePaths() {
				if f == "" {
					continue
				}
				// env file paths are absolute after yaml_loader's baseDir resolution
				cmd.Env = append(cmd.Env, "RAIOZ_ENV_FILE="+f)
			}
		}

		if out, err := cmd.CombinedOutput(); err != nil {
			logging.WarnWithContext(ctx, "Custom stop command failed",
				"service", name, "error", err.Error(), "output", string(out))
			output.PrintWarning(fmt.Sprintf("Stop command for %s failed: %v", name, err))
		}
	}
}
