package checkcase

import (
	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
)

// resolveWorkspace resolves the workspace and determines the project name
func (uc *UseCase) resolveWorkspace(opts Options) (string, *interfaces.Workspace, error) {
	// Try to determine project name and workspace
	projectName := opts.ProjectName
	var workspaceName string
	if projectName == "" {
		deps, _, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
		if deps != nil {
			projectName = deps.Project.Name
			workspaceName = deps.GetWorkspaceName()
		} else {
			return "", nil, errors.New(
				errors.ErrCodeInvalidConfig,
				"Could not determine project name",
			).WithSuggestion(
				"Please provide --config or --project flag to specify the project.",
			)
		}
	} else {
		// If project name comes from CLI, load config to get workspace name
		deps, _, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
		if deps != nil && deps.Project.Name == projectName {
			workspaceName = deps.GetWorkspaceName()
		} else {
			// Fallback: use project name as workspace (backward compatibility)
			workspaceName = projectName
		}
	}

	// Resolve workspace using workspace name
	ws, err := uc.deps.Workspace.Resolve(workspaceName)
	if err != nil {
		return "", nil, errors.New(
			errors.ErrCodeWorkspaceError,
			"Failed to resolve workspace",
		).WithSuggestion(
			"Check that the project name is correct. " +
				"Verify workspace directories exist and are accessible.",
		).WithContext("project", projectName).WithError(err)
	}

	return projectName, ws, nil
}

// loadConfig loads the current configuration
func (uc *UseCase) loadConfig(configPath string) (*config.Deps, error) {
	currentDeps, _, err := uc.deps.ConfigLoader.LoadDeps(configPath)
	if err != nil {
		return nil, errors.New(
			errors.ErrCodeInvalidConfig,
			"Failed to load config",
		).WithSuggestion(
			"Ensure .raioz.json exists and is valid JSON. " +
				"Use --config flag to specify a different path if needed.",
		).WithError(err)
	}
	return currentDeps, nil
}
