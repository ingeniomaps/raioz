package orchestrate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/detect"
	"raioz/internal/domain/interfaces"
	"raioz/internal/mocks"

	"gopkg.in/yaml.v3"
)

func makeComposeSvc(t *testing.T) interfaces.ServiceContext {
	t.Helper()
	dir := t.TempDir()
	composePath := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte("services: {}\n"), 0644); err != nil {
		t.Fatalf("write compose: %v", err)
	}
	return interfaces.ServiceContext{
		Name:          "api",
		Path:          dir,
		ProjectName:   "proj",
		ContainerName: "raioz-proj-api",
		NetworkName:   "proj-net",
		Detection: detect.DetectResult{
			Runtime:     detect.RuntimeCompose,
			ComposeFile: composePath,
		},
	}
}

func TestComposeRunner_OverlayPath(t *testing.T) {
	r := &ComposeRunner{}
	svc := makeComposeSvc(t)

	path := r.overlayPath(svc)
	expectedDir := filepath.Dir(svc.Detection.ComposeFile)
	if filepath.Dir(path) != expectedDir {
		t.Errorf("expected dir %s, got %s", expectedDir, filepath.Dir(path))
	}
	if filepath.Base(path) != ".raioz-overlay.yml" {
		t.Errorf("expected .raioz-overlay.yml, got %s", filepath.Base(path))
	}
}

func TestComposeRunner_WriteOverlay(t *testing.T) {
	r := &ComposeRunner{}
	svc := makeComposeSvc(t)

	overlay := map[string]any{
		"networks": map[string]any{
			"proj-net": map[string]any{"external": true},
		},
	}

	path, err := r.writeOverlay(svc, overlay)
	if err != nil {
		t.Fatalf("writeOverlay: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("overlay file not created: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read overlay: %v", err)
	}

	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Errorf("overlay is not valid yaml: %v", err)
	}

	if _, ok := parsed["networks"]; !ok {
		t.Error("overlay missing networks key")
	}
}

func TestComposeRunner_WriteOverlay_CreatesDir(t *testing.T) {
	r := &ComposeRunner{}
	dir := t.TempDir()
	subdir := filepath.Join(dir, "nested", "deep")
	composePath := filepath.Join(subdir, "docker-compose.yml")

	// Create the subdir first since overlayPath uses Dir of compose file
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	svc := interfaces.ServiceContext{
		Name:        "api",
		NetworkName: "proj-net",
		Detection:   detect.DetectResult{ComposeFile: composePath},
	}

	overlay := map[string]any{"networks": map[string]any{}}
	path, err := r.writeOverlay(svc, overlay)
	if err != nil {
		t.Errorf("writeOverlay: %v", err)
	}
	if !strings.HasSuffix(path, ".raioz-overlay.yml") {
		t.Errorf("expected overlay suffix, got %s", path)
	}
}

func TestComposeRunner_CreateNetworkOverlay_WithServices(t *testing.T) {
	svc := makeComposeSvc(t)
	mock := &mocks.MockDockerRunner{
		GetAvailableServicesWithContextFunc: func(
			_ context.Context, _ string,
		) ([]string, error) {
			return []string{"web", "db"}, nil
		},
	}
	r := &ComposeRunner{docker: mock}

	path, err := r.createNetworkOverlay(svc)
	if err != nil {
		t.Fatalf("createNetworkOverlay: %v", err)
	}

	data, _ := os.ReadFile(path)
	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse overlay: %v", err)
	}

	services, ok := parsed["services"].(map[string]any)
	if !ok {
		t.Fatal("expected services map in overlay")
	}
	if _, ok := services["web"]; !ok {
		t.Error("expected 'web' in overlay services")
	}
	if _, ok := services["db"]; !ok {
		t.Error("expected 'db' in overlay services")
	}

	networks, ok := parsed["networks"].(map[string]any)
	if !ok {
		t.Fatal("expected networks map in overlay")
	}
	if _, ok := networks[svc.NetworkName]; !ok {
		t.Errorf("expected network %s in overlay", svc.NetworkName)
	}
}

