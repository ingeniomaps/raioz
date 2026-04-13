package app

import (
	"context"
	"os"
	"testing"

	"raioz/internal/config"
)

func TestIsHostProcessAlive_NonExistent(t *testing.T) {
	// PID 0 should not be alive
	if isHostProcessAlive(0) {
		t.Error("expected PID 0 to not be alive")
	}
	// PID 1 (init) is typically always present on unix
	// Use current process for deterministic behavior
	if !isHostProcessAlive(os.Getpid()) {
		t.Error("expected current process to be alive")
	}
}

func TestCheckYAML_Empty(t *testing.T) {
	initI18nForTest(t)
	proj := &YAMLProject{
		ProjectName: "test",
		Deps: &config.Deps{
			Project:  config.Project{Name: "test"},
			Services: map[string]config.Service{},
			Infra:    map[string]config.InfraEntry{},
		},
	}
	if err := CheckYAML(proj); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckYAML_WithValidInfra(t *testing.T) {
	initI18nForTest(t)
	proj := &YAMLProject{
		ProjectName: "test",
		Deps: &config.Deps{
			Project:  config.Project{Name: "test"},
			Services: map[string]config.Service{},
			Infra: map[string]config.InfraEntry{
				"redis": {Inline: &config.Infra{Image: "redis:7"}},
			},
		},
	}
	if err := CheckYAML(proj); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckYAML_UnknownDependsOn(t *testing.T) {
	initI18nForTest(t)
	proj := &YAMLProject{
		ProjectName: "test",
		Deps: &config.Deps{
			Project: config.Project{Name: "test"},
			Services: map[string]config.Service{
				"api": {DependsOn: []string{"missing-dep"}},
			},
			Infra: map[string]config.InfraEntry{},
		},
	}
	// Unresolved dep is a real issue: CheckYAML should return a non-nil
	// error so the CLI wrapper can exit non-zero and skip the "valid"
	// banner. The human-readable output is already printed by CheckYAML
	// itself; the error is purely a signal.
	err := CheckYAML(proj)
	if err == nil {
		t.Fatal("expected error for unresolved dependency, got nil")
	}
}

func TestRestartYAML_Empty(t *testing.T) {
	initI18nForTest(t)
	proj := &YAMLProject{
		ProjectName: "test",
		Deps:        &config.Deps{},
	}
	if err := RestartYAML(context.Background(), proj, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLogsYAML_NoServices(t *testing.T) {
	initI18nForTest(t)
	proj := &YAMLProject{
		ProjectName: "test",
		Deps: &config.Deps{
			Services: map[string]config.Service{},
			Infra:    map[string]config.InfraEntry{},
		},
	}
	if err := LogsYAML(context.Background(), proj, nil, false, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
