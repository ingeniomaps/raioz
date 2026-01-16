package docker

import (
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/config"
	"raioz/internal/env"
	"raioz/internal/workspace"

	"gopkg.in/yaml.v3"
)

// addDefaultInfraEnv adds default environment variables for common infra services
func addDefaultInfraEnv(name, image string) map[string]string {
	envVars := make(map[string]string)

	// PostgreSQL defaults
	if image == "postgres" || name == "database" || name == "postgres" || name == "postgresql" {
		// Only add default if not already set via env_file
		envVars["POSTGRES_PASSWORD"] = "postgres"
		envVars["POSTGRES_USER"] = "postgres"
		envVars["POSTGRES_DB"] = "postgres"
	}

	return envVars
}

func GenerateCompose(deps *config.Deps, ws *workspace.Workspace) (string, error) {
	// Validate dependency cycles before generating compose
	if err := ValidateDependencyCycle(deps); err != nil {
		return "", fmt.Errorf("dependency validation failed: %w", err)
	}

	// Ensure env directories exist
	if err := env.EnsureEnvDirs(ws); err != nil {
		return "", fmt.Errorf("failed to ensure env directories: %w", err)
	}

	// Collect all volumes to find named volumes
	var allVolumes []string
	for _, svc := range deps.Services {
		allVolumes = append(allVolumes, svc.Docker.Volumes...)
	}
	for _, infra := range deps.Infra {
		allVolumes = append(allVolumes, infra.Volumes...)
	}

	// Extract named volumes
	namedVolumes, err := ExtractNamedVolumes(allVolumes)
	if err != nil {
		return "", fmt.Errorf("failed to extract named volumes: %w", err)
	}

	// Ensure named volumes exist
	// Note: GenerateCompose doesn't accept context, so we use background context
	// This is acceptable as volume creation is fast and happens during compose generation
	for _, volName := range namedVolumes {
		if err := EnsureVolume(volName); err != nil {
			return "", fmt.Errorf("failed to ensure volume '%s': %w", volName, err)
		}
	}

	compose := map[string]any{
		"version":  "3.9",
		"services": map[string]any{},
		"networks": map[string]any{
			deps.Project.Network: map[string]any{
				"external": true,
			},
		},
	}

	// Add volumes section if there are named volumes
	if len(namedVolumes) > 0 {
		volumesMap := make(map[string]any)
		for _, volName := range namedVolumes {
			volumesMap[volName] = map[string]any{} // Empty config uses default driver
		}
		compose["volumes"] = volumesMap
	}

	services := compose["services"].(map[string]any)

	// Process services (disabled services are already filtered out by FilterByFeatureFlags)
	for name, svc := range deps.Services {
		// Double-check: skip if explicitly disabled (shouldn't happen, but safety check)
		if svc.Enabled != nil && !*svc.Enabled {
			continue
		}
		// Skip services that run on host (source.command exists means host execution)
		if svc.Source.Command != "" {
			continue
		}
		// Skip services with custom commands (no docker, no source.command, but has commands)
		if svc.Docker == nil && svc.Commands != nil {
			continue
		}
		// Generate normalized container name
		containerName, err := NormalizeContainerName(deps.Project.Name, name)
		if err != nil {
			return "", fmt.Errorf("failed to normalize container name for service %s: %w", name, err)
		}

		serviceConfig := map[string]any{
			"container_name": containerName,
			"ports":          svc.Docker.Ports,
			"networks":       []string{deps.Project.Network},
		}

		// Add volumes if present, applying readonly mode if needed
		if len(svc.Docker.Volumes) > 0 {
			volumes := ApplyReadonlyToVolumes(svc.Docker.Volumes, svc)
			serviceConfig["volumes"] = volumes
		}

		// Add depends_on if present
		if len(svc.Docker.DependsOn) > 0 {
			serviceConfig["depends_on"] = svc.Docker.DependsOn
		}

		if svc.Source.Kind == "git" {
			// Ensure dockerfile exists or generate wrapper
			dockerfilePath, err := EnsureDockerfile(ws, name, svc)
			if err != nil {
				return "", fmt.Errorf("failed to ensure dockerfile for service %s: %w", name, err)
			}

			// Use correct directory based on access mode (readonly vs editable)
			context := workspace.GetServicePath(ws, name, svc)
			buildConfig := map[string]any{
				"context": context,
			}

			// Use dockerfile path (either relative or absolute wrapper)
			buildConfig["dockerfile"] = dockerfilePath

			serviceConfig["build"] = buildConfig
		} else if svc.Source.Kind == "image" {
			image := svc.Source.Image
			if svc.Source.Tag != "" {
				image = image + ":" + svc.Source.Tag
			}
			serviceConfig["image"] = image
		}

		// Resolve and add env_file for service
		envFilePath, err := env.ResolveEnvFileForService(ws, deps, name, svc.Env)
		if err != nil {
			return "", fmt.Errorf("failed to resolve env files for service %s: %w", name, err)
		}
		if envFilePath != "" {
			serviceConfig["env_file"] = []string{envFilePath}
		}

		// Apply mode-specific configuration (dev vs prod)
		// Note: readonly volumes are already applied above
		ApplyModeConfig(serviceConfig, name, svc, ws)

		services[name] = serviceConfig
	}

	// Process infra
	for name, infra := range deps.Infra {
		image := infra.Image
		if infra.Tag != "" {
			image = image + ":" + infra.Tag
		}

		// Generate normalized container name for infra
		containerName, err := NormalizeInfraName(deps.Project.Name, name)
		if err != nil {
			return "", fmt.Errorf("failed to normalize container name for infra %s: %w", name, err)
		}

		infraConfig := map[string]any{
			"container_name": containerName,
			"image":          image,
			"ports":          infra.Ports,
			"volumes":        infra.Volumes,
			"networks":       []string{deps.Project.Network},
		}

		// Add default environment variables for common infra services
		envVars := addDefaultInfraEnv(name, infra.Image)

		// Resolve and add env_file for infra if specified
		if infra.Env != nil {
			envFilePath, err := env.ResolveEnvFileForService(ws, deps, name, infra.Env)
			if err != nil {
				return "", fmt.Errorf("failed to resolve env files for infra %s: %w", name, err)
			}
			if envFilePath != "" {
				infraConfig["env_file"] = []string{envFilePath}
			}
		}

		// Add environment variables if any
		if len(envVars) > 0 {
			infraConfig["environment"] = envVars
		}

		services[name] = infraConfig
	}

	// Marshal YAML (yaml.v3 uses 2-space indentation by default)
	out, err := yaml.Marshal(compose)
	if err != nil {
		return "", fmt.Errorf("failed to marshal compose: %w", err)
	}

	// Add header comment for better readability
	header := `# This file is auto-generated by raioz
# You can run it directly with: docker compose -f docker-compose.generated.yml up
# Or modify it manually if needed (changes will be overwritten on next raioz up)
#
`

	path := filepath.Join(ws.Root, "docker-compose.generated.yml")
	content := header + string(out)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write compose file: %w", err)
	}

	return path, nil
}
