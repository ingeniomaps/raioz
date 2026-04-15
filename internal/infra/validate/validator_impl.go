package validate

import (
	"context"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/validate"
	"raioz/internal/workspace"
)

// Ensure ValidatorImpl implements interfaces.Validator
var _ interfaces.Validator = (*ValidatorImpl)(nil)

// ValidatorImpl is the concrete implementation of Validator
type ValidatorImpl struct{}

// NewValidator creates a new Validator implementation
func NewValidator() interfaces.Validator {
	return &ValidatorImpl{}
}

// ValidateBeforeUp validates configuration before running up command
func (v *ValidatorImpl) ValidateBeforeUp(ctx interface{}, deps *config.Deps, ws interface{}) error {
	ctxTyped, ok := ctx.(context.Context)
	if !ok {
		ctxTyped = context.Background()
	}
	wsTyped, ok := ws.(*workspace.Workspace)
	if !ok {
		return validate.All(deps)
	}
	return validate.ValidateBeforeUp(ctxTyped, deps, wsTyped)
}

// ValidateBeforeDown validates configuration before running down command
func (v *ValidatorImpl) ValidateBeforeDown(ctx interface{}, ws interface{}) error {
	ctxTyped, ok := ctx.(context.Context)
	if !ok {
		ctxTyped = context.Background()
	}
	wsTyped, ok := ws.(*workspace.Workspace)
	if !ok {
		return nil
	}
	return validate.ValidateBeforeDown(ctxTyped, wsTyped)
}

// All validates the entire configuration
func (v *ValidatorImpl) All(deps *config.Deps) error {
	return validate.All(deps)
}

// CheckDockerInstalled checks if Docker is installed
func (v *ValidatorImpl) CheckDockerInstalled() error {
	return validate.CheckDockerInstalled()
}

// CheckDockerRunning checks if Docker daemon is running
func (v *ValidatorImpl) CheckDockerRunning() error {
	return validate.CheckDockerRunning()
}

// ValidateSchema validates the configuration schema
func (v *ValidatorImpl) ValidateSchema(deps *config.Deps) error {
	return validate.ValidateSchema(deps)
}

// ValidateProject validates the project configuration
func (v *ValidatorImpl) ValidateProject(deps *config.Deps) error {
	return validate.ValidateProject(deps)
}

// ValidateServices validates services configuration
func (v *ValidatorImpl) ValidateServices(deps *config.Deps) error {
	return validate.ValidateServices(deps)
}

// ValidateInfra validates infra configuration
func (v *ValidatorImpl) ValidateInfra(deps *config.Deps) error {
	return validate.ValidateInfra(deps)
}

// ValidateDependencies validates dependencies configuration
func (v *ValidatorImpl) ValidateDependencies(deps *config.Deps) error {
	return validate.ValidateDependencies(deps)
}

// CheckWorkspacePermissions checks workspace permissions
func (v *ValidatorImpl) CheckWorkspacePermissions(workspacePath string) error {
	return validate.CheckWorkspacePermissions(workspacePath)
}

// PreflightCheckWithContext runs preflight checks (Docker, Git, disk space)
func (v *ValidatorImpl) PreflightCheckWithContext(ctx context.Context) error {
	return validate.PreflightCheckWithContext(ctx)
}
