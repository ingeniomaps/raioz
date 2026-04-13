package env

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"raioz/internal/config"
	raiozErr "raioz/internal/errors"
	pathvalidate "raioz/internal/path"
	"raioz/internal/workspace"
)

// ResolveProjectEnv resolves project.env configuration.
// If project.env is ["."], uses .env in project directory as primary (read-only if exists).
// If .env doesn't exist, creates it normally.
// projectDir is the directory where .raioz.json is located.
func ResolveProjectEnv(ws *workspace.Workspace, deps *config.Deps, projectDir string) (string, error) {
	if deps.Project.Env == nil {
		return "", nil
	}

	// If project.env is an object (direct variables), create/update project.env file
	if deps.Project.Env.IsObject && deps.Project.Env.Variables != nil {
		envDir := filepath.Join(ws.EnvDir, "projects", deps.Project.Name)
		envPath := filepath.Join(envDir, "project.env")

		if err := os.MkdirAll(envDir, 0700); err != nil {
			return "", raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to create env directory for project").
				WithContext("directory", envDir).
				WithContext("project", deps.Project.Name).
				WithSuggestion("Check that the parent directory exists and you have write permissions").
				WithError(err)
		}

		existingVars := make(map[string]string)
		if _, err := os.Stat(envPath); err == nil {
			loaded, err := loadSingleFile(envPath)
			if err != nil {
				return "", raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to load existing project.env").
					WithContext("file", envPath).
					WithContext("project", deps.Project.Name).
					WithSuggestion("Check that the project.env file has valid KEY=VALUE format").
					WithError(err)
			}
			existingVars = loaded
		}

		for key, value := range deps.Project.Env.Variables {
			existingVars[key] = value
		}

		file, err := os.OpenFile(envPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return "", raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to create project.env file").
				WithContext("file", envPath).
				WithContext("project", deps.Project.Name).
				WithSuggestion("Check file permissions and that the directory exists").
				WithError(err)
		}
		defer file.Close()

		keys := make([]string, 0, len(existingVars))
		for key := range existingVars {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			value := existingVars[key]
			escapedValue := value
			if strings.Contains(value, " ") || strings.Contains(value, "$") || strings.Contains(value, "\"") {
				escapedValue = fmt.Sprintf("\"%s\"", strings.ReplaceAll(value, "\"", "\\\""))
			}
			if _, err := fmt.Fprintf(file, "%s=%s\n", key, escapedValue); err != nil {
				return "", raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to write to project.env").
					WithContext("file", envPath).
					WithContext("key", key).
					WithSuggestion("Check disk space and file permissions").
					WithError(err)
			}
		}

		return envPath, nil
	}

	// If project.env is an array, check for special case ["."]
	envFiles := deps.Project.Env.GetFilePaths()
	if len(envFiles) == 1 && envFiles[0] == "." {
		localEnvPath := filepath.Join(projectDir, ".env")
		if _, err := os.Stat(localEnvPath); err == nil {
			return localEnvPath, nil
		}
		return "", nil
	}

	// For other array values, resolve normally
	if len(envFiles) > 0 {
		resolvedPaths, err := ResolveEnvFiles(ws, deps, "", envFiles, "", true, projectDir)
		if err != nil {
			return "", err
		}
		if len(resolvedPaths) > 0 {
			return resolvedPaths[0], nil
		}
	}

	return "", nil
}

