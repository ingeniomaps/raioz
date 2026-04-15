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
)

// addInfraToCompose processes all infra entries and adds them to the compose map.
// Returns the list of service names from external compose files.
func addInfraToCompose(
	compose map[string]any,
	deps *config.Deps,
	ws *workspace.Workspace,
	projectDir, networkName string,
	infraVolumeMap map[string]string,
) ([]string, error) {
	services := compose["services"].(map[string]any)
	workspaceName := deps.GetWorkspaceName()
	hasExplicitWorkspace := deps.HasExplicitWorkspace()
	var externalInfraNames []string

	for name, entry := range deps.Infra {
		if entry.Path != "" {
			names, err := mergeExternalComposeFile(
				compose, projectDir, entry.Path, workspaceName,
				deps.Project.Name, networkName, hasExplicitWorkspace,
				name, infraVolumeMap,
			)
			if err != nil {
				return nil, err
			}
			externalInfraNames = append(externalInfraNames, names...)
			continue
		}
		if entry.Inline == nil {
			continue
		}

		infraConfig, err := buildInlineInfraConfig(
			name, *entry.Inline, deps, ws, projectDir,
			networkName, workspaceName, hasExplicitWorkspace,
			infraVolumeMap,
		)
		if err != nil {
			return nil, err
		}
		services[name] = infraConfig
	}

	return externalInfraNames, nil
}

// buildInlineInfraConfig builds the compose config for a single inline infra entry.
func buildInlineInfraConfig(
	name string,
	infra config.Infra,
	deps *config.Deps,
	ws *workspace.Workspace,
	projectDir, networkName, workspaceName string,
	hasExplicitWorkspace bool,
	infraVolumeMap map[string]string,
) (map[string]any, error) {
	image := infra.Image
	if infra.Tag != "" {
		image = image + ":" + infra.Tag
	}

	containerName, err := NormalizeInfraName(workspaceName, name, deps.Project.Name, hasExplicitWorkspace)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize container name for infra %s: %w", name, err)
	}

	// Configure network: use IP if specified, otherwise simple list
	var infraNetworksConfig any
	if infra.IP != "" {
		infraNetworksConfig = map[string]any{
			networkName: map[string]any{
				"ipv4_address": infra.IP,
			},
		}
	} else {
		infraNetworksConfig = []string{networkName}
	}

	infraConfig := map[string]any{
		"container_name": containerName,
		"image":          image,
		"networks":       infraNetworksConfig,
	}

	if len(infra.Ports) > 0 {
		infraConfig["ports"] = infra.Ports
	}

	// Infra volumes use workspace prefix so they are shared across projects
	if len(infra.Volumes) > 0 {
		resolvedVolumes, err := ResolveRelativeVolumes(infra.Volumes, projectDir)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve relative volumes for infra %s: %w", name, err)
		}
		normalizedVolumes, err := NormalizeVolumeNamesInStrings(resolvedVolumes, workspaceName, infraVolumeMap)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize volume names for infra %s: %w", name, err)
		}
		infraConfig["volumes"] = normalizedVolumes
	}

	// Seed data: mount files/dirs in /docker-entrypoint-initdb.d/
	if len(infra.Seed) > 0 {
		initDir := getInitDir(infra.Image)
		var seedVolumes []string
		for _, seedPath := range infra.Seed {
			absPath := seedPath
			if !filepath.IsAbs(seedPath) {
				absPath = filepath.Join(projectDir, seedPath)
			}
			seedVolumes = append(seedVolumes, absPath+":"+initDir+"/"+filepath.Base(seedPath)+":ro")
		}
		// Append to existing volumes
		if existing, ok := infraConfig["volumes"].([]string); ok {
			infraConfig["volumes"] = append(existing, seedVolumes...)
		} else {
			infraConfig["volumes"] = seedVolumes
		}
	}

	// Resolve env
	envVars, hasEnvFile, err := resolveInfraEnv(infraConfig, name, infra, deps, ws, projectDir)
	if err != nil {
		return nil, err
	}

	// Add default environment variables ONLY if no env_file is configured
	if !hasEnvFile {
		defaultVars := addDefaultInfraEnv(name, infra.Image)
		for key, value := range defaultVars {
			if _, exists := envVars[key]; !exists {
				envVars[key] = value
			}
		}
	}

	if len(envVars) > 0 {
		infraConfig["environment"] = envVars
	}

	// Healthcheck: use config if set, otherwise default for common infra
	if infra.Healthcheck != nil {
		if m := HealthcheckToMap(infra.Healthcheck); len(m) > 0 {
			infraConfig["healthcheck"] = m
		}
	} else {
		healthcheck := addDefaultInfraHealthcheck(name, infra.Image)
		if healthcheck != nil {
			infraConfig["healthcheck"] = healthcheck
		}
	}

	return infraConfig, nil
}

