package app

import (
	"context"
	"os"
	"strings"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/i18n"
	"raioz/internal/mocks"
)

func initI18nStatus(t *testing.T) {
	t.Helper()
	os.Setenv("RAIOZ_LANG", "en")
	t.Cleanup(func() { os.Unsetenv("RAIOZ_LANG") })
	i18n.Init("en")
}

func TestNewStatusUseCase(t *testing.T) {
	uc := NewStatusUseCase(&Dependencies{})
	if uc == nil {
		t.Fatal("should return non-nil")
	}
}

// Without a resolvable raioz.yaml there is nothing to report: the loader
// hard-errors on any other shape (ADR-038), so status says so instead of
// falling through to a path that could never load a project.
func TestStatusWithoutYAMLProjectErrors(t *testing.T) {
	initI18nStatus(t)

	uc := NewStatusUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*models.Deps, []string, error) {
				return nil, nil, nil
			},
		},
	})

	err := uc.Execute(context.Background(), StatusOptions{ConfigPath: "raioz.yaml"})
	if err == nil {
		t.Fatal("expected an error when no YAML project resolves")
	}
	if !strings.Contains(err.Error(), i18n.T("error.no_project")) {
		t.Errorf("error = %v, want it to report the missing project", err)
	}
}