// ResolveEnvFiles resolves and returns paths to env files for a service or infra.
// When includeProjectLevel is false (for services/infra), only global + service/infra env are included.
// When includeProjectLevel is true, global + projectEnvPath + env.files + envFiles are included.
// projectDir is the directory where .raioz.json is located.
func ResolveEnvFiles(
	ws *workspace.Workspace,
	deps *config.Deps,
	serviceName string,
	envFiles []string,
	projectEnvPath string,
	includeProjectLevel bool,
	projectDir string,
) ([]string, error) {
	var resolvedPaths []string

	// 1. Global env file (if useGlobal is true)
	if deps.Env.UseGlobal {
		globalPath := filepath.Join(ws.EnvDir, "global.env")
		if _, err := os.Stat(globalPath); err == nil {
			resolvedPaths = append(resolvedPaths, globalPath)
		}
	}

	// 2. Project.env file: only when resolving for project context
	if includeProjectLevel && projectEnvPath != "" {
		resolvedPaths = append(resolvedPaths, projectEnvPath)
	}

	// 3. Project-specific env files (from env.files) — only for project context
	if includeProjectLevel {
		for _, envFile := range deps.Env.Files {
			var envPath string
			var err error

			if strings.HasPrefix(envFile, "projects/") {
				envPath, err = pathvalidate.EnsurePathInBase(ws.EnvDir, envFile+".env")
				if err != nil {
					return nil, raiozErr.New(raiozErr.ErrCodeInvalidField, "invalid env file path").
						WithContext("envFile", envFile).
						WithContext("baseDir", ws.EnvDir).
						WithSuggestion("Check that the env file path does not escape the env directory").
						WithError(err)
				}
			} else if strings.HasPrefix(envFile, "services/") {
				continue
			} else {
				envPath, err = pathvalidate.EnsurePathInBase(ws.EnvDir, filepath.Join("projects", envFile+".env"))
				if err != nil {
					return nil, raiozErr.New(raiozErr.ErrCodeInvalidField, "invalid env file path").
						WithContext("envFile", envFile).
						WithContext("baseDir", ws.EnvDir).
						WithSuggestion("Check that the env file path does not escape the env directory").
						WithError(err)
				}
			}

			if _, err := os.Stat(envPath); err == nil {
				resolvedPaths = append(resolvedPaths, envPath)
			}
		}
	}

	// 4. Service-specific env files (or infra)
	for _, envFile := range envFiles {
		resolved, err := resolveServiceEnvFile(ws, deps, serviceName, envFile, projectEnvPath, projectDir)
		if err != nil {
			return nil, err
		}
		if resolved != "" {
			resolvedPaths = append(resolvedPaths, resolved)
		}
	}

	return resolvedPaths, nil
}

// resolveServiceEnvFile resolves a single service/infra env file reference.
func resolveServiceEnvFile(
	ws *workspace.Workspace,
	deps *config.Deps,
	serviceName, envFile, projectEnvPath, projectDir string,
) (string, error) {
	// Check project-relative paths first
	hasPath := strings.Contains(envFile, "/") ||
		(strings.HasPrefix(envFile, ".") && len(envFile) > 1)
	if projectDir != "" && envFile != "" && hasPath {
		projectRelPath := filepath.Join(projectDir, envFile)
		if _, statErr := os.Stat(projectRelPath); statErr == nil {
			return projectRelPath, nil
		}
	}

	if strings.HasPrefix(envFile, "services/") {
		svcName := strings.TrimPrefix(envFile, "services/")
		envPath, err := pathvalidate.EnsurePathInBase(ws.EnvDir, filepath.Join("services", svcName+".env"))
		if err != nil {
			return "", raiozErr.New(raiozErr.ErrCodeInvalidField, "invalid service env file path").
				WithContext("envFile", envFile).
				WithContext("service", svcName).
				WithSuggestion("Check that the env file path does not escape the env directory").
				WithError(err)
		}
		if _, err := os.Stat(envPath); err == nil {
			return envPath, nil
		} else if !os.IsNotExist(err) {
			return "", raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to check env file").
				WithContext("file", envPath).
				WithContext("service", svcName).
				WithSuggestion("Check that the env file path is correct and accessible").
				WithError(err)
		}
		return "", nil
	}

	// Determine service name for lookup
	envServiceName := serviceName
	if envFile == "." {
		if projectEnvPath != "" {
			return projectEnvPath, nil
		}
	} else {
		envServiceName = envFile
	}

	// Try project-specific location: projects/{project}/services/{service}.env
	projectSvcEnv := filepath.Join(
		"projects", deps.Project.Name, "services",
		envServiceName+".env",
	)
	projectSpecificPath, err := pathvalidate.EnsurePathInBase(
		ws.EnvDir, projectSvcEnv,
	)
	if err != nil {
		return "", raiozErr.New(raiozErr.ErrCodeInvalidField, "invalid env file path for service").
			WithContext("envFile", envFile).
			WithContext("service", envServiceName).
			WithContext("project", deps.Project.Name).
			WithSuggestion("Check that the env file path does not escape the env directory").
			WithError(err)
	}
	if _, err := os.Stat(projectSpecificPath); err == nil {
		return projectSpecificPath, nil
	} else if !os.IsNotExist(err) {
		return "", raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to check env file").
			WithContext("file", projectSpecificPath).
			WithContext("service", envServiceName).
			WithSuggestion("Check that the env file path is correct and accessible").
			WithError(err)
	}

	// Fallback: shared location: services/{service}.env
	sharedPath, err := pathvalidate.EnsurePathInBase(ws.EnvDir, filepath.Join("services", envServiceName+".env"))
	if err != nil {
		return "", raiozErr.New(raiozErr.ErrCodeInvalidField, "invalid shared env file path").
			WithContext("envFile", envFile).
			WithContext("service", envServiceName).
			WithSuggestion("Check that the env file path does not escape the env directory").
			WithError(err)
	}
	if _, err := os.Stat(sharedPath); err == nil {
		return sharedPath, nil
	} else if !os.IsNotExist(err) {
		return "", raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to check shared env file").
			WithContext("file", sharedPath).
			WithContext("service", envServiceName).
			WithSuggestion("Check that the env file path is correct and accessible").
			WithError(err)
	}

	return "", nil
}