// resolveInfraEnv resolves environment variables and env_file for an infra entry.
// Returns the env vars map, whether an env_file was set, and any error.
func resolveInfraEnv(
	infraConfig map[string]any,
	name string,
	infra config.Infra,
	deps *config.Deps,
	ws *workspace.Workspace,
	projectDir string,
) (map[string]string, bool, error) {
	envVars := make(map[string]string)
	var hasEnvFile bool

	if infra.Env == nil {
		return envVars, false, nil
	}

	if infra.Env.IsObject && infra.Env.Variables != nil {
		for key, value := range infra.Env.Variables {
			envVars[key] = value
		}
		return envVars, false, nil
	}

	// env is an array of file paths - resolve them
	envFiles := infra.Env.GetFilePaths()
	var envFilePath string

	if len(envFiles) == 1 && envFiles[0] == "." {
		localEnvPath := filepath.Join(projectDir, ".env")
		if _, statErr := os.Stat(localEnvPath); statErr == nil {
			envFilePath = localEnvPath
			hasEnvFile = true
		} else {
			resolved, err := env.ResolveEnvFileForService(ws, deps, name, infra.Env, projectDir, "")
			if err != nil {
				return nil, false, fmt.Errorf("failed to resolve env files for infra %s: %w", name, err)
			}
			if resolved != "" {
				envFilePath = resolved
				hasEnvFile = true
			}
		}
	} else {
		resolved, err := env.ResolveEnvFileForService(ws, deps, name, infra.Env, projectDir, "")
		if err != nil {
			return nil, false, fmt.Errorf("failed to resolve env files for infra %s: %w", name, err)
		}
		if resolved != "" {
			envFilePath = resolved
			hasEnvFile = true
		}
	}

	if hasEnvFile {
		infraConfig["env_file"] = []string{envFilePath}
	}

	return envVars, hasEnvFile, nil
}

// writeCombinedEnvFile creates a combined .env file for Docker Compose from all infra and project variables.
func writeCombinedEnvFile(deps *config.Deps, ws *workspace.Workspace, projectDir string) error {
	allCombinedVars := make(map[string]string)

	// Collect variables from inline infra
	for name, entry := range deps.Infra {
		if entry.Inline == nil {
			continue
		}
		infra := *entry.Inline
		if infra.Env == nil {
			continue
		}
		if infra.Env.IsObject && infra.Env.Variables != nil {
			for k, v := range infra.Env.Variables {
				allCombinedVars[k] = v
			}
		} else {
			collectInfraEnvFromFiles(allCombinedVars, name, infra, deps, ws, projectDir)
		}
	}

	// Merge global.env
	if deps.Env.UseGlobal {
		globalPath := filepath.Join(ws.EnvDir, "global.env")
		if _, err := os.Stat(globalPath); err == nil {
			globalVars, err := env.LoadFiles([]string{globalPath})
			if err == nil {
				for k, v := range globalVars {
					allCombinedVars[k] = v
				}
			}
		}
	}

	// Merge project-relative env.files
	for _, envFile := range deps.Env.Files {
		if envFile == "" || strings.HasPrefix(envFile, "projects/") || strings.HasPrefix(envFile, "services/") {
			continue
		}
		if strings.HasPrefix(envFile, ".") || strings.Contains(envFile, ".") {
			projectRelPath := filepath.Join(projectDir, envFile)
			if _, err := os.Stat(projectRelPath); err == nil {
				fileVars, err := env.LoadFiles([]string{projectRelPath})
				if err == nil {
					for k, v := range fileVars {
						allCombinedVars[k] = v
					}
				}
			}
		}
	}

	// Merge direct variables
	if deps.Env.Variables != nil {
		for k, v := range deps.Env.Variables {
			allCombinedVars[k] = v
		}
	}

	if len(allCombinedVars) == 0 {
		return nil
	}

	// Write combined .env file
	combinedEnvPath := filepath.Join(ws.Root, ".env")
	if err := os.MkdirAll(ws.Root, 0700); err != nil {
		return fmt.Errorf("failed to create workspace root: %w", err)
	}

	file, err := os.OpenFile(combinedEnvPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create combined env file: %w", err)
	}
	defer file.Close()

	keys := make([]string, 0, len(allCombinedVars))
	for k := range allCombinedVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := allCombinedVars[key]
		escapedValue := value
		if strings.Contains(value, " ") || strings.Contains(value, "$") || strings.Contains(value, "\"") {
			escapedValue = fmt.Sprintf("\"%s\"", strings.ReplaceAll(value, "\"", "\\\""))
		}
		if _, err := fmt.Fprintf(file, "%s=%s\n", key, escapedValue); err != nil {
			return fmt.Errorf("failed to write to combined env file: %w", err)
		}
	}

	return nil
}

// collectInfraEnvFromFiles loads env variables from infra env file paths into the target map.
func collectInfraEnvFromFiles(
	target map[string]string,
	name string,
	infra config.Infra,
	deps *config.Deps,
	ws *workspace.Workspace,
	projectDir string,
) {
	envFiles := infra.Env.GetFilePaths()
	var envFilePath string

	if len(envFiles) == 1 && envFiles[0] == "." {
		localEnvPath := filepath.Join(projectDir, ".env")
		if _, statErr := os.Stat(localEnvPath); statErr == nil {
			envFilePath = localEnvPath
		} else {
			resolvedPath, _ := env.ResolveEnvFileForService(ws, deps, name, infra.Env, projectDir, "")
			if resolvedPath != "" {
				envFilePath = resolvedPath
			}
		}
	} else {
		resolvedPath, _ := env.ResolveEnvFileForService(ws, deps, name, infra.Env, projectDir, "")
		if resolvedPath != "" {
			envFilePath = resolvedPath
		}
	}

	if envFilePath != "" {
		loadedVars, loadErr := env.LoadFiles([]string{envFilePath})
		if loadErr == nil {
			for k, v := range loadedVars {
				target[k] = v
			}
		}
	}
}

// getInitDir returns the init directory for seed data based on the image name
func getInitDir(image string) string {
	img := strings.ToLower(image)
	switch {
	case strings.Contains(img, "postgres"):
		return "/docker-entrypoint-initdb.d"
	case strings.Contains(img, "mysql"), strings.Contains(img, "mariadb"):
		return "/docker-entrypoint-initdb.d"
	case strings.Contains(img, "mongo"):
		return "/docker-entrypoint-initdb.d"
	default:
		return "/docker-entrypoint-initdb.d"
	}
}
