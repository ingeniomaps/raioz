package app

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"raioz/internal/config"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/output"
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

	// Stop host processes from local state
	localState, _ := state.LoadLocalState(projectDir)
	if localState != nil {
		for name, pid := range localState.HostPIDs {
			if pid > 0 {
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
			exec.CommandContext(ctx, "docker", "network", "rm", networkName).Run()
			logging.InfoWithContext(ctx, "Network removed", "network", networkName)
		}
	}

	output.PrintSuccess("Project '" + projectName + "' stopped")
	return nil
}

// stopContainersByPrefix stops and removes all containers matching a name prefix.
func stopContainersByPrefix(ctx context.Context, prefix string) {
	// List containers matching prefix
	cmd := exec.CommandContext(ctx, "docker", "ps", "-a",
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
		exec.CommandContext(ctx, "docker", "stop", name).Run()
		exec.CommandContext(ctx, "docker", "rm", "-f", name).Run()
	}
}

// stopDependencyComposeProjects stops compose projects created by image_runner.
func stopDependencyComposeProjects(ctx context.Context, deps *config.Deps, projectName string) {
	for name := range deps.Infra {
		composePath := naming.DepComposePath(projectName, name)
		cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composePath, "down")
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
