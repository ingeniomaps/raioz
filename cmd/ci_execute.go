package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"raioz/internal/config"
	"raioz/internal/docker"
	"raioz/internal/lock"
	"raioz/internal/state"
	"raioz/internal/validate"
	"raioz/internal/workspace"
)

// executeCICommand performs the CI command execution
func executeCICommand() error {
	startTime := time.Now()

	// Build result structure for JSON output
	result := CIResult{
		Success:     false,
		StartTime:   startTime.Format(time.RFC3339),
		Validations: []ValidationResult{},
		Errors:      []string{},
		Warnings:    []string{},
	}

	defer func() {
		result.EndTime = time.Now().Format(time.RFC3339)
		result.Duration = time.Since(startTime).Seconds()

		// Always output JSON at the end
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			// If JSON encoding fails, output simple error
			fmt.Fprintf(os.Stderr, "Failed to encode result: %v\n", err)
		}

		// Exit with appropriate code
		if !result.Success {
			os.Exit(1)
		}
	}()

	// Fast preflight checks (skip non-critical checks)
	if err := validateFastPreflight(); err != nil {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "preflight",
			Status:  "failed",
			Message: err.Error(),
		})
		result.Errors = append(result.Errors, fmt.Sprintf("Preflight check failed: %v", err))
		return nil // Don't return error, output JSON instead
	}

	result.Validations = append(result.Validations, ValidationResult{
		Check:  "preflight",
		Status: "passed",
	})

	// Load configuration
	deps, warnings, err := config.LoadDeps(configPath)
	if err != nil {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "load_config",
			Status:  "failed",
			Message: err.Error(),
		})
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to load config: %v", err))
		return nil
	}

	// Add deprecation warnings
	for _, warning := range warnings {
		result.Warnings = append(result.Warnings, warning)
	}

	result.Validations = append(result.Validations, ValidationResult{
		Check:  "load_config",
		Status: "passed",
	})

	// Fast validation (skip compatibility checks in CI)
	if err := validateFast(deps); err != nil {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "validation",
			Status:  "failed",
			Message: err.Error(),
		})
		result.Errors = append(result.Errors, fmt.Sprintf("Validation failed: %v", err))
		return nil
	}

	result.Validations = append(result.Validations, ValidationResult{
		Check:  "validation",
		Status: "passed",
	})

	// Validate feature flags
	if err := config.ValidateFeatureFlags(deps); err != nil {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "feature_flags",
			Status:  "failed",
			Message: err.Error(),
		})
		result.Errors = append(result.Errors, fmt.Sprintf("Feature flags validation failed: %v", err))
		return nil
	}

	result.Validations = append(result.Validations, ValidationResult{
		Check:  "feature_flags",
		Status: "passed",
	})

	// If only validation is requested, exit early
	if ciOnlyValidate {
		result.Success = true
		result.Message = "All validations passed"
		return nil
	}

	// Continue with setup...
	if err := executeCISetup(deps, &result); err != nil {
		// Error already added to result
		return nil
	}

	result.Success = true
	result.Message = "CI run completed successfully"

	if ciEphemeral && !ciKeep {
		result.Message += " (ephemeral environment will be cleaned up)"
	}

	return nil
}

