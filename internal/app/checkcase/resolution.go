package checkcase

import (
	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
	"raioz/internal/i18n"
)

// resolveWorkspace resolves the workspace and determines the project name
func (uc *UseCase) resolveWorkspace(opts Options) (string, *interfaces.Workspace, error) {
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
				i18n.T("error.check_could_not_determine_project"),
			).WithSuggestion(
				i18n.T("error.check_could_not_determine_project_suggestion"),
			)
		}
	} else {
		deps, _, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
		if deps != nil && deps.Project.Name == projectName {
			workspaceName = deps.GetWorkspaceName()
		} else {
			workspaceName = projectName
		}
	}

	ws, err := uc.deps.Workspace.Resolve(workspaceName)
	if err != nil {
		return "", nil, errors.New(
			errors.ErrCodeWorkspaceError,
			i18n.T("error.check_workspace_resolve"),
		).WithSuggestion(
			i18n.T("error.check_workspace_resolve_suggestion"),
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
			i18n.T("error.check_load_config"),
		).WithSuggestion(
			i18n.T("error.check_load_config_suggestion"),
		).WithError(err)
	}
	return currentDeps, nil
}
