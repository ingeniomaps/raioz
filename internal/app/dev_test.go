package app

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/config"
	"raioz/internal/mocks"
	"raioz/internal/state"
)

func TestNewDevUseCase(t *testing.T) {
	deps := newFullMockDeps()
	uc := NewDevUseCase(deps)
	if uc == nil {
		t.Fatal("expected non-nil DevUseCase")
	}
}

func TestInfraNames(t *testing.T) {
	deps := &config.Deps{
		Infra: map[string]config.InfraEntry{
			"postgres": {},
			"redis":    {},
		},
	}
	names := infraNames(deps)
	if !strings.Contains(names, "postgres") {
		t.Errorf("expected postgres in %q", names)
	}
	if !strings.Contains(names, "redis") {
		t.Errorf("expected redis in %q", names)
	}
}

func TestInfraNamesEmpty(t *testing.T) {
	deps := &config.Deps{Infra: map[string]config.InfraEntry{}}
	names := infraNames(deps)
	if names != "" {
		t.Errorf("expected empty string, got %q", names)
	}
}

func TestInfraPorts(t *testing.T) {
	entry := config.InfraEntry{
		Inline: &config.Infra{
			Ports: []string{"5432", "5433"},
		},
	}
	ports := infraPorts(entry)
	if len(ports) != 2 {
		t.Errorf("expected 2 ports, got %d", len(ports))
	}
}

func TestInfraPorts_Nil(t *testing.T) {
	entry := config.InfraEntry{}
	ports := infraPorts(entry)
	if ports != nil {
		t.Errorf("expected nil, got %v", ports)
	}
}

func TestDevUseCase_Execute_ConfigLoadError(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
			return nil, nil, fmt.Errorf("load error")
		},
	}
	uc := NewDevUseCase(deps)
	err := uc.Execute(context.Background(), DevOptions{
		ConfigPath: filepath.Join(tmpDir, "raioz.yaml"),
		Name:       "redis",
	})
	if err == nil {
		t.Fatal("expected error for config load failure, got nil")
	}
}

func TestDevUseCase_Execute_MissingName(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
			return &config.Deps{Project: config.Project{Name: "test"}}, nil, nil
		},
	}
	uc := NewDevUseCase(deps)
	err := uc.Execute(context.Background(), DevOptions{
		ConfigPath: filepath.Join(tmpDir, "raioz.yaml"),
	})
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
}

func TestDevUseCase_Execute_ListEmpty(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	deps := newFullMockDeps()
	uc := NewDevUseCase(deps)
	err := uc.Execute(context.Background(), DevOptions{
		ConfigPath: filepath.Join(tmpDir, "raioz.yaml"),
		List:       true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDevUseCase_ListOverrides_Empty(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	uc := NewDevUseCase(deps)
	err := uc.listOverrides(&state.LocalState{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDevUseCase_ListOverrides_WithOverrides(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	uc := NewDevUseCase(deps)
	localState := &state.LocalState{
		DevOverrides: map[string]state.DevOverride{
			"redis": {
				OriginalImage: "redis:7",
				LocalPath:     "/tmp/redis",
			},
		},
	}
	var buf bytes.Buffer
	_ = buf // output.PrintSectionHeader writes to stdout; just ensure no error
	err := uc.listOverrides(localState)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