func TestComposeRunner_CreateNetworkOverlay_DockerError(t *testing.T) {
	svc := makeComposeSvc(t)
	mock := &mocks.MockDockerRunner{
		GetAvailableServicesWithContextFunc: func(
			_ context.Context, _ string,
		) ([]string, error) {
			return nil, os.ErrNotExist
		},
	}
	r := &ComposeRunner{docker: mock}

	// Should still succeed with generic overlay
	path, err := r.createNetworkOverlay(svc)
	if err != nil {
		t.Fatalf("createNetworkOverlay: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("overlay file not created: %v", err)
	}
}

func TestComposeRunner_Start(t *testing.T) {
	svc := makeComposeSvc(t)
	called := false
	mock := &mocks.MockDockerRunner{
		GetAvailableServicesWithContextFunc: func(
			_ context.Context, _ string,
		) ([]string, error) {
			return []string{"web"}, nil
		},
		UpWithContextFunc: func(_ context.Context, composePath string) error {
			called = true
			if !strings.Contains(composePath, ":") {
				t.Errorf("expected original:overlay, got %s", composePath)
			}
			return nil
		},
	}
	r := &ComposeRunner{docker: mock}

	if err := r.Start(context.Background(), svc); err != nil {
		t.Errorf("Start: %v", err)
	}
	if !called {
		t.Error("UpWithContext was not called")
	}
}

func TestComposeRunner_Stop_NoOverlay(t *testing.T) {
	svc := makeComposeSvc(t)
	called := false
	mock := &mocks.MockDockerRunner{
		DownWithContextFunc: func(_ context.Context, composePath string) error {
			called = true
			if strings.Contains(composePath, ":") {
				t.Errorf("expected single path, got %s", composePath)
			}
			return nil
		},
	}
	r := &ComposeRunner{docker: mock}

	if err := r.Stop(context.Background(), svc); err != nil {
		t.Errorf("Stop: %v", err)
	}
	if !called {
		t.Error("DownWithContext was not called")
	}
}

func TestComposeRunner_Stop_WithOverlay(t *testing.T) {
	svc := makeComposeSvc(t)
	// Create overlay file
	overlayPath := filepath.Join(filepath.Dir(svc.Detection.ComposeFile), ".raioz-overlay.yml")
	if err := os.WriteFile(overlayPath, []byte("networks: {}\n"), 0644); err != nil {
		t.Fatalf("write overlay: %v", err)
	}

	var seen string
	mock := &mocks.MockDockerRunner{
		DownWithContextFunc: func(_ context.Context, composePath string) error {
			seen = composePath
			return nil
		},
	}
	r := &ComposeRunner{docker: mock}

	if err := r.Stop(context.Background(), svc); err != nil {
		t.Errorf("Stop: %v", err)
	}
	if !strings.Contains(seen, ":") {
		t.Errorf("expected overlay path in compose arg, got %s", seen)
	}
}

func TestComposeRunner_Status_Running(t *testing.T) {
	svc := makeComposeSvc(t)
	mock := &mocks.MockDockerRunner{
		GetServicesStatusWithContextFunc: func(
			_ context.Context, _ string,
		) (map[string]string, error) {
			return map[string]string{"web": "running", "db": "stopped"}, nil
		},
	}
	r := &ComposeRunner{docker: mock}

	status, err := r.Status(context.Background(), svc)
	if err != nil {
		t.Errorf("Status: %v", err)
	}
	if status != "running" {
		t.Errorf("expected running, got %s", status)
	}
}

func TestComposeRunner_Status_Stopped(t *testing.T) {
	svc := makeComposeSvc(t)
	mock := &mocks.MockDockerRunner{
		GetServicesStatusWithContextFunc: func(
			_ context.Context, _ string,
		) (map[string]string, error) {
			return map[string]string{"web": "stopped"}, nil
		},
	}
	r := &ComposeRunner{docker: mock}

	status, err := r.Status(context.Background(), svc)
	if err != nil {
		t.Errorf("Status: %v", err)
	}
	if status != "stopped" {
		t.Errorf("expected stopped, got %s", status)
	}
}

func TestComposeRunner_Status_Error(t *testing.T) {
	svc := makeComposeSvc(t)
	mock := &mocks.MockDockerRunner{
		GetServicesStatusWithContextFunc: func(
			_ context.Context, _ string,
		) (map[string]string, error) {
			return nil, os.ErrPermission
		},
	}
	r := &ComposeRunner{docker: mock}

	status, err := r.Status(context.Background(), svc)
	if err == nil {
		t.Error("expected error")
	}
	if status != "unknown" {
		t.Errorf("expected unknown, got %s", status)
	}
}

func TestComposeRunner_Logs(t *testing.T) {
	svc := makeComposeSvc(t)
	called := false
	mock := &mocks.MockDockerRunner{
		ViewLogsWithContextFunc: func(
			_ context.Context, _ string, opts interfaces.LogsOptions,
		) error {
			called = true
			if !opts.Follow {
				t.Error("expected Follow=true")
			}
			if opts.Tail != 10 {
				t.Errorf("expected tail=10, got %d", opts.Tail)
			}
			return nil
		},
	}
	r := &ComposeRunner{docker: mock}

	if err := r.Logs(context.Background(), svc, true, 10); err != nil {
		t.Errorf("Logs: %v", err)
	}
	if !called {
		t.Error("ViewLogsWithContext was not called")
	}
}

func TestComposeRunner_Restart(t *testing.T) {
	svc := makeComposeSvc(t)
	var downs, ups int
	mock := &mocks.MockDockerRunner{
		GetAvailableServicesWithContextFunc: func(
			_ context.Context, _ string,
		) ([]string, error) {
			return []string{"web"}, nil
		},
		DownWithContextFunc: func(_ context.Context, _ string) error {
			downs++
			return nil
		},
		UpWithContextFunc: func(_ context.Context, _ string) error {
			ups++
			return nil
		},
	}
	r := &ComposeRunner{docker: mock}

	if err := r.Restart(context.Background(), svc); err != nil {
		t.Errorf("Restart: %v", err)
	}
	if downs != 1 {
		t.Errorf("expected 1 Down call, got %d", downs)
	}
	if ups != 1 {
		t.Errorf("expected 1 Up call, got %d", ups)
	}
}
