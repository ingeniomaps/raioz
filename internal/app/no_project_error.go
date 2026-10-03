package app

import (
	"os"

	"raioz/internal/errors"
	"raioz/internal/i18n"
)

// noProjectError is what a command returns when it could not resolve the
// project to act on. When there IS a config file and it simply fails to
// load, the answer is why it fails — "could not determine the project" on
// top of a broken raioz.yaml sends the user looking in the wrong place.
func noProjectError(deps *Dependencies, configPath string) error {
	if configPath == "" || configPath == ":auto:" {
		configPath = findConfigFile()
	}
	if configPath != "" && deps != nil && deps.ConfigLoader != nil {
		if _, statErr := os.Stat(configPath); statErr == nil {
			if _, _, loadErr := deps.ConfigLoader.LoadDeps(configPath); loadErr != nil {
				return errors.New(
					errors.ErrCodeInvalidConfig,
					i18n.T("error.config_invalid", configPath, loadErr.Error()),
				).WithSuggestion(i18n.T("error.config_invalid_suggestion"))
			}
		}
	}
	return errors.New(
		errors.ErrCodeInvalidConfig,
		i18n.T("error.no_project"),
	).WithSuggestion(i18n.T("error.no_project_suggestion"))
}
