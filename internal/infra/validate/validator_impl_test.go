package validate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
)

func TestNewValidator(t *testing.T) {
	v := NewValidator()
	if v == nil {
		t.Fatal("NewValidator returned nil")
	}
}

func TestValidatorImpl_ValidateSchema(t *testing.T) {
	v := NewValidator()
	// Empty deps should not pass schema but shouldn't crash
	deps := &config.Deps{}
	_ = v.ValidateSchema(deps)
}

func TestValidatorImpl_ValidateProject(t *testing.T) {
	v := NewValidator()
	deps := &config.Deps{
		Project: config.Project{Name: "valid-project"},
	}
	_ = v.ValidateProject(deps)
}

func TestValidatorImpl_ValidateProject_Missing(t *testing.T) {
	v := NewValidator()
	deps := &config.Deps{}
	if err := v.ValidateProject(deps); err == nil {
		t.Error("expected error for missing project name")
	}
}

func TestValidatorImpl_ValidateServices(t *testing.T) {
	v := NewValidator()
	deps := &config.Deps{
		Services: map[string]config.Service{},
	}
	_ = v.ValidateServices(deps)
}

func TestValidatorImpl_ValidateInfra(t *testing.T) {
	v := NewValidator()
	deps := &config.Deps{
		Infra: map[string]config.InfraEntry{},
	}
	_ = v.ValidateInfra(deps)
}

func TestValidatorImpl_ValidateDependencies(t *testing.T) {
	v := NewValidator()
	deps := &config.Deps{
		Services: map[string]config.Service{},
	}
	_ = v.ValidateDependencies(deps)
}

func TestValidatorImpl_All(t *testing.T) {
	v := NewValidator()
	deps := &config.Deps{
		Project: config.Project{Name: "p"},
	}
	_ = v.All(deps)
}

func TestValidatorImpl_CheckWorkspacePermissions(t *testing.T) {
	v := NewValidator()
	dir := t.TempDir()
	if err := v.CheckWorkspacePermissions(dir); err != nil {
		t.Errorf("tempdir should have valid permissions: %v", err)
	}
}

func TestValidatorImpl_CheckWorkspacePermissions_NonExistent(t *testing.T) {
	v := NewValidator()
	err := v.CheckWorkspacePermissions(filepath.Join(t.TempDir(), "nope"))
	// Some implementations create it, some error. Just verify no panic.
	_ = err
}

func TestValidatorImpl_CheckDockerInstalled(t *testing.T) {
	v := NewValidator()
	// May fail on CI without Docker. Just verify no panic.
	_ = v.CheckDockerInstalled()
}

func TestValidatorImpl_CheckDockerRunning(t *testing.T) {
	v := NewValidator()
	// May fail without Docker daemon. Just verify no panic.
	_ = v.CheckDockerRunning()
}

func TestValidatorImpl_PreflightCheckWithContext(t *testing.T) {
	v := NewValidator()
	ctx := context.Background()
	_ = v.PreflightCheckWithContext(ctx)
}

func TestValidatorImpl_ValidateBeforeUp_Nil(t *testing.T) {
	v := NewValidator()
	deps := &config.Deps{
		Project: config.Project{Name: "p"},
	}

	// With nil context and workspace, should fall through to validate.All
	_ = v.ValidateBeforeUp(nil, deps, nil)
}

func TestValidatorImpl_ValidateBeforeUp_TypedNil(t *testing.T) {
	v := NewValidator()
	deps := &config.Deps{
		Project: config.Project{Name: "p"},
	}
	ctx := context.Background()
	// Pass a non-workspace to hit the fallback path
	_ = v.ValidateBeforeUp(ctx, deps, "not-a-workspace")
}

func TestValidatorImpl_ValidateBeforeDown_Nil(t *testing.T) {
	v := NewValidator()
	_ = v.ValidateBeforeDown(nil, nil)
}

func TestValidatorImpl_ValidateBeforeDown_WithContext(t *testing.T) {
	v := NewValidator()
	ctx := context.Background()
	_ = v.ValidateBeforeDown(ctx, "not-a-workspace")
}

func TestValidatorImpl_CheckWorkspacePermissions_Unwritable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root — unwritable check won't work")
	}
	v := NewValidator()
	// Root usually isn't writable
	_ = v.CheckWorkspacePermissions("/root/raioz-test-should-fail")
}
