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

// EnvTemplateNames lists the possible template file names to search for
var EnvTemplateNames = []string{
	".env.example",
	".env.template",
	".env-example",
	".env-template",
}

// GenerateEnvFromTemplate generates a .env file from a template if found
// and injects variables from resolved env files (global, project.env, project, service)
// projectEnvPath is the resolved path from project.env (if project.env is ["."] and .env exists)
// projectDir is the directory where .raioz.json is located (for resolving "." to project .env)
func GenerateEnvFromTemplate(
	ws *workspace.Workspace,
	deps *config.Deps,
	serviceName string,
	servicePath string,
	svc config.Service,
	projectEnvPath string,
	projectDir string,
) error {
	// Find template file
	var templatePath string
	for _, templateName := range EnvTemplateNames {
		candidatePath := filepath.Join(servicePath, templateName)
		if _, err := os.Stat(candidatePath); err == nil {
			templatePath = candidatePath
			break
		}
	}

	// If no template found, skip
	if templatePath == "" {
		return nil
	}

	// Special case: if project.env is ["."] and .env exists in project directory,
	// don't generate .env from template (use existing .env as primary)
	if projectEnvPath != "" && serviceName == deps.Project.Name {
		// This is the project itself, and project.env is ["."] with existing .env
		// Don't generate from template - use existing .env
		return nil
	}

	// Log that we're generating .env from template
	fmt.Printf("📝 Generating .env from template for service '%s'...\n", serviceName)

	// Read template content
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to read env template file").
			WithContext("file", templatePath).
			WithContext("service", serviceName).
			WithSuggestion("Check that the template file exists and is readable").
			WithError(err)
	}

	// Resolve ALL env files (global, project, service) to get variables
	// This matches the same resolution order used by ResolveEnvFileForService
	var allResolvedPaths []string

	// 1. Global env file (if useGlobal is true)
	if deps.Env.UseGlobal {
		globalPath := filepath.Join(ws.EnvDir, "global.env")
		if _, err := os.Stat(globalPath); err == nil {
			allResolvedPaths = append(allResolvedPaths, globalPath)
		}
	}

	// 2. Project.env file (if project.env is ["."] and .env exists in project directory)
	// IMPORTANT: Only include projectEnvPath for the project itself, NOT for services
	// Services should NOT inherit the project's .env file
	if projectEnvPath != "" && serviceName == deps.Project.Name {
		// This is the project itself, include project .env
		allResolvedPaths = append(allResolvedPaths, projectEnvPath)
	}

	// 3. Project-specific env files (from env.files)
	// IMPORTANT: Only include project-specific files for the project itself, NOT for services
	// Services should only use their own env files
	if serviceName == deps.Project.Name {
		for _, envFile := range deps.Env.Files {
			var envPath string
			var err error

			if strings.HasPrefix(envFile, "projects/") {
				envPath, err = pathvalidate.EnsurePathInBase(ws.EnvDir, envFile+".env")
				if err != nil {
					continue // Skip invalid paths
				}
			} else if strings.HasPrefix(envFile, "services/") {
				// Skip service files here, they're handled in step 4
				continue
			} else {
				// Assume it's a project name
				envPath, err = pathvalidate.EnsurePathInBase(ws.EnvDir, filepath.Join("projects", envFile+".env"))
				if err != nil {
					continue // Skip invalid paths
				}
			}

			if envPath != "" {
				if _, err := os.Stat(envPath); err == nil {
					allResolvedPaths = append(allResolvedPaths, envPath)
				}
			}
		}
	}

	// 4. Service-specific env files (if service has env config)
	var directServiceVars map[string]string
	if svc.Env != nil {
		if svc.Env.IsObject && svc.Env.Variables != nil {
			// If env is an object, use variables directly (they're already in memory)
			directServiceVars = svc.Env.Variables
			// Also try to load from file if it exists (for merging)
			envFilePath, err := ResolveEnvFileForService(ws, deps, serviceName, svc.Env, projectDir, servicePath)
			if err == nil && envFilePath != "" {
				allResolvedPaths = append(allResolvedPaths, envFilePath)
			}
		} else {
			// If env is an array, resolve all files
			// IMPORTANT: Do NOT pass projectEnvPath for services - they should not inherit project .env
			serviceEnvFiles := svc.Env.GetFilePaths()
			if len(serviceEnvFiles) > 0 {
				// For services, pass empty string for projectEnvPath (only project itself should use it)
				serviceProjectEnvPath := ""
				if serviceName == deps.Project.Name {
					// Only use projectEnvPath if this is the project itself
					serviceProjectEnvPath = projectEnvPath
				}
				resolvedPaths, err := ResolveEnvFiles(
					ws, deps, serviceName, serviceEnvFiles,
					serviceProjectEnvPath, false, projectDir,
				)
				if err == nil {
					allResolvedPaths = append(allResolvedPaths, resolvedPaths...)
				}
			}
		}
	}

	// Load all resolved env files (in order of precedence)
	var envVars map[string]string
	if len(allResolvedPaths) > 0 {
		loaded, err := LoadFiles(allResolvedPaths)
		if err != nil {
			return raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to load env files for template generation").
				WithContext("service", serviceName).
				WithSuggestion("Check that all referenced env files exist and have valid format").
				WithError(err)
		}
		envVars = loaded
	}

	// If no env vars resolved, use empty map
	if envVars == nil {
		envVars = make(map[string]string)
	}

	// Merge direct service variables (highest precedence - they override everything)
	if directServiceVars != nil {
		for key, value := range directServiceVars {
			envVars[key] = value
		}
	}

	// Process template: replace ${VAR} or $VAR with actual values
	processedContent := processTemplate(string(templateContent), envVars)

	// Parse processed content to get variables from template
	templateVars := parseEnvContent(processedContent)

	// Merge resolved variables with template variables
	// Resolved variables (from env files) have precedence and will override template values
	newVars := make(map[string]string)
	// First, add all template variables
	for k, v := range templateVars {
		newVars[k] = v
	}
	// Then, override/add resolved variables (they have precedence)
	for k, v := range envVars {
		newVars[k] = v
	}

	// Check if .env file already exists
	envFilePath := filepath.Join(servicePath, ".env")
	envExists := false
	var existingVars map[string]string
	if _, err := os.Stat(envFilePath); err == nil {
		envExists = true
		// Load existing .env file
		existingContent, err := os.ReadFile(envFilePath)
		if err == nil {
			existingVars = parseEnvContent(string(existingContent))
		}
	}

	// If .env exists, merge with global + service-specific variables
	// IMPORTANT: Keep all existing variables, only add/update with global + service-specific
	if envExists && existingVars != nil {
		// Start with existing variables (preserve all existing)
		finalVars := make(map[string]string)
		for k, v := range existingVars {
			finalVars[k] = v
		}

		// Merge with resolved variables (global + service-specific)
		// This will add new variables and update existing ones with values from global + service files
		for key, value := range envVars {
			// Only update if the variable comes from global or service-specific files
			// (not from project .env, which we already excluded above)
			finalVars[key] = value
		}

		// Merge direct service variables (highest precedence)
		if directServiceVars != nil {
			for key, value := range directServiceVars {
				finalVars[key] = value
			}
		}

		// Write merged .env file (preserving existing + adding global + service-specific)
		if err := writeEnvFile(envFilePath, finalVars); err != nil {
			return raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to write .env file").
				WithContext("file", envFilePath).
				WithContext("service", serviceName).
				WithSuggestion("Check file permissions and disk space").
				WithError(err)
		}
		fmt.Printf("✅ .env file updated for service '%s' (merged with global + service-specific variables)\n", serviceName)
	} else {
		// .env doesn't exist, create it from template + global + service-specific
		if err := writeEnvFile(envFilePath, newVars); err != nil {
			return raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to write .env file from template").
				WithContext("file", envFilePath).
				WithContext("service", serviceName).
				WithSuggestion("Check file permissions and disk space").
				WithError(err)
		}
		fmt.Printf("✅ .env file created for service '%s' with %d variables\n", serviceName, len(newVars))
	}

	return nil
}

