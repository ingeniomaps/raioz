package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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

// addDefaultInfraHealthcheck adds default healthcheck configuration for common infra services
func addDefaultInfraHealthcheck(name, image string) map[string]any {
	// PostgreSQL healthcheck
	if image == "postgres" || name == "database" || name == "postgres" || name == "postgresql" {
		return map[string]any{
			"test": []string{
				"CMD-SHELL",
				"pg_isready -U ${POSTGRES_USER:-postgres} -d ${POSTGRES_DB:-postgres}",
			},
			"interval":    "5s",
			"timeout":     "5s",
			"retries":     10,
			"start_period": "10s",
		}
	}

	// PgAdmin healthcheck
	if image == "dpage/pgadmin4" || name == "pgadmin" {
		return map[string]any{
			"test": []string{
				"CMD-SHELL",
				"curl -f http://localhost/misc/ping 2>/dev/null || wget --no-verbose --tries=1 --spider http://localhost/misc/ping 2>/dev/null || exit 1",
			},
			"interval":    "30s",
			"timeout":     "10s",
			"retries":     5,
			"start_period": "40s",
		}
	}

	// Redis healthcheck
	if image == "redis" || name == "redis" {
		return map[string]any{
			"test": []string{
				"CMD-SHELL",
				"redis-cli ping | grep PONG",
			},
			"interval":    "10s",
			"timeout":     "5s",
			"retries":     5,
			"start_period": "10s",
		}
	}

	// MongoDB healthcheck
	if image == "mongo" || name == "mongo" || name == "mongodb" {
		return map[string]any{
			"test": []string{
				"CMD-SHELL",
				"mongosh --eval 'db.adminCommand(\"ping\")' | grep -q 'ok.*1'",
			},
			"interval":    "10s",
			"timeout":     "5s",
			"retries":     5,
			"start_period": "10s",
		}
	}

	// MySQL/MariaDB healthcheck
	if image == "mysql" || image == "mariadb" || name == "mysql" || name == "mariadb" {
		return map[string]any{
			"test": []string{
				"CMD-SHELL",
				"mysqladmin ping -h localhost || exit 1",
			},
			"interval":    "10s",
			"timeout":     "5s",
			"retries":     5,
			"start_period": "10s",
		}
	}

	return nil
}

