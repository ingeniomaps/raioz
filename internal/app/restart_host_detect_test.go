package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/discovery"
	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/mocks"
)

// writeServiceDir creates a service directory holding the given files so
// detection has something real to scan.
func writeServiceDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// captureHostStart replaces the host-service launcher for one test and
// hands back what restart asked it to start.
func captureHostStart(t *testing.T) *interfaces.ServiceContext {
	t.Helper()
	var got interfaces.ServiceContext
	prev := startHostServiceFn
	startHostServiceFn = func(
		_ context.Context, _ interfaces.DockerRunner, svcCtx interfaces.ServiceContext,
	) (int, error) {
		got = svcCtx
		return 4242, nil
	}
	t.Cleanup(func() { startHostServiceFn = prev })
	return &got
}

// launchCommand is the command the host runner would run for a context:
// dev first, start as the fallback — the order orchestrate.HostRunner uses.
func launchCommand(svcCtx *interfaces.ServiceContext) string {
	if svcCtx.Detection.DevCommand != "" {
		return svcCtx.Detection.DevCommand
	}
	return svcCtx.Detection.StartCommand
}

const npmPackageJSON = `{"name":"bff","scripts":{"start":"node dist/main","dev":"nest start --watch"}}`

func TestIsYAMLHostService(t *testing.T) {
	npmDir := writeServiceDir(t, map[string]string{"package.json": npmPackageJSON})
	dockerDir := writeServiceDir(t, map[string]string{"Dockerfile": "FROM alpine\n"})
	emptyDir := writeServiceDir(t, nil)

	local := func(path string) models.SourceConfig {
		return models.SourceConfig{Kind: "local", Path: path}
	}
	tests := []struct {
		name string
		svc  models.Service
		want bool
	}{
		{"declared command", models.Service{Source: models.SourceConfig{Kind: "local", Command: "make dev"}}, true},
		{"stop-only commands block", models.Service{Source: local(dockerDir),
			Commands: &models.ServiceCommands{Down: "make stop"}}, true},
		// The reported bug: npm detected from the directory, no command.
		{"auto-detected npm", models.Service{Source: local(npmDir)}, true},
		{"declared runtime npm", models.Service{Source: models.SourceConfig{
			Kind: "local", Path: npmDir, Runtime: string(models.RuntimeNPM)}}, true},
		{"dockerfile stays on the container path", models.Service{Source: local(dockerDir)}, false},
		{"docker block stays on the container path", models.Service{Source: local(npmDir),
			Docker: &models.DockerConfig{}}, false},
		{"unknown runtime keeps the old fallback", models.Service{Source: local(emptyDir)}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			proj := &YAMLProject{Deps: &models.Deps{Services: map[string]models.Service{"svc": tc.svc}}}
			if got := isYAMLHostService(proj, "svc"); got != tc.want {
				t.Errorf("isYAMLHostService = %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("undeclared name", func(t *testing.T) {
		proj := &YAMLProject{Deps: &models.Deps{Services: map[string]models.Service{}}}
		if isYAMLHostService(proj, "ghost") {
			t.Error("undeclared name classified as host service")
		}
	})
}

func TestRestartHostService_RelaunchCommand(t *testing.T) {
	npmDir := writeServiceDir(t, map[string]string{"package.json": npmPackageJSON})

	tests := []struct {
		name string
		svc  models.Service
		want string
	}{
		// up prefers the dev command for host runtimes; restart must
		// relaunch the same thing.
		{"auto-detected npm uses the dev command",
			models.Service{Source: models.SourceConfig{Kind: "local", Path: npmDir}}, "npm run dev"},
		{"declared command is left untouched",
			models.Service{Source: models.SourceConfig{Kind: "local", Path: npmDir, Command: "pnpm start:dev"}},
			"pnpm start:dev"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			started := captureHostStart(t)
			uc := NewRestartUseCase(&Dependencies{HostRunner: &mocks.MockHostRunner{}})
			proj := &YAMLProject{
				ProjectName: "bff",
				ConfigPath:  filepath.Join(t.TempDir(), "raioz.yaml"),
				Deps:        &models.Deps{Services: map[string]models.Service{"bff": tc.svc}},
			}
			if err := uc.restartHostService(context.Background(), proj, "bff"); err != nil {
				t.Fatalf("restartHostService: %v", err)
			}
			if launched := launchCommand(started); launched != tc.want {
				t.Errorf("relaunched with %q, want %q", launched, tc.want)
			}
		})
	}
}

// A service restart cannot relaunch must be left running: failing after
// the stop step turns a refused restart into an outage.
func TestRestartHostService_NoCommandStopsNothing(t *testing.T) {
	dockerDir := writeServiceDir(t, map[string]string{"Dockerfile": "FROM alpine\n"})

	var stopped bool
	started := captureHostStart(t)
	hostRunner := &mocks.MockHostRunner{
		StopServiceWithCommandAndPathFunc: func(_ context.Context, _ int, _, _ string) error {
			stopped = true
			return nil
		},
	}
	uc := NewRestartUseCase(&Dependencies{HostRunner: hostRunner})
	proj := &YAMLProject{
		ProjectName: "bff",
		ConfigPath:  filepath.Join(t.TempDir(), "raioz.yaml"),
		Deps: &models.Deps{Services: map[string]models.Service{"bff": {
			Source:   models.SourceConfig{Kind: "local", Path: dockerDir},
			Commands: &models.ServiceCommands{Down: "make stop"},
		}}},
	}

	if err := uc.restartHostService(context.Background(), proj, "bff"); err == nil {
		t.Fatal("expected an error for a service with no relaunch command")
	}
	if launched := started.Name != ""; stopped || launched {
		t.Errorf("stopped=%v launched=%v, want the service left untouched", stopped, launched)
	}
}

// up injects PORT and the discovery vars; a restart that relaunches without
// them brings the service back on a different env than it was started with.
func TestRestartHostService_RelaunchesWithComputedEnv(t *testing.T) {
	npmDir := writeServiceDir(t, map[string]string{"package.json": npmPackageJSON})

	started := captureHostStart(t)
	uc := NewRestartUseCase(&Dependencies{
		HostRunner:       &mocks.MockHostRunner{},
		DiscoveryManager: discovery.NewManager(),
	})
	proj := &YAMLProject{
		ProjectName: "bff",
		ConfigPath:  filepath.Join(t.TempDir(), "raioz.yaml"),
		Deps: &models.Deps{
			Project: models.Project{Name: "bff"},
			Services: map[string]models.Service{
				"bff": {Source: models.SourceConfig{Kind: "local", Path: npmDir}, Port: 3002},
				"api": {Source: models.SourceConfig{Kind: "local", Path: npmDir}, Port: 3003},
			},
		},
	}

	if err := uc.restartHostService(context.Background(), proj, "bff"); err != nil {
		t.Fatalf("restartHostService: %v", err)
	}
	got := started.EnvVars
	if got["PORT"] != "3002" {
		t.Errorf("PORT = %q, want 3002", got["PORT"])
	}
	if got["API_URL"] == "" {
		t.Errorf("API_URL missing from the relaunch env: %v", got)
	}
}

// `stop:` runs where `command:` ran — the service's path, not the project
// root, or `stop: make stop` finds no Makefile.
func TestStopCommandRunsInServicePath(t *testing.T) {
	projectDir := t.TempDir()
	svcDir := filepath.Join(projectDir, "stack")
	if err := os.MkdirAll(svcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Run("down", func(t *testing.T) {
		deps := &models.Deps{Services: map[string]models.Service{"stack": {
			Source:   models.SourceConfig{Kind: "local", Path: svcDir},
			Commands: &models.ServiceCommands{Down: "touch stopped.marker"},
		}}}
		if failed := runCustomStopCommands(context.Background(), deps, projectDir); len(failed) != 0 {
			t.Fatalf("stop failed for %v", failed)
		}
		if _, err := os.Stat(filepath.Join(svcDir, "stopped.marker")); err != nil {
			t.Errorf("stop did not run in the service path: %v", err)
		}
	})

	t.Run("restart", func(t *testing.T) {
		var gotDir string
		hostRunner := &mocks.MockHostRunner{
			StopServiceWithCommandAndPathFunc: func(_ context.Context, _ int, _, dir string) error {
				gotDir = dir
				return nil
			},
		}
		captureHostStart(t)
		uc := NewRestartUseCase(&Dependencies{HostRunner: hostRunner})
		proj := &YAMLProject{
			ProjectName: "rzc",
			ConfigPath:  filepath.Join(projectDir, "raioz.yaml"),
			Deps: &models.Deps{Services: map[string]models.Service{"stack": {
				Source:   models.SourceConfig{Kind: "local", Path: svcDir, Command: "make start"},
				Commands: &models.ServiceCommands{Down: "make stop"},
			}}},
		}
		if err := uc.restartHostService(context.Background(), proj, "stack"); err != nil {
			t.Fatalf("restartHostService: %v", err)
		}
		if gotDir != svcDir {
			t.Errorf("stop ran in %q, want %q", gotDir, svcDir)
		}
	})
}
