package upcase

import (
	"context"
	stderrors "errors"
	"testing"

	"raioz/internal/domain/models"
	raiozerr "raioz/internal/errors"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
)

// withStubbedTTY forces stdinIsInteractiveFn to the given value for the
// duration of the test and restores it afterwards.
func withStubbedTTY(t *testing.T, interactive bool) {
	t.Helper()
	prev := stdinIsInteractiveFn
	stdinIsInteractiveFn = func() bool { return interactive }
	t.Cleanup(func() { stdinIsInteractiveFn = prev })
}

// TestResolvePortBindConflictsNonInteractive verifies that a bind conflict
// surfaced in a non-tty session fails fast with a PORT_CONFLICT error
// instead of crashing on an EOF read from a closed stdin (issue 020 fix c).
func TestResolvePortBindConflictsNonInteractive(t *testing.T) {
	initI18nUp(t)
	withStubbedTTY(t, false)

	conflicts := []PortBindConflict{
		{Kind: "dep", Name: "redis", Port: 6379},
	}
	result := &PortAllocResult{}

	err := resolvePortBindConflicts(
		context.Background(), conflicts, result, "", "my-project",
	)
	if err == nil {
		t.Fatal("expected non-interactive conflict to fail, got nil")
	}
	var re *raiozerr.RaiozError
	if !stderrors.As(err, &re) || re.Code != raiozerr.ErrCodePortConflict {
		t.Fatalf("expected PORT_CONFLICT RaiozError, got %v", err)
	}
}

func TestHandleDependencyAssistNonInteractive(t *testing.T) {
	initI18nUp(t)
	withStubbedTTY(t, false)

	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(
				ws *workspace.Workspace, n string, svc models.Service,
			) string {
				return "/" + n
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			DetectMissingDependenciesFunc: func(
				*models.Deps, func(string, models.Service) string,
			) ([]models.MissingDependency, error) {
				return []models.MissingDependency{
					{ServiceName: "cache", RequiredBy: "api"},
				}, nil
			},
		},
	})

	ok, _, err := uc.handleDependencyAssist(context.Background(),
		&models.Deps{}, &workspace.Workspace{Root: "/tmp"}, false,
	)
	if err == nil {
		t.Fatal("expected non-interactive missing-deps prompt to fail, got nil")
	}
	if ok {
		t.Error("should not continue when bailing out non-interactively")
	}
}

func TestHandleDependencyConflictsNonInteractive(t *testing.T) {
	initI18nUp(t)
	withStubbedTTY(t, false)

	uc := NewUseCase(&Dependencies{
		Workspace: &mocks.MockWorkspaceManager{
			GetServicePathFunc: func(
				ws *workspace.Workspace, n string, svc models.Service,
			) string {
				return "/" + n
			},
		},
		ConfigLoader: &mocks.MockConfigLoader{
			DetectDependencyConflictsFunc: func(
				*models.Deps, func(string, models.Service) string,
			) ([]models.DependencyConflict, error) {
				return []models.DependencyConflict{
					{ServiceName: "svc", Differences: []string{"branch differs"}},
				}, nil
			},
		},
	})

	ok, _, err := uc.handleDependencyConflicts(context.Background(),
		&models.Deps{}, &workspace.Workspace{Root: "/tmp"}, false,
	)
	if err == nil {
		t.Fatal("expected non-interactive conflict prompt to fail, got nil")
	}
	if ok {
		t.Error("should not continue when bailing out non-interactively")
	}
}
