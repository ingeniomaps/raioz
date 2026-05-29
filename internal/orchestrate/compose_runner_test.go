package orchestrate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/docker"
	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/mocks"
	"raioz/internal/naming"

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
		Detection: models.DetectResult{
			Runtime:     models.RuntimeCompose,
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
		Detection:   models.DetectResult{ComposeFile: composePath},
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

func TestComposeRunner_CreateNetworkOverlay_AliasesCanonicalName(t *testing.T) {
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

	want := "raioz-" + svc.ProjectName + "-" + svc.Name
	for _, name := range []string{"web", "db"} {
		entry, ok := services[name].(map[string]any)
		if !ok {
			t.Fatalf("service %q missing or wrong shape in overlay", name)
		}
		nets, ok := entry["networks"].(map[string]any)
		if !ok {
			t.Fatalf("service %q networks should be a map (long form)", name)
		}
		netCfg, ok := nets[svc.NetworkName].(map[string]any)
		if !ok {
			t.Fatalf("service %q missing %q in networks", name, svc.NetworkName)
		}
		aliases, ok := netCfg["aliases"].([]any)
		if !ok {
			t.Fatalf("service %q missing aliases on %q", name, svc.NetworkName)
		}
		found := false
		for _, a := range aliases {
			if a == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("service %q aliases %v missing canonical %q",
				name, aliases, want)
		}
		if _, ok := nets["default"]; !ok {
			t.Errorf("service %q missing default network entry", name)
		}
	}
}

// Regression: ComposeRunner must inject host-gateway so
// containers spawned from the user's compose file can resolve
// host.docker.internal on Linux without Docker Desktop. ImageRunner
// and DockerfileRunner already do this; ComposeRunner used to omit it.
func TestComposeRunner_CreateNetworkOverlay_InjectsHostGateway(t *testing.T) {
	svc := makeComposeSvc(t)
	mock := &mocks.MockDockerRunner{
		GetAvailableServicesWithContextFunc: func(
			_ context.Context, _ string,
		) ([]string, error) {
			return []string{"web"}, nil
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
	services := parsed["services"].(map[string]any)
	web := services["web"].(map[string]any)
	extra, ok := web["extra_hosts"].([]any)
	if !ok {
		t.Fatalf("service web missing extra_hosts; got %+v", web)
	}
	want := "host.docker.internal:host-gateway"
	for _, e := range extra {
		if e == want {
			return
		}
	}
	t.Errorf("extra_hosts %v missing %q", extra, want)
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

// TestComposeRunner_Start_InjectsInterpolationAndEnvFiles is the regression
// test for the runner asymmetry where a service declared with `compose:` could
// not interpolate ${NETWORK_NAME}/${PROJECT_PREFIX} nor read its env: files,
// while an identical dependency could. ComposeRunner.Start must enrich the
// docker compose context exactly like ImageRunner: env: files become
// --env-file flags and the computed vars are exported for interpolation.
func TestComposeRunner_Start_InjectsInterpolationAndEnvFiles(t *testing.T) {
	naming.SetPrefix("acme")
	t.Cleanup(func() { naming.SetPrefix("") })

	svc := makeComposeSvc(t)
	svc.NetworkName = "acme-net"
	envFile := filepath.Join(svc.Path, ".env.app")
	if err := os.WriteFile(envFile, []byte("FOO=bar\n"), 0644); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	svc.EnvFilePaths = []string{envFile}

	var captured context.Context
	mock := &mocks.MockDockerRunner{
		GetAvailableServicesWithContextFunc: func(
			_ context.Context, _ string,
		) ([]string, error) {
			return []string{"web"}, nil
		},
		UpWithContextFunc: func(ctx context.Context, _ string) error {
			captured = ctx
			return nil
		},
	}
	r := &ComposeRunner{docker: mock}

	if err := r.Start(context.Background(), svc); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if captured == nil {
		t.Fatal("UpWithContext was not called")
	}

	// env: files must reach docker compose as --env-file sources.
	gotFiles := docker.ComposeEnvFilesFromContext(captured)
	if len(gotFiles) != 1 || gotFiles[0] != envFile {
		t.Errorf("env files = %v, want [%s]", gotFiles, envFile)
	}

	// Interpolation vars must match what ImageRunner exports for the same svc.
	gotEnv := docker.ComposeExtraEnvFromContext(captured)
	if gotEnv["NETWORK_NAME"] != "acme-net" {
		t.Errorf("NETWORK_NAME = %q, want %q", gotEnv["NETWORK_NAME"], "acme-net")
	}
	if gotEnv["PROJECT_PREFIX"] != "acme" {
		t.Errorf("PROJECT_PREFIX = %q, want %q", gotEnv["PROJECT_PREFIX"], "acme")
	}

	// Parity check: the exported interpolation env is exactly what the
	// dependency path (composeInterpolationEnv) would produce.
	want := composeInterpolationEnv(svc)
	if len(gotEnv) != len(want) {
		t.Fatalf("extra env = %v, want %v", gotEnv, want)
	}
	for k, v := range want {
		if gotEnv[k] != v {
			t.Errorf("extra env[%q] = %q, want %q", k, gotEnv[k], v)
		}
	}
}

// TestComposeRunner_Start_NoWorkspaceStripsPrefix mirrors the dependency
// behavior where, without a workspace, PROJECT_PREFIX is omitted so a
// compose's `:-default` fallback kicks in instead of resolving to the literal
// "raioz" prefix.
func TestComposeRunner_Start_NoWorkspaceStripsPrefix(t *testing.T) {
	naming.SetPrefix("") // default prefix → no workspace
	svc := makeComposeSvc(t)
	svc.NetworkName = "proj-net"

	var captured context.Context
	mock := &mocks.MockDockerRunner{
		GetAvailableServicesWithContextFunc: func(
			_ context.Context, _ string,
		) ([]string, error) {
			return []string{"web"}, nil
		},
		UpWithContextFunc: func(ctx context.Context, _ string) error {
			captured = ctx
			return nil
		},
	}
	r := &ComposeRunner{docker: mock}

	if err := r.Start(context.Background(), svc); err != nil {
		t.Fatalf("Start: %v", err)
	}
	gotEnv := docker.ComposeExtraEnvFromContext(captured)
	if _, ok := gotEnv["PROJECT_PREFIX"]; ok {
		t.Errorf("PROJECT_PREFIX must be absent without a workspace, got %v", gotEnv)
	}
	if gotEnv["NETWORK_NAME"] != "proj-net" {
		t.Errorf("NETWORK_NAME = %q, want %q", gotEnv["NETWORK_NAME"], "proj-net")
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