func GenerateCompose(deps *config.Deps, ws *workspace.Workspace, projectDir string) (string, error) {
	// Validate dependency cycles before generating compose
	if err := ValidateDependencyCycle(deps); err != nil {
		return "", fmt.Errorf("dependency validation failed: %w", err)
	}

	// Ensure env directories exist
	if err := env.EnsureEnvDirs(ws); err != nil {
		return "", fmt.Errorf("failed to ensure env directories: %w", err)
	}

	// Collect all volumes to find named volumes
	// First resolve relative paths to absolute for accurate named volume detection
	var allVolumes []string
	for _, svc := range deps.Services {
		// Skip if docker is nil (host execution - no docker volumes)
		if svc.Docker != nil {
			resolved, err := ResolveRelativeVolumes(svc.Docker.Volumes, projectDir)
			if err != nil {
				return "", fmt.Errorf("failed to resolve relative volumes for service %s: %w", svc, err)
			}
			allVolumes = append(allVolumes, resolved...)
		}
	}
	for _, infra := range deps.Infra {
		resolved, err := ResolveRelativeVolumes(infra.Volumes, projectDir)
		if err != nil {
			return "", fmt.Errorf("failed to resolve relative volumes for infra %s: %w", infra, err)
		}
		allVolumes = append(allVolumes, resolved...)
	}

	// Extract named volumes (original names from config)
	originalNamedVolumes, err := ExtractNamedVolumes(allVolumes)
	if err != nil {
		return "", fmt.Errorf("failed to extract named volumes: %w", err)
	}

	// Normalize volume names with project prefix and create mapping
	volumeMap := make(map[string]string) // original -> normalized
	normalizedVolumes := make([]string, 0, len(originalNamedVolumes))
	for _, volName := range originalNamedVolumes {
		normalizedName, err := NormalizeVolumeName(deps.Project.Name, volName)
		if err != nil {
			return "", fmt.Errorf("failed to normalize volume name '%s': %w", volName, err)
		}
		volumeMap[volName] = normalizedName
		normalizedVolumes = append(normalizedVolumes, normalizedName)
	}

	// Ensure normalized volumes exist
	// Note: GenerateCompose doesn't accept context, so we use background context
	// This is acceptable as volume creation is fast and happens during compose generation
	for _, volName := range normalizedVolumes {
		if err := EnsureVolume(volName); err != nil {
			return "", fmt.Errorf("failed to ensure volume '%s': %w", volName, err)
		}
	}

	networkName := deps.Project.Network.GetName()
	networkSubnet := deps.Project.Network.GetSubnet()

	// Check if any service or infra has IP configured
	hasStaticIPs := false
	for _, svc := range deps.Services {
		if svc.Docker != nil && svc.Docker.IP != "" {
			hasStaticIPs = true
			break
		}
	}
	if !hasStaticIPs {
		for _, infra := range deps.Infra {
			if infra.IP != "" {
				hasStaticIPs = true
				break
			}
		}
	}

	// Configure network: if static IPs are used, we need subnet in compose
	// Even if network is external, we need subnet config for static IPs
	networkConfig := map[string]any{
		"external": true,
	}
	if hasStaticIPs && networkSubnet != "" {
		// Add subnet configuration for static IPs
		networkConfig["ipam"] = map[string]any{
			"config": []map[string]any{
				{
					"subnet": networkSubnet,
				},
			},
		}
	}

	compose := map[string]any{
		"services": map[string]any{},
		"networks": map[string]any{
			networkName: networkConfig,
		},
	}

	// Add volumes section if there are named volumes
	// Mark volumes as external since Raioz creates them manually before generating compose
	// Volumes are already normalized with project prefix (e.g., roax_postgres_data)
	if len(normalizedVolumes) > 0 {
		volumesMap := make(map[string]any)
		for _, volName := range normalizedVolumes {
			volumesMap[volName] = map[string]any{
				"external": true, // External volumes are created by Raioz, not Docker Compose
			}
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

		// Configure network: use IP if specified, otherwise simple list
		var networksConfig any
		if svc.Docker.IP != "" {
			// Static IP configuration
			networksConfig = map[string]any{
				networkName: map[string]any{
					"ipv4_address": svc.Docker.IP,
				},
			}
		} else {
			// Default: simple network list
			networksConfig = []string{networkName}
		}

		serviceConfig := map[string]any{
			"container_name": containerName,
			"ports":          svc.Docker.Ports,
			"networks":       networksConfig,
		}

		// Add volumes if present, applying readonly mode if needed
		// First resolve relative paths to absolute, then normalize volume names with project prefix
		if len(svc.Docker.Volumes) > 0 {
			// Resolve relative paths to absolute based on project directory
			resolvedVolumes, err := ResolveRelativeVolumes(svc.Docker.Volumes, projectDir)
			if err != nil {
				return "", fmt.Errorf("failed to resolve relative volumes for service %s: %w", name, err)
			}
			// Normalize volume names with project prefix
			normalizedVolumes, err := NormalizeVolumeNamesInStrings(resolvedVolumes, deps.Project.Name, volumeMap)
			if err != nil {
				return "", fmt.Errorf("failed to normalize volume names for service %s: %w", name, err)
			}
			volumes := ApplyReadonlyToVolumes(normalizedVolumes, svc)
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

		// Configure network: use IP if specified, otherwise simple list
		var infraNetworksConfig any
		if infra.IP != "" {
			// Static IP configuration
			infraNetworksConfig = map[string]any{
				networkName: map[string]any{
					"ipv4_address": infra.IP,
				},
			}
		} else {
			// Default: simple network list
			infraNetworksConfig = []string{networkName}
		}

		infraConfig := map[string]any{
			"container_name": containerName,
			"image":          image,
			"networks":       infraNetworksConfig,
		}

		// Add ports only if present (not nil and not empty)
		if len(infra.Ports) > 0 {
			infraConfig["ports"] = infra.Ports
		}

		// Add volumes only if present (not nil and not empty)
		// First resolve relative paths to absolute, then normalize volume names with project prefix
		if len(infra.Volumes) > 0 {
			// Resolve relative paths to absolute based on project directory
			resolvedVolumes, err := ResolveRelativeVolumes(infra.Volumes, projectDir)
			if err != nil {
				return "", fmt.Errorf("failed to resolve relative volumes for infra %s: %w", name, err)
			}
			// Normalize volume names with project prefix
			normalizedVolumes, err := NormalizeVolumeNamesInStrings(resolvedVolumes, deps.Project.Name, volumeMap)
			if err != nil {
				return "", fmt.Errorf("failed to normalize volume names for infra %s: %w", name, err)
			}
			infraConfig["volumes"] = normalizedVolumes
		}

		// Resolve and add env_file for infra if specified
		// Also extract direct variables if env is an object
		var envFilePath string
		var hasEnvFile bool
		envVars := make(map[string]string)
		
		if infra.Env != nil {
			// If env is an object with direct variables, use them directly (no env_file)
			if infra.Env.IsObject && infra.Env.Variables != nil {
				// Use direct variables from config - add them to environment
				for key, value := range infra.Env.Variables {
					envVars[key] = value
				}
				// Don't resolve env_file when env is an object - use environment variables directly
			} else {
				// env is an array of file paths - resolve them
				var err error
				
				envFiles := infra.Env.GetFilePaths()
				if len(envFiles) == 1 && envFiles[0] == "." {
					// Special case: use .env in project directory (same as project.env)
					localEnvPath := filepath.Join(projectDir, ".env")
					if _, statErr := os.Stat(localEnvPath); statErr == nil {
						// .env exists in project directory - use it
						envFilePath = localEnvPath
						hasEnvFile = true
					} else {
						// .env doesn't exist - try normal resolution (will look for {infra-name}.env)
						envFilePath, err = env.ResolveEnvFileForService(ws, deps, name, infra.Env)
						if err != nil {
							return "", fmt.Errorf("failed to resolve env files for infra %s: %w", name, err)
						}
						if envFilePath != "" {
							hasEnvFile = true
						}
					}
				} else {
					// Normal resolution for other cases
					envFilePath, err = env.ResolveEnvFileForService(ws, deps, name, infra.Env)
					if err != nil {
						return "", fmt.Errorf("failed to resolve env files for infra %s: %w", name, err)
					}
					if envFilePath != "" {
						hasEnvFile = true
					}
				}
				
				if hasEnvFile {
					infraConfig["env_file"] = []string{envFilePath}
				}
			}
		}

		// Add default environment variables ONLY if no env_file is configured
		// Docker Compose: environment variables override env_file, so we should not
		// add defaults when env_file exists to avoid overriding values from the file
		if !hasEnvFile {
			defaultVars := addDefaultInfraEnv(name, infra.Image)
			// Merge defaults with direct variables (direct variables override defaults)
			for key, value := range defaultVars {
				if _, exists := envVars[key]; !exists {
					envVars[key] = value
				}
			}
		}

		// Add environment variables if any (only for direct variables or defaults, not from env_file)
		if len(envVars) > 0 {
			infraConfig["environment"] = envVars
		}

		// Add default healthcheck for common infra services if not already configured
		healthcheck := addDefaultInfraHealthcheck(name, infra.Image)
		if healthcheck != nil {
			infraConfig["healthcheck"] = healthcheck
		}

		services[name] = infraConfig
	}

	// Create combined .env file with all variables from all infra (for internal use only)
	// This file is NOT used as env_file, it's only created to have all variables in one place
	allCombinedVars := make(map[string]string)
	for name, infra := range deps.Infra {
		if infra.Env != nil {
			if infra.Env.IsObject && infra.Env.Variables != nil {
				// Direct variables
				for k, v := range infra.Env.Variables {
					allCombinedVars[k] = v
				}
			} else {
				// Variables from env_file
				var envFilePath string
				envFiles := infra.Env.GetFilePaths()
				if len(envFiles) == 1 && envFiles[0] == "." {
					localEnvPath := filepath.Join(projectDir, ".env")
					if _, statErr := os.Stat(localEnvPath); statErr == nil {
						envFilePath = localEnvPath
					} else {
						resolvedPath, _ := env.ResolveEnvFileForService(ws, deps, name, infra.Env)
						if resolvedPath != "" {
							envFilePath = resolvedPath
						}
					}
				} else {
					resolvedPath, _ := env.ResolveEnvFileForService(ws, deps, name, infra.Env)
					if resolvedPath != "" {
						envFilePath = resolvedPath
					}
				}
				
				if envFilePath != "" {
					loadedVars, loadErr := env.LoadFiles([]string{envFilePath})
					if loadErr == nil {
						for k, v := range loadedVars {
							allCombinedVars[k] = v
						}
					}
				}
			}
		}
	}
	
	// Write combined .env file (only for internal reference, not used anywhere)
	if len(allCombinedVars) > 0 {
		combinedEnvPath := filepath.Join(ws.Root, ".env")
		
		// Ensure workspace root exists
		if err := os.MkdirAll(ws.Root, 0700); err != nil {
			return "", fmt.Errorf("failed to create workspace root: %w", err)
		}
		
		// Write combined env file
		file, err := os.OpenFile(combinedEnvPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return "", fmt.Errorf("failed to create combined env file: %w", err)
		}
		
		// Sort keys for consistent output
		keys := make([]string, 0, len(allCombinedVars))
		for k := range allCombinedVars {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		
		for _, key := range keys {
			value := allCombinedVars[key]
			// Escape value if it contains spaces or special characters
			escapedValue := value
			if strings.Contains(value, " ") || strings.Contains(value, "$") || strings.Contains(value, "\"") {
				escapedValue = fmt.Sprintf("\"%s\"", strings.ReplaceAll(value, "\"", "\\\""))
			}
			if _, err := fmt.Fprintf(file, "%s=%s\n", key, escapedValue); err != nil {
				file.Close()
				return "", fmt.Errorf("failed to write to combined env file: %w", err)
			}
		}
		
		file.Close()
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
