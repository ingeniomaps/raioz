package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/discovery"
	"raioz/internal/domain/models"
	"raioz/internal/host"
	"raioz/internal/mocks"
	"raioz/internal/workspace"
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
			var launched string
			hostRunner := &mocks.MockHostRunner{
				StartServiceFunc: func(_ context.Context, _ *workspace.Workspace, _ *models.Deps,
					_ string, svc models.Service, _ string,
				) (*host.ProcessInfo, error) {
					launched = svc.Source.Command
					return &host.ProcessInfo{PID: 4242}, nil
				},
			}
			uc := NewRestartUseCase(&Dependencies{HostRunner: hostRunner})
			proj := &YAMLProject{
				ProjectName: "bff",
				ConfigPath:  filepath.Join(t.TempDir(), "raioz.yaml"),
				Deps:        &models.Deps{Services: map[string]models.Service{"bff": tc.svc}},
			}
			if err := uc.restartHostService(context.Background(), proj, "bff"); err != nil {
				t.Fatalf("restartHostService: %v", err)
			}
			if launched != tc.want {
				t.Errorf("relaunched with %q, want %q", launched, tc.want)
			}
		})
	}
}

// A service restart cannot relaunch must be left running: failing after
// the stop step turns a refused restart into an outage.
func TestRestartHostService_NoCommandStopsNothing(t *testing.T) {
	dockerDir := writeServiceDir(t, map[string]string{"Dockerfile": "FROM alpine\n"})

	var stopped, launched bool
	hostRunner := &mocks.MockHostRunner{
		StopServiceWithCommandFunc: func(_ context.Context, _ int, _ string) error {
			stopped = true
			return nil
		},
		StartServiceFunc: func(_ context.Context, _ *workspace.Workspace, _ *models.Deps,
			_ string, _ models.Service, _ string,
		) (*host.ProcessInfo, error) {
			launched = true
			return &host.ProcessInfo{PID: 4242}, nil
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
	if stopped || launched {
		t.Errorf("stopped=%v launched=%v, want the service left untouched", stopped, launched)
	}
}

// up injects PORT and the discovery vars; a restart that relaunches without
// them brings the service back on a different env than it was started with.
func TestRestartHostService_RelaunchesWithComputedEnv(t *testing.T) {
	npmDir := writeServiceDir(t, map[string]string{"package.json": npmPackageJSON})

	var got map[string]string
	hostRunner := &mocks.MockHostRunner{
		StartServiceFunc: func(ctx context.Context, _ *workspace.Workspace, _ *models.Deps,
			_ string, _ models.Service, _ string,
		) (*host.ProcessInfo, error) {
			got = host.ExtraEnv(ctx)
			return &host.ProcessInfo{PID: 4242}, nil
		},
	}
	uc := NewRestartUseCase(&Dependencies{
		HostRunner:       hostRunner,
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
	if got["PORT"] != "3002" {
		t.Errorf("PORT = %q, want 3002", got["PORT"])
	}
	if got["API_URL"] == "" {
		t.Errorf("API_URL missing from the relaunch env: %v", got)
	}
}
