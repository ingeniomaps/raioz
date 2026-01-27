package env

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"raioz/internal/config"
	"raioz/internal/workspace"
	pathvalidate "raioz/internal/path"
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

		// Ensure directory exists
		if err := os.MkdirAll(envDir, 0700); err != nil {
			return "", fmt.Errorf("failed to create env directory: %w", err)
		}

		// Load existing variables if file exists
		existingVars := make(map[string]string)
		if _, err := os.Stat(envPath); err == nil {
			loaded, err := loadSingleFile(envPath)
			if err != nil {
				return "", fmt.Errorf("failed to load existing project.env: %w", err)
			}
			existingVars = loaded
		}

		// Merge: new variables override existing ones
		for key, value := range deps.Project.Env.Variables {
			existingVars[key] = value
		}

		// Write merged variables to file
		file, err := os.OpenFile(envPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return "", fmt.Errorf("failed to create project.env file: %w", err)
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
				return "", fmt.Errorf("failed to write to project.env: %w", err)
			}
		}

		return envPath, nil
	}

	// If project.env is an array, check for special case ["."]
	envFiles := deps.Project.Env.GetFilePaths()
	if len(envFiles) == 1 && envFiles[0] == "." {
		// Special case: use .env in project directory as primary
		localEnvPath := filepath.Join(projectDir, ".env")
		if _, err := os.Stat(localEnvPath); err == nil {
			// .env exists in project directory - use it as primary (read-only)
			return localEnvPath, nil
		}
		// .env doesn't exist - will be created normally by template generation or other processes
		return "", nil
	}

	// For other array values, resolve normally (without projectEnvPath to avoid recursion)
	if len(envFiles) > 0 {
		resolvedPaths, err := ResolveEnvFiles(ws, deps, "", envFiles, "")
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
// Returns paths in order of precedence: global -> project.env -> project -> service
// Special case: if envFile is ".", it uses the serviceName as the env file name
// projectEnvPath is the resolved path from project.env (if project.env is ["."] and .env exists)
func ResolveEnvFiles(
	ws *workspace.Workspace,
	deps *config.Deps,
	serviceName string,
	envFiles []string,
	projectEnvPath string,
) ([]string, error) {
	var resolvedPaths []string

	// 1. Global env file (if useGlobal is true)
	if deps.Env.UseGlobal {
		globalPath := filepath.Join(ws.EnvDir, "global.env")
		if _, err := os.Stat(globalPath); err == nil {
			resolvedPaths = append(resolvedPaths, globalPath)
		}
	}

	// 2. Project.env file (if project.env is ["."] and .env exists in project directory)
	// This has highest precedence after global
	if projectEnvPath != "" {
		resolvedPaths = append(resolvedPaths, projectEnvPath)
	}

	// 3. Project-specific env files (from env.files)
	for _, envFile := range deps.Env.Files {
		var envPath string
		var err error

		// Check if it's a project file (starts with "projects/")
		if strings.HasPrefix(envFile, "projects/") {
			// Validate path to prevent path traversal
			envPath, err = pathvalidate.EnsurePathInBase(ws.EnvDir, envFile+".env")
			if err != nil {
				return nil, fmt.Errorf("invalid env file path '%s': %w", envFile, err)
			}
		} else if strings.HasPrefix(envFile, "services/") {
			// Skip service files here, handle them in step 3
			continue
		} else {
			// Assume it's a project name - validate path to prevent path traversal
			envPath, err = pathvalidate.EnsurePathInBase(ws.EnvDir, filepath.Join("projects", envFile+".env"))
			if err != nil {
				return nil, fmt.Errorf("invalid env file path '%s': %w", envFile, err)
			}
		}

		if _, err := os.Stat(envPath); err == nil {
			resolvedPaths = append(resolvedPaths, envPath)
		}
	}

	// 3. Service-specific env files
	// Priority: projects/{project}/services/{service}.env (project-specific) > services/{service}.env (shared)
	for _, envFile := range envFiles {
		var envPath string
		var err error
		var found bool
		var envServiceName string

		if strings.HasPrefix(envFile, "services/") {
			// Explicitly shared service (services/{service}) - only check shared location
			// Extract service name from "services/{service}"
			serviceName := strings.TrimPrefix(envFile, "services/")
			envPath, err = pathvalidate.EnsurePathInBase(ws.EnvDir, filepath.Join("services", serviceName+".env"))
			if err != nil {
				return nil, fmt.Errorf("invalid env file path '%s': %w", envFile, err)
			}
			if _, err := os.Stat(envPath); err == nil {
				resolvedPaths = append(resolvedPaths, envPath)
				found = true
			} else if !os.IsNotExist(err) {
				// Error only if it's not a "file doesn't exist" error (e.g., permission error)
				return nil, fmt.Errorf("failed to check env file %s: %w", envPath, err)
			}
		} else {
			// Special case: "." means use project .env (same as for infra)
			if envFile == "." {
				// If projectEnvPath is provided (from ResolveEnvFileForService), use it
				if projectEnvPath != "" {
					resolvedPaths = append(resolvedPaths, projectEnvPath)
					found = true
				}
				// If not found via projectEnvPath, fall through to service name resolution
				if !found {
					envServiceName = serviceName
				}
			} else {
				// Service name only - check project-specific first, then shared as fallback
				envServiceName = envFile
			}

			// Only try service-specific locations if "." wasn't resolved as project .env
			if !found {
				// First: try project-specific location: projects/{project}/services/{service}.env
				projectSpecificPath, err := pathvalidate.EnsurePathInBase(ws.EnvDir, filepath.Join("projects", deps.Project.Name, "services", envServiceName+".env"))
				if err != nil {
					return nil, fmt.Errorf("invalid env file path '%s': %w", envFile, err)
				}

				if _, err := os.Stat(projectSpecificPath); err == nil {
					resolvedPaths = append(resolvedPaths, projectSpecificPath)
					found = true
				} else if !os.IsNotExist(err) {
					// Error only if it's not a "file doesn't exist" error (e.g., permission error)
					return nil, fmt.Errorf("failed to check env file %s: %w", projectSpecificPath, err)
				}

				// Fallback: try shared location: services/{service}.env
				if !found {
					sharedPath, err := pathvalidate.EnsurePathInBase(ws.EnvDir, filepath.Join("services", envServiceName+".env"))
					if err != nil {
						return nil, fmt.Errorf("invalid env file path '%s': %w", envFile, err)
					}

					if _, err := os.Stat(sharedPath); err == nil {
						resolvedPaths = append(resolvedPaths, sharedPath)
						found = true
					} else if !os.IsNotExist(err) {
						// Error only if it's not a "file doesn't exist" error (e.g., permission error)
						return nil, fmt.Errorf("failed to check env file %s: %w", sharedPath, err)
					}
				}
			}
		}

		// If file doesn't exist, silently skip it (env files are optional)
	}

	return resolvedPaths, nil
}

// LoadFiles loads and merges environment variables from multiple files
// Later files override earlier ones (order of precedence)
func LoadFiles(filePaths []string) (map[string]string, error) {
	env := make(map[string]string)

	for _, filePath := range filePaths {
		fileEnv, err := loadSingleFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load %s: %w", filePath, err)
		}

		// Merge: later files override earlier ones
		for k, v := range fileEnv {
			env[k] = v
		}
	}

	return env, nil
}

