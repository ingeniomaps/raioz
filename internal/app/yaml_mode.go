package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"raioz/internal/config"
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
	cmd := exec.CommandContext(ctx, "docker", "ps",
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

// ContainerStatus returns status of a specific container.
func (p *YAMLProject) ContainerStatus(ctx context.Context, name string) string {
	containerName := fmt.Sprintf("raioz-%s-%s", p.ProjectName, name)
	cmd := exec.CommandContext(ctx, "docker", "inspect",
		"--format", "{{.State.Status}}", containerName)
	out, err := cmd.Output()
	if err != nil {
		return "stopped"
	}
	return strings.TrimSpace(string(out))
}

// ContainerStats returns CPU and memory for a container.
func (p *YAMLProject) ContainerStats(ctx context.Context, name string) (cpu, mem string) {
	containerName := fmt.Sprintf("raioz-%s-%s", p.ProjectName, name)
	cmd := exec.CommandContext(ctx, "docker", "stats", "--no-stream",
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
