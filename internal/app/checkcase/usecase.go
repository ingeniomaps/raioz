package checkcase

import (
	"context"
	"io"

	"raioz/internal/domain/interfaces"
	"raioz/internal/state"
)

// Options contains options for the Check use case
type Options struct {
	ProjectName string
	ConfigPath  string
}

// Dependencies contains the dependencies needed by the check use case
type Dependencies struct {
	ConfigLoader interfaces.ConfigLoader
	Workspace    interfaces.WorkspaceManager
	StateManager interfaces.StateManager
}

// CheckResult holds the outcome of a check operation
type CheckResult struct {
	ConfigValid      bool
	ValidationErrors []string
	AlignmentIssues  []state.AlignmentIssue
	NoState          bool
	Output           string
	HasIssues        bool

	// YAMLMode is true when the check ran against a raioz.yaml project.
	// The CLI display handler uses this to skip legacy flows (state
	// alignment, "no state found" hint) that don't apply to yaml mode —
	// CheckYAML already printed its own per-service/dep results.
	YAMLMode bool
}

// UseCase handles the "check" use case
type UseCase struct {
	deps *Dependencies
	Out  io.Writer
}

// NewUseCase creates a new CheckUseCase with injected dependencies
func NewUseCase(deps *Dependencies) *UseCase {
	return &UseCase{
		deps: deps,
	}
}

// Execute runs config validation and alignment check, returns result
func (uc *UseCase) Execute(ctx context.Context, opts Options) (*CheckResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// Resolve workspace
	_, ws, err := uc.resolveWorkspace(opts)
	if err != nil {
		return nil, err
	}

	// Load current config
	currentDeps, err := uc.loadConfig(opts.ConfigPath)
	if err != nil {
		return nil, err
	}

	result := &CheckResult{ConfigValid: true}

	// Validate config (schema + business rules)
	validationErrors := uc.validateConfig(currentDeps)
	if len(validationErrors) > 0 {
		result.ConfigValid = false
		result.ValidationErrors = validationErrors
		result.HasIssues = true
	}

	// Check alignment if state exists
	if !uc.deps.StateManager.Exists(ws) {
		result.NoState = true
		return result, nil
	}

	issues, err := uc.checkAlignment(ws, currentDeps)
	if err != nil {
		return nil, err
	}

	result.AlignmentIssues = issues
	if state.HasCriticalIssues(issues) || state.HasWarningOrCriticalIssues(issues) {
		result.HasIssues = true
	}

	return result, nil
}