// loadSingleFile loads environment variables from a single .env file
func loadSingleFile(filePath string) (map[string]string, error) {
	env := make(map[string]string)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid format at line %d in %s: expected KEY=VALUE", lineNum, filePath)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		env[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	return env, nil
}

// ResolveEnvFileForService resolves the env_file path(s) for a service.
// If envValue contains direct variables (object), it creates/updates the appropriate .env file.
// Returns either a single combined env file path or multiple paths
// projectDir is the directory where .raioz.json is located (for resolving "." to project .env)
// servicePath is the directory where the service is located (for Git services, this is the cloned directory)
func ResolveEnvFileForService(
	ws *workspace.Workspace,
	deps *config.Deps,
	serviceName string,
	envValue *config.EnvValue,
	projectDir string,
	servicePath string,
) (string, error) {
	// If envValue is nil, no env files to use
	if envValue == nil {
		return "", nil
	}

	// If envValue contains direct variables (object), create/update the .env file
	if envValue.IsObject && envValue.Variables != nil {
		envFilePath, err := CreateOrUpdateEnvFile(ws, deps, serviceName, envValue.Variables, servicePath)
		if err != nil {
			return "", fmt.Errorf("failed to create/update env file: %w", err)
		}
		return envFilePath, nil
	}

	// Otherwise, resolve file paths (array of strings)
	envFiles := envValue.GetFilePaths()
	if len(envFiles) == 0 {
		return "", nil
	}

	// Check if any envFile is "." - if so, resolve project .env (same as for infra)
	var projectEnvPath string
	var hasDotEnv bool
	for _, envFile := range envFiles {
		if envFile == "." {
			hasDotEnv = true
			// Special case: use .env in project directory (same as project.env)
			if projectDir != "" {
				localEnvPath := filepath.Join(projectDir, ".env")
				if _, statErr := os.Stat(localEnvPath); statErr == nil {
					// .env exists in project directory - include it in resolved paths
					projectEnvPath = localEnvPath
					break
				}
			}
		}
	}

	resolvedPaths, err := ResolveEnvFiles(ws, deps, serviceName, envFiles, projectEnvPath)
	if err != nil {
		return "", err
	}

	if len(resolvedPaths) == 0 {
		// No env files to use
		return "", nil
	}

	if len(resolvedPaths) == 1 {
		// Single file, use it directly
		return resolvedPaths[0], nil
	}

	// Multiple files, need to create a combined file
	// Special case: if "." was in the list, create/update .env in service directory (if Git service) or project directory
	// Otherwise, create .env.{serviceName} in service directory (if Git service) or workspace root
	var combinedPath string
	if hasDotEnv {
		// "." was specified - create/update .env in service directory (if Git service) or project directory
		// For Git services, create .env in the cloned service directory
		// For other services, create .env in project directory
		if servicePath != "" {
			// Service has a specific path (Git service) - create .env there
			combinedPath = filepath.Join(servicePath, ".env")
		} else if projectDir != "" {
			// Fallback to project directory
			combinedPath = filepath.Join(projectDir, ".env")
		} else {
			// Last resort: workspace root
			combinedPath = filepath.Join(ws.Root, fmt.Sprintf(".env.%s", serviceName))
		}
	} else {
		// Create combined file in workspace root
		combinedPath = filepath.Join(ws.Root, fmt.Sprintf(".env.%s", serviceName))
	}

	// Ensure directory exists (use 0700 for security - owner only)
	dirToCreate := filepath.Dir(combinedPath)
	if err := os.MkdirAll(dirToCreate, 0700); err != nil {
		return "", fmt.Errorf("failed to create directory for combined env file: %w", err)
	}

	// Load and merge all env files
	mergedEnv, err := LoadFiles(resolvedPaths)
	if err != nil {
		return "", err
	}

	// Write combined file (use 0600 permissions for security - contains secrets)
	file, err := os.OpenFile(combinedPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to create combined env file: %w", err)
	}
	defer file.Close()

	for key, value := range mergedEnv {
		// Escape value if it contains spaces or special characters
		escapedValue := value
		if strings.Contains(value, " ") || strings.Contains(value, "$") {
			escapedValue = fmt.Sprintf("\"%s\"", value)
		}
		if _, err := fmt.Fprintf(file, "%s=%s\n", key, escapedValue); err != nil {
			return "", fmt.Errorf("failed to write to combined env file: %w", err)
		}
	}

	return combinedPath, nil
}

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
			return "", fmt.Errorf("failed to create env directory: %w", err)
		}
	}

	// Ensure directory exists (use 0700 for security - owner only)
	if err := os.MkdirAll(filepath.Dir(envPath), 0700); err != nil {
		return "", fmt.Errorf("failed to create env directory: %w", err)
	}

	// Load existing variables if file exists
	existingVars := make(map[string]string)
	if _, err := os.Stat(envPath); err == nil {
		// File exists, load it
		loaded, err := loadSingleFile(envPath)
		if err != nil {
			return "", fmt.Errorf("failed to load existing env file: %w", err)
		}
		existingVars = loaded
	}

	// Merge: new variables override existing ones
	for key, value := range variables {
		existingVars[key] = value
	}

	// Write merged variables to file (use 0600 permissions for security - contains secrets)
	file, err := os.OpenFile(envPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to create env file: %w", err)
	}
	defer file.Close()

	// Write variables in sorted order for consistency
	keys := make([]string, 0, len(existingVars))
	for key := range existingVars {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := existingVars[key]
		// Escape value if it contains spaces or special characters
		escapedValue := value
		if strings.Contains(value, " ") || strings.Contains(value, "$") || strings.Contains(value, "\"") {
			escapedValue = fmt.Sprintf("\"%s\"", strings.ReplaceAll(value, "\"", "\\\""))
		}
		if _, err := fmt.Fprintf(file, "%s=%s\n", key, escapedValue); err != nil {
			return "", fmt.Errorf("failed to write to env file: %w", err)
		}
	}

	return envPath, nil
}

