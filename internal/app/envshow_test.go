package app

import (
	"context"
	"fmt"
	"testing"

	"raioz/internal/config"
	"raioz/internal/mocks"
)

func TestNewEnvShowUseCase(t *testing.T) {
	uc := NewEnvShowUseCase(newFullMockDeps())
	if uc == nil {
		t.Fatal("expected non-nil EnvShowUseCase")
	}
}

func TestEnvShowUseCase_Execute_ConfigLoadError(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
			return nil, nil, fmt.Errorf("fail")
		},
	}
	uc := NewEnvShowUseCase(deps)
	_, err := uc.Execute(context.Background(), EnvShowOptions{
		ConfigPath:  "bad.json",
		ServiceName: "api",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestEnvShowUseCase_Execute_ServiceNotFound(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
			return &config.Deps{
				Project:  config.Project{Name: "test"},
				Services: map[string]config.Service{},
				Infra:    map[string]config.InfraEntry{},
			}, nil, nil
		},
	}
	uc := NewEnvShowUseCase(deps)
	_, err := uc.Execute(context.Background(), EnvShowOptions{
		ConfigPath:  "raioz.json",
		ServiceName: "missing",
	})
	if err == nil {
		t.Fatal("expected error for missing service, got nil")
	}
}

func TestEnvShowUseCase_Execute_ServiceIsInfra(t *testing.T) {
	initI18nForTest(t)
	deps := newFullMockDeps()
	deps.ConfigLoader = &mocks.MockConfigLoader{
		LoadDepsFunc: func(configPath string) (*config.Deps, []string, error) {
			return &config.Deps{
				Project: config.Project{Name: "test"},
				Infra: map[string]config.InfraEntry{
					"redis": {},
				},
			}, nil, nil
		},
	}
	uc := NewEnvShowUseCase(deps)
	_, err := uc.Execute(context.Background(), EnvShowOptions{
		ConfigPath:  "raioz.json",
		ServiceName: "redis",
	})
	if err == nil {
		t.Fatal("expected error for dependency (not a service), got nil")
	}
}

func TestParseFirstPort(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"8080", 8080},
		{"8080:80", 8080},
		{"bad", 0},
	}
	for _, tt := range tests {
		if got := parseFirstPort(tt.input); got != tt.want {
			t.Errorf("parseFirstPort(%q): expected %d, got %d", tt.input, tt.want, got)
		}
	}
}

func TestJoinServiceNames(t *testing.T) {
	deps := &config.Deps{
		Services: map[string]config.Service{
			"api": {},
			"web": {},
		},
	}
	result := joinServiceNames(deps)
	if result != "api, web" {
		t.Errorf("expected 'api, web', got %q", result)
	}
}

func TestJoinServiceNames_Empty(t *testing.T) {
	deps := &config.Deps{Services: map[string]config.Service{}}
	if got := joinServiceNames(deps); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}
