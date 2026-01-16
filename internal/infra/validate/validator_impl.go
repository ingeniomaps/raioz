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
