package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"raioz/internal/config"
	"raioz/internal/naming"
	"raioz/internal/runtime"
)

func findConfigFile() string {
	for _, c := range []string{"raioz.yaml", "raioz.yml", ".raioz.json"} {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// YAMLProject holds resolved info for a YAML-mode project.
type YAMLProject struct {
	Deps        *config.Deps
	ProjectName string
	NetworkName string
	ConfigPath  string
}

// ResolveYAMLProject attempts to load the config and returns a YAMLProject
// if it's a YAML-mode project (schemaVersion 2.0). Returns nil if not.
func ResolveYAMLProject(deps *Dependencies, configPath string) *YAMLProject {
	if configPath == "" || configPath == ":auto:" {
		configPath = findConfigFile()
		if configPath == "" {
			return nil
		}
	}

	cfgDeps, _, err := deps.ConfigLoader.LoadDeps(configPath)
	if err != nil || cfgDeps == nil {
		return nil
	}

	if cfgDeps.SchemaVersion != "2.0" {
		return nil
	}

	// Activate the workspace prefix so naming.DepContainer / .Container
	// produce the same names the up flow used. Without this, status /
	// subsequent commands would look for `raioz-<proj>-<dep>` while the
	// container on disk is `<workspace>-<dep>`.
	naming.SetPrefix(cfgDeps.Workspace)

	return &YAMLProject{
		Deps:        cfgDeps,
		ProjectName: cfgDeps.Project.Name,
		NetworkName: cfgDeps.Network.GetName(),
		ConfigPath:  configPath,
	}
}

// ContainerPrefix returns the naming prefix for this project's containers.
func (p *YAMLProject) ContainerPrefix() string {
	return fmt.Sprintf("raioz-%s-", p.ProjectName)
}

// ListRunningContainers returns names of running containers for this project.
func (p *YAMLProject) ListRunningContainers(ctx context.Context) []string {
	cmd := exec.CommandContext(ctx, runtime.Binary(), "ps",
		"--filter", "name="+p.ContainerPrefix(),
		"--format", "{{.Names}}")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	names := strings.TrimSpace(string(out))
	if names == "" {
		return nil
	}
	return strings.Split(names, "\n")
}

// resolveInfraContainerName picks the right container name for a dependency
// based on workspace-sharing rules and user-supplied `name:` overrides,
// falling back to the legacy per-project form when neither applies.
func (p *YAMLProject) resolveInfraContainerName(name string) string {
	if p.Deps == nil {
		return naming.Container(p.ProjectName, name)
	}
	var override string
	if entry, ok := p.Deps.Infra[name]; ok && entry.Inline != nil {
		override = entry.Inline.Name
	}
	if _, isInfra := p.Deps.Infra[name]; isInfra {
		return naming.DepContainer(p.ProjectName, name, override)
	}
	return naming.Container(p.ProjectName, name)
}

// ContainerStatus returns status of a specific container.
func (p *YAMLProject) ContainerStatus(ctx context.Context, name string) string {
	containerName := p.resolveInfraContainerName(name)
	cmd := exec.CommandContext(ctx, runtime.Binary(), "inspect",
		"--format", "{{.State.Status}}", containerName)
	out, err := cmd.Output()
	if err != nil {
		return "stopped"
	}
	return strings.TrimSpace(string(out))
}

// ContainerStats returns CPU and memory for a container.
func (p *YAMLProject) ContainerStats(ctx context.Context, name string) (cpu, mem string) {
	containerName := p.resolveInfraContainerName(name)
	cmd := exec.CommandContext(ctx, runtime.Binary(), "stats", "--no-stream",
		"--format", "{{.CPUPerc}}\t{{.MemUsage}}", containerName)
	out, err := cmd.Output()
	if err != nil {
		return "-", "-"
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "\t")
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	return "-", "-"
}
