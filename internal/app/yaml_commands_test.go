package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
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
		Deps: &models.Deps{
			Project:  models.Project{Name: "test"},
			Services: map[string]models.Service{},
			Infra:    map[string]models.InfraEntry{},
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
		Deps: &models.Deps{
			Project:  models.Project{Name: "test"},
			Services: map[string]models.Service{},
			Infra: map[string]models.InfraEntry{
				"redis": {Inline: &models.Infra{Image: "redis:7"}},
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
		Deps: &models.Deps{
			Project: models.Project{Name: "test"},
			Services: map[string]models.Service{
				"api": {DependsOn: []string{"missing-dep"}},
			},
			Infra: map[string]models.InfraEntry{},
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

// Asked to restart nothing, restart says so and fails: a script that got
// its argument list wrong must not read that as a successful restart.
func TestRestartYAML_Empty(t *testing.T) {
	initI18nForTest(t)
	proj := &YAMLProject{
		ProjectName: "test",
		Deps:        &models.Deps{},
	}
	uc := &RestartUseCase{}
	if err := uc.RestartYAML(context.Background(), proj, RestartOptions{}); err == nil {
		t.Fatal("expected an error when no service and no --all are given")
	}
}

// exec hands back the exit code of the command it ran.
func TestExecError_KeepsExitCode(t *testing.T) {
	failing := exec.Command("sh", "-c", "exit 3").Run()
	var exit *ExitCodeError
	if err := execError("docker exec", failing); !errors.As(err, &exit) || exit.Code != 3 {
		t.Errorf("got %v, want an ExitCodeError with code 3", err)
	}
	if err := execError("docker exec", errors.New("not started")); errors.As(err, &exit) {
		t.Errorf("a command that never ran has no exit code to keep: %v", err)
	}
}

func TestLogsYAML_NoServices(t *testing.T) {
	initI18nForTest(t)
	proj := &YAMLProject{
		ProjectName: "test",
		Deps: &models.Deps{
			Services: map[string]models.Service{},
			Infra:    map[string]models.InfraEntry{},
		},
	}
	if err := LogsYAML(context.Background(), proj, nil, false, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --force-recreate goes through the recreate path for container targets,
// and a failure there fails the command.
func TestRestartYAML_ForceRecreate(t *testing.T) {
	initI18nForTest(t)
	proj := &YAMLProject{
		ProjectName: "test",
		ConfigPath:  "raioz.yaml",
		Deps: &models.Deps{
			Infra: map[string]models.InfraEntry{"kv": {Inline: &models.Infra{Image: "redis", Tag: "7"}}},
		},
	}
	prev := recreateTargetFn
	t.Cleanup(func() { recreateTargetFn = prev })

	var recreated int
	recreateTargetFn = func(
		context.Context, interfaces.DockerRunner, func() (interfaces.ServiceContext, bool),
	) error {
		recreated++
		return nil
	}
	uc := &RestartUseCase{deps: &Dependencies{}}
	opts := RestartOptions{Services: []string{"kv"}, ForceRecreate: true}
	if err := uc.RestartYAML(context.Background(), proj, opts); err != nil {
		t.Fatalf("recreate succeeded, restart must too: %v", err)
	}
	if recreated != 1 {
		t.Fatalf("recreate calls = %d, want 1", recreated)
	}

	recreateTargetFn = func(
		context.Context, interfaces.DockerRunner, func() (interfaces.ServiceContext, bool),
	) error {
		return errors.New("boom")
	}
	if err := uc.RestartYAML(context.Background(), proj, opts); err == nil {
		t.Error("a failed recreate must fail the restart")
	}
}