// ResolveEnvFileForService resolves the env_file path(s) for a service.
// If envValue contains direct variables (object), it creates/updates the appropriate .env file.
func ResolveEnvFileForService(
	ws *workspace.Workspace,
	deps *config.Deps,
	serviceName string,
	envValue *config.EnvValue,
	projectDir string,
	servicePath string,
) (string, error) {
	if envValue == nil {
		return "", nil
	}

	// If envValue contains direct variables (object), create/update the .env file
	if envValue.IsObject && envValue.Variables != nil {
		envFilePath, err := CreateOrUpdateEnvFile(ws, deps, serviceName, envValue.Variables, servicePath)
		if err != nil {
			return "", raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to create/update env file for service").
				WithContext("service", serviceName).
				WithSuggestion("Check file permissions and disk space").
				WithError(err)
		}
		return envFilePath, nil
	}

	// Otherwise, resolve file paths (array of strings)
	envFiles := envValue.GetFilePaths()
	if len(envFiles) == 0 {
		return "", nil
	}

	// Check if any envFile is "." - resolve project .env
	var projectEnvPath string
	var hasDotEnv bool
	for _, ef := range envFiles {
		if ef == "." {
			hasDotEnv = true
			if projectDir != "" {
				localEnvPath := filepath.Join(projectDir, ".env")
				if _, statErr := os.Stat(localEnvPath); statErr == nil {
					projectEnvPath = localEnvPath
					break
				}
			}
		}
	}

	resolvedPaths, err := ResolveEnvFiles(ws, deps, serviceName, envFiles, projectEnvPath, false, projectDir)
	if err != nil {
		return "", err
	}

	if len(resolvedPaths) == 0 {
		return "", nil
	}

	if len(resolvedPaths) == 1 {
		return resolvedPaths[0], nil
	}

	// Multiple files, need to create a combined file
	return createCombinedEnvFile(ws, serviceName, resolvedPaths, hasDotEnv, servicePath, projectDir)
}

// createCombinedEnvFile merges multiple env files into a single combined file.
func createCombinedEnvFile(
	ws *workspace.Workspace,
	serviceName string,
	resolvedPaths []string,
	hasDotEnv bool,
	servicePath, projectDir string,
) (string, error) {
	var combinedPath string
	if hasDotEnv {
		if servicePath != "" {
			combinedPath = filepath.Join(servicePath, ".env")
		} else if projectDir != "" {
			combinedPath = filepath.Join(projectDir, ".env")
		} else {
			combinedPath = filepath.Join(ws.Root, fmt.Sprintf(".env.%s", serviceName))
		}
	} else {
		combinedPath = filepath.Join(ws.Root, fmt.Sprintf(".env.%s", serviceName))
	}

	if err := os.MkdirAll(filepath.Dir(combinedPath), 0700); err != nil {
		return "", raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to create directory for combined env file").
			WithContext("directory", filepath.Dir(combinedPath)).
			WithContext("service", serviceName).
			WithSuggestion("Check that the parent directory exists and you have write permissions").
			WithError(err)
	}

	mergedEnv, err := LoadFiles(resolvedPaths)
	if err != nil {
		return "", err
	}
	file, err := os.OpenFile(combinedPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to create combined env file").
			WithContext("file", combinedPath).
			WithContext("service", serviceName).
			WithSuggestion("Check file permissions and disk space").
			WithError(err)
	}
	defer file.Close()

	for key, value := range mergedEnv {
		escapedValue := value
		if strings.Contains(value, " ") || strings.Contains(value, "$") {
			escapedValue = fmt.Sprintf("\"%s\"", value)
		}
		if _, err := fmt.Fprintf(file, "%s=%s\n", key, escapedValue); err != nil {
			return "", raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to write to combined env file").
				WithContext("file", combinedPath).
				WithContext("service", serviceName).
				WithSuggestion("Check disk space and file permissions").
				WithError(err)
		}
	}

	return combinedPath, nil
}