// parseEnvContent parses env file content into a map of key=value pairs
func parseEnvContent(content string) map[string]string {
	vars := make(map[string]string)
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			// Remove quotes if present
			if len(value) >= 2 {
				if (value[0] == '"' && value[len(value)-1] == '"') ||
					(value[0] == '\'' && value[len(value)-1] == '\'') {
					value = value[1 : len(value)-1]
				}
			}

			vars[key] = value
		}
	}

	return vars
}

// writeEnvFile writes variables to an .env file
func writeEnvFile(filePath string, vars map[string]string) error {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to create .env file").
			WithContext("file", filePath).
			WithSuggestion("Check file permissions and that the directory exists").
			WithError(err)
	}
	defer file.Close()

	// Write variables (preserve order by sorting keys)
	keys := make([]string, 0, len(vars))
	for key := range vars {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := vars[key]
		// Escape value if it contains spaces or special characters
		escapedValue := value
		if strings.Contains(value, " ") || strings.Contains(value, "$") || strings.Contains(value, "\"") {
			escapedValue = fmt.Sprintf("\"%s\"", strings.ReplaceAll(value, "\"", "\\\""))
		}
		if _, err := fmt.Fprintf(file, "%s=%s\n", key, escapedValue); err != nil {
			return raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to write to .env file").
				WithContext("file", filePath).
				WithContext("key", key).
				WithSuggestion("Check disk space and file permissions").
				WithError(err)
		}
	}

	return nil
}

// processTemplate replaces ${VAR} or $VAR patterns with actual values from envVars
func processTemplate(template string, envVars map[string]string) string {
	result := template

	// Replace ${VAR} patterns first (more specific)
	for key, value := range envVars {
		placeholder := fmt.Sprintf("${%s}", key)
		result = strings.ReplaceAll(result, placeholder, value)
		// Also handle ${VAR:-default} syntax (use default if var not set)
		placeholderWithDefault := fmt.Sprintf("${%s:-", key)
		if strings.Contains(result, placeholderWithDefault) {
			// Find and replace ${VAR:-default} patterns
			start := strings.Index(result, placeholderWithDefault)
			if start != -1 {
				end := strings.Index(result[start:], "}")
				if end != -1 {
					fullPattern := result[start : start+end+1]
					// Extract default value
					defaultStart := start + len(placeholderWithDefault)
					defaultEnd := start + end
					defaultValue := result[defaultStart:defaultEnd]
					// Replace with actual value (or default if not set)
					result = strings.ReplaceAll(result, fullPattern, value)
					// If value is empty, use default
					if value == "" {
						result = strings.ReplaceAll(result, fullPattern, defaultValue)
					}
				}
			}
		}
	}

	// Replace $VAR patterns (simple, no braces)
	// Only replace if it's a complete word (not part of another variable)
	for key, value := range envVars {
		// Match $VAR at word boundaries
		placeholder := fmt.Sprintf("$%s", key)
		// Simple replacement for now (can be enhanced with regex if needed)
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result
}
