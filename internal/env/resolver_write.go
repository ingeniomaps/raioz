package env

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"raioz/internal/config"
	raiozErr "raioz/internal/errors"
	"raioz/internal/workspace"
)

// CreateOrUpdateEnvFile creates or updates an .env file with the given variables.
// If servicePath is provided (Git service), creates .env in the service directory.
// Otherwise, creates at: projects/{project}/services/{service}.env
// If the file already exists, variables are merged (new values override existing ones)
func CreateOrUpdateEnvFile(
	ws *workspace.Workspace,
	deps *config.Deps,
	serviceName string,
	variables map[string]string,
	servicePath string,
) (string, error) {
	var envPath string

	// If servicePath is provided (Git service), create .env in service directory
	if servicePath != "" {
		envPath = filepath.Join(servicePath, ".env")
	} else {
		// Otherwise, create in workspace env directory
		envDir := filepath.Join(ws.EnvDir, "projects", deps.Project.Name, "services")
		envPath = filepath.Join(envDir, serviceName+".env")

		// Ensure directory exists (use 0700 for security - owner only)
		if err := os.MkdirAll(envDir, 0700); err != nil {
			return "", raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to create env directory").
				WithContext("directory", envDir).
				WithContext("service", serviceName).
				WithSuggestion("Check that the parent directory exists and you have write permissions").
				WithError(err)
		}
	}

	// Ensure directory exists (use 0700 for security - owner only)
	if err := os.MkdirAll(filepath.Dir(envPath), 0700); err != nil {
		return "", raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to create env directory").
			WithContext("directory", filepath.Dir(envPath)).
			WithContext("service", serviceName).
			WithSuggestion("Check that the parent directory exists and you have write permissions").
			WithError(err)
	}

	// Load existing variables if file exists
	existingVars := make(map[string]string)
	if _, err := os.Stat(envPath); err == nil {
		loaded, err := loadSingleFile(envPath)
		if err != nil {
			return "", raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to load existing env file").
				WithContext("file", envPath).
				WithContext("service", serviceName).
				WithSuggestion("Check that the existing env file has valid KEY=VALUE format").
				WithError(err)
		}
		existingVars = loaded
	}

	// Merge: new variables override existing ones
	for key, value := range variables {
		existingVars[key] = value
	}

	// Write merged variables to file (use 0600 permissions for security - contains secrets)
	if err := writeEnvFile(envPath, existingVars); err != nil {
		return "", err
	}

	return envPath, nil
}

// EnsureEnvDirs creates the env directory structure if it doesn't exist
func EnsureEnvDirs(ws *workspace.Workspace) error {
	dirs := []string{
		ws.EnvDir,
		filepath.Join(ws.EnvDir, "services"),
		filepath.Join(ws.EnvDir, "projects"),
	}

	// Use 0700 permissions (read/write/execute for owner only) for security
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to create env directory").
				WithContext("directory", dir).
				WithSuggestion("Check that the parent directory exists and you have write permissions").
				WithError(err)
		}
	}

	return nil
}

// WriteGlobalEnvVariables writes global.env as the union of env.files (project-relative) and env.variables.
// global = env.files content + env.variables; neither replaces the other.
// projectDir is the directory of the .raioz.json (for resolving env.files like ".env.global").
func WriteGlobalEnvVariables(ws *workspace.Workspace, deps *config.Deps, projectDir string) error {
	// Skip if nothing to write
	if !deps.Env.UseGlobal && len(deps.Env.Files) == 0 && len(deps.Env.Variables) == 0 {
		return nil
	}

	if err := EnsureEnvDirs(ws); err != nil {
		return raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to ensure env directories").
			WithSuggestion("Check workspace permissions and disk space").
			WithError(err)
	}

	globalPath := filepath.Join(ws.EnvDir, "global.env")
	merged := make(map[string]string)

	// 1. Load existing global.env if it exists
	if _, err := os.Stat(globalPath); err == nil {
		loaded, err := loadSingleFile(globalPath)
		if err != nil {
			return raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to load existing global.env").
				WithContext("file", globalPath).
				WithSuggestion("Check that global.env has valid KEY=VALUE format").
				WithError(err)
		}
		for k, v := range loaded {
			merged[k] = v
		}
	}

	// 2. Merge env.files (project-relative)
	if deps.Env.Files != nil && projectDir != "" {
		for _, envFile := range deps.Env.Files {
			if envFile == "" || strings.HasPrefix(envFile, "projects/") || strings.HasPrefix(envFile, "services/") {
				continue
			}
			if strings.HasPrefix(envFile, ".") || strings.Contains(envFile, ".") {
				p := filepath.Join(projectDir, envFile)
				if _, err := os.Stat(p); err != nil {
					continue
				}
				fileVars, err := LoadFiles([]string{p})
				if err != nil {
					continue
				}
				for k, v := range fileVars {
					merged[k] = v
				}
			}
		}
	}

	// 3. Merge env.variables (override / add)
	if deps.Env.Variables != nil {
		for k, v := range deps.Env.Variables {
			merged[k] = v
		}
	}

	if len(merged) == 0 {
		return nil
	}

	file, err := os.OpenFile(globalPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to create global.env file").
			WithContext("file", globalPath).
			WithSuggestion("Check file permissions and disk space").
			WithError(err)
	}
	defer file.Close()

	if _, err := fmt.Fprintf(file, "# Variables de entorno globales (env.files + env.variables)\n"); err != nil {
		return raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to write to global.env").
			WithContext("file", globalPath).
			WithSuggestion("Check disk space and file permissions").
			WithError(err)
	}
	if _, err := fmt.Fprintf(file, "# Se aplica a todos los servicios si useGlobal: true\n\n"); err != nil {
		return raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to write to global.env").
			WithContext("file", globalPath).
			WithSuggestion("Check disk space and file permissions").
			WithError(err)
	}

	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := merged[key]
		escapedValue := value
		if strings.Contains(value, " ") || strings.Contains(value, "$") || strings.Contains(value, "\"") {
			escapedValue = fmt.Sprintf("\"%s\"", strings.ReplaceAll(value, "\"", "\\\""))
		}
		if _, err := fmt.Fprintf(file, "%s=%s\n", key, escapedValue); err != nil {
			return raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to write to global.env").
				WithContext("file", globalPath).
				WithSuggestion("Check disk space and file permissions").
				WithError(err)
		}
	}

	return nil
}
