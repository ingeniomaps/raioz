package app

import (
	"context"

	"raioz/internal/errors"
	"raioz/internal/i18n"
)

// StatusOptions contains options for the Status use case
type StatusOptions struct {
	ProjectName string
	ConfigPath  string
	JSON        bool
	// Services restricts the report to a subset. Empty means "everything"
	// (legacy behavior). When non-empty, services and deps not listed are
	// hidden from the report.
	Services []string
}

// StatusUseCase handles the "status" use case - showing project status
type StatusUseCase struct {
	deps *Dependencies
}

// NewStatusUseCase creates a new StatusUseCase with injected dependencies
func NewStatusUseCase(deps *Dependencies) *StatusUseCase {
	return &StatusUseCase{
		deps: deps,
	}
}

// Execute executes the status use case.
//
// YAML is the only config shape the loader can produce: ADR-038 turned
// `config.LoadDeps` into a hard error for `.raioz.json`, so a project that
// doesn't resolve here cannot be loaded by any other path either. The
// legacy branch that used to follow this dispatch — compose queries, host
// process info, the JSON and human printers — was therefore unreachable,
// and testable only by mocking a config loader that returned a shape the
// real one no longer produces.
func (uc *StatusUseCase) Execute(ctx context.Context, opts StatusOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}

	proj := ResolveYAMLProject(uc.deps, opts.ConfigPath)
	if proj == nil {
		return errors.New(
			errors.ErrCodeInvalidConfig,
			i18n.T("error.no_project"),
		).WithSuggestion(
			i18n.T("error.no_project_suggestion"),
		)
	}
	return uc.StatusYAML(ctx, proj, opts.Services)
}
