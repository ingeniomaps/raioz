package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
)

// executeLegacy runs CI for legacy JSON config.
func (uc *CIUseCase) executeLegacy(
	opts CIOptions,
	result *CIResult,
) (*CIResult, error) {
	configDeps, warnings, err := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
	if err != nil {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "load_config",
			Status:  "failed",
			Message: err.Error(),
		})
		result.Errors = append(
			result.Errors,
			fmt.Sprintf("Failed to load config: %v", err),
		)
		return result, nil
	}

	result.Warnings = append(result.Warnings, warnings...)

	result.Validations = append(result.Validations, ValidationResult{
		Check:  "load_config",
		Status: "passed",
	})

	if err := uc.validateFast(configDeps); err != nil {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "validation",
			Status:  "failed",
			Message: err.Error(),
		})
		result.Errors = append(
			result.Errors,
			fmt.Sprintf("Validation failed: %v", err),
		)
		return result, nil
	}

	result.Validations = append(result.Validations, ValidationResult{
		Check:  "validation",
		Status: "passed",
	})

	if err := uc.deps.ConfigLoader.ValidateFeatureFlags(configDeps); err != nil {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "feature_flags",
			Status:  "failed",
			Message: err.Error(),
		})
		result.Errors = append(
			result.Errors,
			fmt.Sprintf("Feature flags validation failed: %v", err),
		)
		return result, nil
	}

	result.Validations = append(result.Validations, ValidationResult{
		Check:  "feature_flags",
		Status: "passed",
	})

	if opts.OnlyValidate {
		result.Success = true
		result.Message = "All validations passed"
		return result, nil
	}

	if err := uc.executeSetup(configDeps, opts, result); err != nil {
		return result, nil
	}

	result.Success = true
	result.Message = "CI run completed successfully"

	if opts.Ephemeral && !opts.Keep {
		result.Message += " (ephemeral environment will be cleaned up)"
	}

	return result, nil
}

// executeSetup performs the setup phase of CI command (legacy mode).
func (uc *CIUseCase) executeSetup(
	deps *config.Deps,
	opts CIOptions,
	result *CIResult,
) error {
	envVars := make(map[string]string)
	for _, key := range getEnviron() {
		pair := strings.SplitN(key, "=", 2)
		if len(pair) == 2 {
			envVars[pair[0]] = pair[1]
		}
	}

	profile := ""
	var mockServices []string
	deps, mockServices = uc.deps.ConfigLoader.FilterByFeatureFlags(
		deps, profile, envVars,
	)

	if len(mockServices) > 0 {
		result.Warnings = append(
			result.Warnings,
			fmt.Sprintf(
				"Using mocks for services: %s",
				strings.Join(mockServices, ", "),
			),
		)
	}

	projectName := deps.Project.Name
	workspaceName := projectName
	if opts.Ephemeral {
		if opts.JobID != "" {
			workspaceName = fmt.Sprintf("%s-ci-%s", projectName, opts.JobID)
		} else {
			workspaceName = fmt.Sprintf(
				"%s-ci-%d", projectName, time.Now().Unix(),
			)
		}
		result.Workspace = workspaceName
	}

	ws, err := uc.deps.Workspace.Resolve(workspaceName)
	if err != nil {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "workspace",
			Status:  "failed",
			Message: err.Error(),
		})
		result.Errors = append(
			result.Errors,
			fmt.Sprintf("Failed to resolve workspace: %v", err),
		)
		return err
	}

	if err := uc.deps.Validator.CheckWorkspacePermissions(ws.Root); err != nil {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "workspace_permissions",
			Status:  "failed",
			Message: err.Error(),
		})
		result.Errors = append(
			result.Errors,
			fmt.Sprintf("Workspace permissions check failed: %v", err),
		)
		return err
	}

	result.Validations = append(result.Validations, ValidationResult{
		Check:  "workspace",
		Status: "passed",
	})

	lockInstance, err := uc.deps.LockManager.Acquire(ws)
	if err != nil {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "lock",
			Status:  "failed",
			Message: err.Error(),
		})
		result.Errors = append(
			result.Errors,
			fmt.Sprintf("Failed to acquire lock: %v", err),
		)
		return err
	}

	if opts.Ephemeral && !opts.Keep {
		defer func() {
			uc.cleanupEphemeral(ws, projectName)
		}()
	}

	defer lockInstance.Release()

	if err := uc.validateCIPorts(deps, ws, workspaceName, result); err != nil {
		return err
	}
	if err := uc.validateCIImages(deps, opts, result); err != nil {
		return err
	}
	if err := uc.ensureCINetwork(deps, result); err != nil {
		return err
	}
	if err := uc.ensureCIVolumes(deps, result); err != nil {
		return err
	}
	if err := uc.resolveCIGit(deps, ws, opts, result); err != nil {
		return err
	}

	projectDir, err := filepath.Abs(filepath.Dir(opts.ConfigPath))
	if err != nil {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "compose",
			Status:  "failed",
			Message: fmt.Sprintf("Failed to get project dir: %v", err),
		})
		result.Errors = append(
			result.Errors,
			fmt.Sprintf("Failed to get project directory: %v", err),
		)
		return err
	}

	composePath, _, err := uc.deps.DockerRunner.GenerateCompose(
		deps, ws, projectDir,
	)
	if err != nil {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "compose",
			Status:  "failed",
			Message: err.Error(),
		})
		result.Errors = append(
			result.Errors,
			fmt.Sprintf("Compose generation failed: %v", err),
		)
		return err
	}

	result.ComposeFile = composePath
	result.Validations = append(result.Validations, ValidationResult{
		Check:  "compose",
		Status: "passed",
	})

	if !opts.SkipBuild {
		if err := uc.deps.DockerRunner.Up(composePath); err != nil {
			result.Validations = append(result.Validations, ValidationResult{
				Check:   "start_services",
				Status:  "failed",
				Message: err.Error(),
			})
			result.Errors = append(
				result.Errors,
				fmt.Sprintf("Failed to start services: %v", err),
			)
			return err
		}

		result.Validations = append(result.Validations, ValidationResult{
			Check:  "start_services",
			Status: "passed",
		})

		if err := uc.deps.StateManager.Save(ws, deps); err != nil {
			result.Warnings = append(
				result.Warnings,
				fmt.Sprintf("Failed to save state: %v", err),
			)
		} else {
			result.StateFile = uc.deps.Workspace.GetStatePath(ws)
		}
	} else {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "start_services",
			Status:  "skipped",
			Message: "Service startup skipped (--skip-build)",
		})
	}

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

// cleanupEphemeral cleans up an ephemeral CI environment.
func (uc *CIUseCase) cleanupEphemeral(
	ws *interfaces.Workspace,
	_ string,
) {
	composePath := uc.deps.Workspace.GetComposePath(ws)

	if _, err := os.Stat(composePath); err == nil {
		if err := uc.deps.DockerRunner.Down(composePath); err != nil {
			fmt.Fprintf(
				os.Stderr,
				"Warning: Failed to stop ephemeral environment: %v\n",
				err,
			)
		}
	}

	statePath := uc.deps.Workspace.GetStatePath(ws)
	if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Warning: Failed to remove state file: %v\n", err)
	}

	if err := os.RemoveAll(ws.Root); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Warning: Failed to remove ephemeral workspace: %v\n",
			err,
		)
	}
}
