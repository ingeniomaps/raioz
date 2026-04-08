package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/config"
	"raioz/internal/i18n"
	"raioz/internal/mocks"
)

func initI18nOverride(t *testing.T) {
	t.Helper()
	os.Setenv("RAIOZ_LANG", "en")
	// Use temp HOME so override operations don't affect real state
	tmpHome := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpHome)
	t.Cleanup(func() {
		os.Unsetenv("RAIOZ_LANG")
		os.Unsetenv("RAIOZ_HOME")
	})
	i18n.Init("en")
}

func TestOverrideApplyEmptyPath(t *testing.T) {
	initI18nOverride(t)
	uc := NewOverrideUseCase(&Dependencies{})
	err := uc.Apply("svc", "", "config.json")
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestOverrideApplyInvalidPath(t *testing.T) {
	initI18nOverride(t)
	uc := NewOverrideUseCase(&Dependencies{})
	var buf bytes.Buffer
	uc.Out = &buf
	err := uc.Apply("svc", "/nonexistent/path", "config.json")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestOverrideApplyValidPath(t *testing.T) {
	initI18nOverride(t)

	extDir := t.TempDir()
	cfgDeps := &config.Deps{
		Project:  config.Project{Name: "test"},
		Services: map[string]config.Service{"api": {}},
	}

	uc := NewOverrideUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return cfgDeps, nil, nil
			},
		},
	})
	var buf bytes.Buffer
	uc.Out = &buf

	err := uc.Apply("api", extDir, "config.json")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(buf.String(), "api") {
		t.Errorf("output should mention service name\ngot: %s", buf.String())
	}
}

func TestOverrideApplyServiceNotInConfig(t *testing.T) {
	initI18nOverride(t)

	extDir := t.TempDir()
	cfgDeps := &config.Deps{
		Project:  config.Project{Name: "test"},
		Services: map[string]config.Service{},
	}

	uc := NewOverrideUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return cfgDeps, nil, nil
			},
		},
	})
	var buf bytes.Buffer
	uc.Out = &buf

	err := uc.Apply("ghost", extDir, "config.json")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// Should warn about service not found
	if !strings.Contains(buf.String(), "ghost") {
		t.Errorf("output should mention missing service\ngot: %s", buf.String())
	}
}

func TestOverrideListEmpty(t *testing.T) {
	initI18nOverride(t)

	uc := NewOverrideUseCase(&Dependencies{})
	var buf bytes.Buffer
	uc.Out = &buf

	err := uc.List()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(strings.ToLower(buf.String()), "no override") {
		t.Errorf("should show empty message\ngot: %s", buf.String())
	}
}

func TestOverrideRemoveNonexistent(t *testing.T) {
	initI18nOverride(t)

	uc := NewOverrideUseCase(&Dependencies{})
	var buf bytes.Buffer
	uc.Out = &buf

	err := uc.Remove("ghost")
	if err == nil {
		t.Error("expected error for removing nonexistent override")
	}
}

func TestOverrideFullCycle(t *testing.T) {
	initI18nOverride(t)

	extDir := t.TempDir()
	uc := NewOverrideUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return nil, nil, nil
			},
		},
	})
	var buf bytes.Buffer
	uc.Out = &buf

	// Apply
	err := uc.Apply("svc", extDir, "")
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}

	// List
	buf.Reset()
	err = uc.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if !strings.Contains(buf.String(), "svc") {
		t.Errorf("list should contain 'svc'\ngot: %s", buf.String())
	}
	if !strings.Contains(buf.String(), filepath.Base(extDir)) {
		t.Errorf("list should contain path\ngot: %s", buf.String())
	}

	// Remove
	buf.Reset()
	err = uc.Remove("svc")
	if err != nil {
		t.Fatalf("Remove error: %v", err)
	}
	if !strings.Contains(buf.String(), "svc") {
		t.Errorf("remove output should mention service\ngot: %s", buf.String())
	}
}