// EnsureEnvDirs creates the env directory structure if it doesn't exist
func EnsureEnvDirs(ws *workspace.Workspace) error {
	dirs := []string{
		ws.EnvDir,
		filepath.Join(ws.EnvDir, "services"),      // Shared services (explicitly shared)
		filepath.Join(ws.EnvDir, "projects"),       // Project-specific env files
	}

	// Use 0700 permissions (read/write/execute for owner only) for security
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create env directory %s: %w", dir, err)
		}
	}

	// Note: projects/{project}/services/ directories are created on-demand when needed
	// This avoids creating directories for all projects upfront

	return nil
}

// WriteGlobalEnvVariables writes/updates variables from env.variables to global.env
// If global.env exists, variables are merged (new values override existing ones)
func WriteGlobalEnvVariables(ws *workspace.Workspace, deps *config.Deps) error {
	// If no variables defined, skip
	if deps.Env.Variables == nil || len(deps.Env.Variables) == 0 {
		return nil
	}

	// Ensure env directory exists
	if err := EnsureEnvDirs(ws); err != nil {
		return fmt.Errorf("failed to ensure env directories: %w", err)
	}

	globalPath := filepath.Join(ws.EnvDir, "global.env")

	// Load existing variables if file exists
	existingVars := make(map[string]string)
	if _, err := os.Stat(globalPath); err == nil {
		// File exists, load it
		loaded, err := loadSingleFile(globalPath)
		if err != nil {
			return fmt.Errorf("failed to load existing global.env: %w", err)
		}
		existingVars = loaded
	}

	// Merge: new variables override existing ones
	for key, value := range deps.Env.Variables {
		existingVars[key] = value
	}

	// Write merged variables to file (use 0600 permissions for security - contains secrets)
	file, err := os.OpenFile(globalPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create global.env file: %w", err)
	}
	defer file.Close()

	// Write header comment
	if _, err := fmt.Fprintf(file, "# Variables de entorno globales\n"); err != nil {
		return fmt.Errorf("failed to write to global.env: %w", err)
	}
	if _, err := fmt.Fprintf(file, "# Este archivo se aplica a TODOS los servicios si useGlobal: true\n"); err != nil {
		return fmt.Errorf("failed to write to global.env: %w", err)
	}
	if _, err := fmt.Fprintf(file, "# Variables definidas en .raioz.json env.variables se escriben aquí\n\n"); err != nil {
		return fmt.Errorf("failed to write to global.env: %w", err)
	}

	// Write variables in sorted order for consistency
	keys := make([]string, 0, len(existingVars))
	for key := range existingVars {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := existingVars[key]
		// Escape value if it contains spaces or special characters
		escapedValue := value
		if strings.Contains(value, " ") || strings.Contains(value, "$") || strings.Contains(value, "\"") {
			escapedValue = fmt.Sprintf("\"%s\"", strings.ReplaceAll(value, "\"", "\\\""))
		}
		if _, err := fmt.Fprintf(file, "%s=%s\n", key, escapedValue); err != nil {
			return fmt.Errorf("failed to write to global.env: %w", err)
		}
	}

	return nil
}
