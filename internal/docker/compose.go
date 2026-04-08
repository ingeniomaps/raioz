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

func GenerateCompose(deps *config.Deps, ws *workspace.Workspace, projectDir string) (string, []string, error) {
	// Validate dependency cycles before generating compose
	if err := ValidateDependencyCycle(deps); err != nil {
		return "", nil, fmt.Errorf("dependency validation failed: %w", err)
	}

	// Ensure env directories exist
	if err := env.EnsureEnvDirs(ws); err != nil {
		return "", nil, fmt.Errorf("failed to ensure env directories: %w", err)
	}

	workspaceName := deps.GetWorkspaceName()

	// Build volume maps for infra and services
	infraVolumeMap, err := buildInfraVolumeMap(deps, projectDir, workspaceName)
	if err != nil {
		return "", nil, err
	}
	serviceVolumeMap, err := buildServiceVolumeMap(deps, projectDir)
	if err != nil {
		return "", nil, err
	}

	// Ensure all normalized volumes exist
	normalizedVolumes := collectNormalizedVolumes(infraVolumeMap, serviceVolumeMap)
	for _, volName := range normalizedVolumes {
		if err := EnsureVolume(volName); err != nil {
			return "", nil, fmt.Errorf("failed to ensure volume '%s': %w", volName, err)
		}
	}

	// Build compose structure
	networkName := deps.Network.GetName()
	compose := buildComposeBase(deps, networkName, normalizedVolumes)

	// Process services
	services := compose["services"].(map[string]any)
	for name, svc := range deps.Services {
		if err := addServiceToCompose(services, name, svc, deps, ws, projectDir, networkName, serviceVolumeMap); err != nil {
			return "", nil, err
		}
	}

	// Process infra
	externalInfraNames, err := addInfraToCompose(compose, deps, ws, projectDir, networkName, infraVolumeMap)
	if err != nil {
		return "", nil, err
	}

	// Write combined .env file for Docker Compose
	if err := writeCombinedEnvFile(deps, ws, projectDir); err != nil {
		return "", nil, err
	}

	// Marshal and write compose file
	path, err := marshalAndWriteCompose(compose, ws)
	if err != nil {
		return "", nil, err
	}

	return path, externalInfraNames, nil
}

// buildInfraVolumeMap builds the volume name mapping for infra entries.
func buildInfraVolumeMap(deps *config.Deps, projectDir, workspaceName string) (map[string]string, error) {
	infraVolumeMap := make(map[string]string)
	for _, entry := range deps.Infra {
		if entry.Inline == nil {
			continue
		}
		infra := *entry.Inline
		resolved, err := ResolveRelativeVolumes(infra.Volumes, projectDir)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve relative volumes for infra: %w", err)
		}
		named, err := ExtractNamedVolumes(resolved)
		if err != nil {
			return nil, err
		}
		for _, volName := range named {
			if _, ok := infraVolumeMap[volName]; ok {
				continue
			}
			normalized, err := NormalizeVolumeName(workspaceName, volName)
			if err != nil {
				return nil, fmt.Errorf("failed to normalize infra volume '%s': %w", volName, err)
			}
			infraVolumeMap[volName] = normalized
		}
	}
	return infraVolumeMap, nil
}

// buildServiceVolumeMap builds the volume name mapping for service entries.
func buildServiceVolumeMap(deps *config.Deps, projectDir string) (map[string]string, error) {
	serviceVolumeMap := make(map[string]string)
	for _, svc := range deps.Services {
		if svc.Docker == nil {
			continue
		}
		resolved, err := ResolveRelativeVolumes(svc.Docker.Volumes, projectDir)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve relative volumes for service: %w", err)
		}
		named, err := ExtractNamedVolumes(resolved)
		if err != nil {
			return nil, err
		}
		for _, volName := range named {
			if _, ok := serviceVolumeMap[volName]; ok {
				continue
			}
			normalized, err := NormalizeVolumeName(deps.Project.Name, volName)
			if err != nil {
				return nil, fmt.Errorf("failed to normalize service volume '%s': %w", volName, err)
			}
			serviceVolumeMap[volName] = normalized
		}
	}
	return serviceVolumeMap, nil
}

// collectNormalizedVolumes returns a deduplicated list of all normalized volume names.
func collectNormalizedVolumes(infraVolumeMap, serviceVolumeMap map[string]string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, n := range infraVolumeMap {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	for _, n := range serviceVolumeMap {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// buildComposeBase creates the base compose map with networks and volumes.
func buildComposeBase(deps *config.Deps, networkName string, normalizedVolumes []string) map[string]any {
	networkSubnet := deps.Network.GetSubnet()

	// Check if any service or infra has IP configured
	hasStaticIPs := false
	for _, svc := range deps.Services {
		if svc.Docker != nil && svc.Docker.IP != "" {
			hasStaticIPs = true
			break
		}
	}
	if !hasStaticIPs {
		for _, entry := range deps.Infra {
			if entry.Inline != nil && entry.Inline.IP != "" {
				hasStaticIPs = true
				break
			}
		}
	}

	networkConfig := map[string]any{
		"external": true,
	}
	if hasStaticIPs && networkSubnet != "" {
		networkConfig["ipam"] = map[string]any{
			"config": []map[string]any{
				{"subnet": networkSubnet},
			},
		}
	}

	compose := map[string]any{
		"services": map[string]any{},
		"networks": map[string]any{
			networkName: networkConfig,
		},
	}

	if len(normalizedVolumes) > 0 {
		volumesMap := make(map[string]any)
		for _, volName := range normalizedVolumes {
			volumesMap[volName] = map[string]any{
				"external": true,
			}
		}
		compose["volumes"] = volumesMap
	}

	return compose
}

// marshalAndWriteCompose marshals the compose map to YAML and writes it to the workspace.
func marshalAndWriteCompose(compose map[string]any, ws *workspace.Workspace) (string, error) {
	out, err := yaml.Marshal(compose)
	if err != nil {
		return "", fmt.Errorf("failed to marshal compose: %w", err)
	}

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
