package orchestrate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/detect"
	"raioz/internal/domain/interfaces"
	"raioz/internal/mocks"
	"raioz/internal/naming"
)

func TestNewDispatcher(t *testing.T) {
	mock := &mocks.MockDockerRunner{}
	d := NewDispatcher(mock)

	if d.compose == nil {
		t.Error("compose runner not initialized")
	}
	if d.dockerfile == nil {
		t.Error("dockerfile runner not initialized")
	}
	if d.host == nil {
		t.Error("host runner not initialized")
	}
	if d.image == nil {
		t.Error("image runner not initialized")
	}
	if d.compose.docker == nil {
		t.Error("compose runner has no docker")
	}
	if d.image.docker == nil {
		t.Error("image runner has no docker")
	}
}

func TestDispatcher_GetHostPID(t *testing.T) {
	d := NewDispatcher(&mocks.MockDockerRunner{})
	d.host.SetPID("api", 42)
	if got := d.GetHostPID("api"); got != 42 {
		t.Errorf("expected 42, got %d", got)
	}
	if got := d.GetHostPID("missing"); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestDispatcher_Start_Image(t *testing.T) {
	svc := interfaces.ServiceContext{
		Name:          "redis",
		ProjectName:   "disp-start-" + t.Name(),
		ContainerName: "raioz-proj-redis",
		NetworkName:   "proj-net",
		EnvVars:       map[string]string{"RAIOZ_IMAGE": "redis:7"},
		Detection:     detect.DetectResult{Runtime: detect.RuntimeImage},
	}
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })

	called := false
	mock := &mocks.MockDockerRunner{
		UpWithContextFunc: func(_ context.Context, _ string) error {
			called = true
			return nil
		},
	}
	d := NewDispatcher(mock)

	if err := d.Start(context.Background(), svc); err != nil {
		t.Errorf("Start: %v", err)
	}
	if !called {
		t.Error("expected UpWithContext called")
	}
}

func TestDispatcher_Start_Unknown(t *testing.T) {
	svc := interfaces.ServiceContext{
		Name:      "x",
		Detection: detect.DetectResult{Runtime: detect.RuntimeUnknown},
	}
	d := NewDispatcher(&mocks.MockDockerRunner{})
	if err := d.Start(context.Background(), svc); err == nil {
		t.Error("expected error for unknown runtime")
	}
}

func TestDispatcher_Stop_Image(t *testing.T) {
	svc := interfaces.ServiceContext{
		Name:        "redis",
		ProjectName: "disp-stop-" + t.Name(),
		NetworkName: "proj-net",
		Detection:   detect.DetectResult{Runtime: detect.RuntimeImage},
	}
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })

	d := NewDispatcher(&mocks.MockDockerRunner{})
	// No file yet -> no error, no Down call
	if err := d.Stop(context.Background(), svc); err != nil {
		t.Errorf("Stop: %v", err)
	}
}

func TestDispatcher_Stop_Unknown(t *testing.T) {
	svc := interfaces.ServiceContext{
		Detection: detect.DetectResult{Runtime: detect.RuntimeUnknown},
	}
	d := NewDispatcher(&mocks.MockDockerRunner{})
	if err := d.Stop(context.Background(), svc); err == nil {
		t.Error("expected error for unknown runtime")
	}
}

func TestDispatcher_Restart_Unknown(t *testing.T) {
	svc := interfaces.ServiceContext{
		Detection: detect.DetectResult{Runtime: detect.RuntimeUnknown},
	}
	d := NewDispatcher(&mocks.MockDockerRunner{})
	if err := d.Restart(context.Background(), svc); err == nil {
		t.Error("expected error for unknown runtime")
	}
}

func TestDispatcher_Status_Compose(t *testing.T) {
	dir := t.TempDir()
	composePath := filepath.Join(dir, "docker-compose.yml")
	_ = os.WriteFile(composePath, []byte("services: {}\n"), 0644)

	svc := interfaces.ServiceContext{
		Name:        "api",
		NetworkName: "proj-net",
		Detection: detect.DetectResult{
			Runtime:     detect.RuntimeCompose,
			ComposeFile: composePath,
		},
	}

	mock := &mocks.MockDockerRunner{
		GetServicesStatusWithContextFunc: func(
			_ context.Context, _ string,
		) (map[string]string, error) {
			return map[string]string{"api": "running"}, nil
		},
	}
	d := NewDispatcher(mock)
	status, err := d.Status(context.Background(), svc)
	if err != nil {
		t.Errorf("Status: %v", err)
	}
	if status != "running" {
		t.Errorf("expected running, got %s", status)
	}
}

func TestDispatcher_Status_Unknown(t *testing.T) {
	svc := interfaces.ServiceContext{
		Detection: detect.DetectResult{Runtime: detect.RuntimeUnknown},
	}
	d := NewDispatcher(&mocks.MockDockerRunner{})
	status, err := d.Status(context.Background(), svc)
	if err == nil {
		t.Error("expected error for unknown runtime")
	}
	if status != "unknown" {
		t.Errorf("expected unknown, got %s", status)
	}
}

func TestDispatcher_Logs_Unknown(t *testing.T) {
	svc := interfaces.ServiceContext{
		Detection: detect.DetectResult{Runtime: detect.RuntimeUnknown},
	}
	d := NewDispatcher(&mocks.MockDockerRunner{})
	if err := d.Logs(context.Background(), svc, false, 0); err == nil {
		t.Error("expected error for unknown runtime")
	}
}

func TestDispatcher_Logs_Compose(t *testing.T) {
	dir := t.TempDir()
	composePath := filepath.Join(dir, "docker-compose.yml")
	_ = os.WriteFile(composePath, []byte("services: {}\n"), 0644)

	svc := interfaces.ServiceContext{
		Name: "api",
		Detection: detect.DetectResult{
			Runtime:     detect.RuntimeCompose,
			ComposeFile: composePath,
		},
	}

	called := false
	mock := &mocks.MockDockerRunner{
		ViewLogsWithContextFunc: func(
			_ context.Context, _ string, _ interfaces.LogsOptions,
		) error {
			called = true
			return nil
		},
	}
	d := NewDispatcher(mock)

	if err := d.Logs(context.Background(), svc, true, 100); err != nil {
		t.Errorf("Logs: %v", err)
	}
	if !called {
		t.Error("ViewLogsWithContext was not called")
	}
}

func TestDispatcher_Restart_Image(t *testing.T) {
	svc := interfaces.ServiceContext{
		Name:          "redis",
		ProjectName:   "disp-restart-" + t.Name(),
		ContainerName: "raioz-proj-redis",
		NetworkName:   "proj-net",
		EnvVars:       map[string]string{"RAIOZ_IMAGE": "redis:7"},
		Detection:     detect.DetectResult{Runtime: detect.RuntimeImage},
	}
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })

	ups := 0
	mock := &mocks.MockDockerRunner{
		UpWithContextFunc: func(_ context.Context, _ string) error {
			ups++
			return nil
		},
	}
	d := NewDispatcher(mock)

	if err := d.Restart(context.Background(), svc); err != nil {
		t.Errorf("Restart: %v", err)
	}
	if ups != 1 {
		t.Errorf("expected 1 Up call, got %d", ups)
	}
}
