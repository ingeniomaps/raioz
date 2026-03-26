package docker

import (
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/config"
	"raioz/internal/env"
	"raioz/internal/workspace"
)

// addServiceToCompose processes a single service and adds it to the compose services map.
func addServiceToCompose(
	services map[string]any,
	name string,
	svc config.Service,
	deps *config.Deps,
	ws *workspace.Workspace,
	projectDir, networkName string,
	serviceVolumeMap map[string]string,
) error {
	// Skip if explicitly disabled
	if svc.Enabled != nil && !*svc.Enabled {
		return nil
	}
	// Skip services that run on host
	if svc.Source.Command != "" {
		return nil
	}
	// Skip services with custom commands (no docker, no source.command, but has commands)
	if svc.Docker == nil && svc.Commands != nil {
		return nil
	}

	workspaceName := deps.GetWorkspaceName()
	hasExplicitWorkspace := deps.HasExplicitWorkspace()
	containerName, err := NormalizeContainerName(workspaceName, name, deps.Project.Name, hasExplicitWorkspace)
	if err != nil {
		return fmt.Errorf("failed to normalize container name for service %s: %w", name, err)
	}

	// Configure network: use IP if specified, otherwise simple list
	var networksConfig any
	if svc.Docker.IP != "" {
		networksConfig = map[string]any{
			networkName: map[string]any{
				"ipv4_address": svc.Docker.IP,
			},
		}
	} else {
		networksConfig = []string{networkName}
	}

	serviceConfig := map[string]any{
		"container_name": containerName,
		"ports":          svc.Docker.Ports,
		"networks":       networksConfig,
	}

	// Add volumes if present
	if len(svc.Docker.Volumes) > 0 {
		resolvedVolumes, err := ResolveRelativeVolumes(svc.Docker.Volumes, projectDir)
		if err != nil {
			return fmt.Errorf("failed to resolve relative volumes for service %s: %w", name, err)
		}
		normalizedVolumes, err := NormalizeVolumeNamesInStrings(resolvedVolumes, deps.Project.Name, serviceVolumeMap)
		if err != nil {
			return fmt.Errorf("failed to normalize volume names for service %s: %w", name, err)
		}
		volumes := ApplyReadonlyToVolumes(normalizedVolumes, svc)
		serviceConfig["volumes"] = volumes
	}

	// Add depends_on if present
	if deps := svc.GetDependsOn(); len(deps) > 0 {
		serviceConfig["depends_on"] = deps
	}

	if svc.Source.Kind == "git" {
		dockerfilePath, err := EnsureDockerfile(ws, name, svc)
		if err != nil {
			return fmt.Errorf("failed to ensure dockerfile for service %s: %w", name, err)
		}
		context := workspace.GetServicePath(ws, name, svc)
		buildConfig := map[string]any{
			"context":    context,
			"dockerfile": dockerfilePath,
		}
		serviceConfig["build"] = buildConfig
	} else if svc.Source.Kind == "image" {
		image := svc.Source.Image
		if svc.Source.Tag != "" {
			image = image + ":" + svc.Source.Tag
		}
		serviceConfig["image"] = image
	}

	// Resolve and add env_file for service
	var servicePath string
	if svc.Source.Kind == "git" {
		servicePath = workspace.GetServicePath(ws, name, svc)
	}
	envFilePath, err := env.ResolveEnvFileForService(ws, deps, name, svc.Env, projectDir, servicePath)
	if err != nil {
		return fmt.Errorf("failed to resolve env files for service %s: %w", name, err)
	}

	// If envVolume is specified but no env file was generated, create one from global.env only.
	if svc.Docker.EnvVolume != "" && envFilePath == "" && deps.Env.UseGlobal {
		allVars := make(map[string]string)
		globalPath := filepath.Join(ws.EnvDir, "global.env")
		if _, err := os.Stat(globalPath); err == nil {
			globalVars, err := env.LoadFiles([]string{globalPath})
			if err == nil {
				for k, v := range globalVars {
					allVars[k] = v
				}
			}
		}
		envFilePath, err = env.CreateOrUpdateEnvFile(ws, deps, name, allVars, servicePath)
		if err != nil {
			return fmt.Errorf("failed to create env file for service %s: %w", name, err)
		}
	}

	if envFilePath != "" {
		serviceConfig["env_file"] = []string{envFilePath}

		// If envVolume is specified, also mount the .env file as a volume
		if svc.Docker.EnvVolume != "" {
			existingVolumes, ok := serviceConfig["volumes"].([]string)
			if !ok {
				existingVolumes = []string{}
			}
			envVolumeMount := fmt.Sprintf("%s:%s:ro", envFilePath, svc.Docker.EnvVolume)
			existingVolumes = append(existingVolumes, envVolumeMount)
			serviceConfig["volumes"] = existingVolumes
		}
	}

	// Add command if specified
	if svc.Docker.Command != "" {
		serviceConfig["command"] = svc.Docker.Command
	}

	// Apply mode-specific configuration (dev vs prod)
	ApplyModeConfig(serviceConfig, name, svc, ws)

	services[name] = serviceConfig
	return nil
}