// executeCISetup performs the setup phase of CI command
func executeCISetup(deps *config.Deps, result *CIResult) error {
	// Load environment variables for feature flags
	envVars := make(map[string]string)
	for _, key := range os.Environ() {
		pair := strings.SplitN(key, "=", 2)
		if len(pair) == 2 {
			envVars[pair[0]] = pair[1]
		}
	}

	// Filter by profile and feature flags (CI doesn't use profiles by default)
	profile := ""
	var mockServices []string
	deps, mockServices = config.FilterByFeatureFlags(deps, profile, envVars)

	if len(mockServices) > 0 {
		result.Warnings = append(
			result.Warnings,
			fmt.Sprintf("Using mocks for services: %s", strings.Join(mockServices, ", ")),
		)
	}

	// Determine workspace name (ephemeral or regular)
	projectName := deps.Project.Name
	workspaceName := projectName
	if ciEphemeral {
		if ciJobID != "" {
			workspaceName = fmt.Sprintf("%s-ci-%s", projectName, ciJobID)
		} else {
			workspaceName = fmt.Sprintf("%s-ci-%d", projectName, time.Now().Unix())
		}
		result.Workspace = workspaceName
	}

	ws, err := workspace.Resolve(workspaceName)
	if err != nil {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "workspace",
			Status:  "failed",
			Message: err.Error(),
		})
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to resolve workspace: %v", err))
		return err
	}

	// Check workspace permissions
	if err := validate.CheckWorkspacePermissions(ws.Root); err != nil {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "workspace_permissions",
			Status:  "failed",
			Message: err.Error(),
		})
		result.Errors = append(result.Errors, fmt.Sprintf("Workspace permissions check failed: %v", err))
		return err
	}

	result.Validations = append(result.Validations, ValidationResult{
		Check:  "workspace",
		Status: "passed",
	})

	// Acquire lock
	lockInstance, err := lock.Acquire(ws)
	if err != nil {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "lock",
			Status:  "failed",
			Message: err.Error(),
		})
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to acquire lock: %v", err))
		return err
	}

	// Schedule cleanup if ephemeral
	if ciEphemeral && !ciKeep {
		defer func() {
			cleanupEphemeralEnvironment(ws, projectName)
		}()
	}

	// Cleanup lock
	defer lockInstance.Release()

	// Validate ports
	if err := validateCIPorts(deps, ws, workspaceName, result); err != nil {
		return err
	}

	// Validate images
	if err := validateCIImages(deps, result); err != nil {
		return err
	}

	// Ensure network
	if err := ensureCINetwork(deps, result); err != nil {
		return err
	}

	// Ensure volumes
	if err := ensureCIVolumes(deps, result); err != nil {
		return err
	}

	// Resolve git repositories
	if err := resolveCIGit(deps, ws, result); err != nil {
		return err
	}

	// Get project directory (where .raioz.json is located)
	projectDir, err := filepath.Abs(filepath.Dir(configPath))
	if err != nil {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "compose",
			Status:  "failed",
			Message: fmt.Sprintf("Failed to get project directory: %v", err),
		})
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to get project directory: %v", err))
		return err
	}

	// Generate compose file
	composePath, err := docker.GenerateCompose(deps, ws, projectDir)
	if err != nil {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "compose",
			Status:  "failed",
			Message: err.Error(),
		})
		result.Errors = append(result.Errors, fmt.Sprintf("Compose generation failed: %v", err))
		return err
	}

	result.ComposeFile = composePath
	result.Validations = append(result.Validations, ValidationResult{
		Check:  "compose",
		Status: "passed",
	})

	// Start services (skip if ciSkipBuild)
	if !ciSkipBuild {
		if err := docker.Up(composePath); err != nil {
			result.Validations = append(result.Validations, ValidationResult{
				Check:   "start_services",
				Status:  "failed",
				Message: err.Error(),
			})
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to start services: %v", err))
			return err
		}

		result.Validations = append(result.Validations, ValidationResult{
			Check:  "start_services",
			Status: "passed",
		})

		// Save state
		if err := state.Save(ws, deps); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to save state: %v", err))
		} else {
			result.StateFile = workspace.GetStatePath(ws)
		}
	} else {
		result.Validations = append(result.Validations, ValidationResult{
			Check:  "start_services",
			Status: "skipped",
			Message: "Service startup skipped (--skip-build)",
		})
	}

	// Collect service names for result
	var serviceNames []string
	for name := range deps.Services {
		serviceNames = append(serviceNames, name)
	}
	var infraNames []string
	for name := range deps.Infra {
		infraNames = append(infraNames, name)
	}

	result.Services = serviceNames
	result.Infra = infraNames

	return nil
}
