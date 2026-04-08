package checkcase

import (
	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/state"
	"raioz/internal/validate"
	workspacepkg "raioz/internal/workspace"
)

// validateConfig validates the configuration and returns any errors
func (uc *UseCase) validateConfig(deps *config.Deps) []string {
	var errs []string

	if err := validate.ValidateSchema(deps); err != nil {
		errs = append(errs, err.Error())
	}
	if err := validate.ValidateProject(deps); err != nil {
		errs = append(errs, err.Error())
	}
	if err := validate.ValidateServices(deps); err != nil {
		errs = append(errs, err.Error())
	}
	if err := validate.ValidateInfra(deps); err != nil {
		errs = append(errs, err.Error())
	}
	if err := validate.ValidateDependencies(deps); err != nil {
		errs = append(errs, err.Error())
	}

	return errs
}

// checkAlignment compares current config against saved state
func (uc *UseCase) checkAlignment(ws *interfaces.Workspace, currentDeps *config.Deps) ([]state.AlignmentIssue, error) {
	wsConcrete := (*workspacepkg.Workspace)(ws)

	issues, err := state.CheckAlignment(wsConcrete, currentDeps)
	if err != nil {
		return nil, errors.New(
			errors.ErrCodeStateLoadError,
			i18n.T("error.check_alignment"),
		).WithSuggestion(
			i18n.T("error.check_alignment_suggestion"),
		).WithError(err)
	}

	return issues, nil
}
