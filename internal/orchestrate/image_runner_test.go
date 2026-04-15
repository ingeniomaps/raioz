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
	"raioz/internal/naming"

	"gopkg.in/yaml.v3"
)

func makeImageSvc() interfaces.ServiceContext {
	return interfaces.ServiceContext{
		Name:          "postgres",
		ProjectName:   "proj",
		ContainerName: "raioz-proj-postgres",
		NetworkName:   "proj-net",
		Ports:         []string{"5432:5432"},
		EnvVars: map[string]string{
			"RAIOZ_IMAGE":       "postgres:16",
			"POSTGRES_USER":     "admin",
			"POSTGRES_PASSWORD": "secret",
		},
		Detection: detect.DetectResult{Runtime: detect.RuntimeImage},
	}
}

func TestImageRunner_ComposePath(t *testing.T) {
	r := &ImageRunner{}
	svc := makeImageSvc()

	path := r.composePath(svc)
	if filepath.Base(path) != "docker-compose.yml" {
		t.Errorf("expected docker-compose.yml basename, got %s", filepath.Base(path))
	}

	// Directory should be under project-isolated deps dir
	expectedDir := filepath.Dir(naming.DepComposePath(svc.ProjectName, svc.Name))
	if filepath.Dir(path) != expectedDir {
		t.Errorf("expected dir %s, got %s", expectedDir, filepath.Dir(path))
	}
}

func TestImageRunner_WriteCompose(t *testing.T) {
	// Override temp dir location by using a project name that writes into t.TempDir
	// Since naming uses os.TempDir(), we can't easily redirect, but writeCompose
	// will create the dir beneath os.TempDir() — use a unique project name.
	svc := makeImageSvc()
	svc.ProjectName = "writeCompose-" + t.Name()
	t.Cleanup(func() {
		os.RemoveAll(naming.TempDir(svc.ProjectName))
	})

	r := &ImageRunner{}
	compose := map[string]any{
		"services": map[string]any{
			svc.Name: map[string]any{"image": "postgres:16"},
		},
	}
	path, err := r.writeCompose(svc, compose)
	if err != nil {
		t.Fatalf("writeCompose: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("compose file not created: %v", err)
	}

	data, _ := os.ReadFile(path)
	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Errorf("compose is not valid yaml: %v", err)
	}
}

func TestImageRunner_GenerateCompose(t *testing.T) {
	svc := makeImageSvc()
	svc.ProjectName = "gen-" + t.Name()
	// Keep ContainerName consistent with ProjectName: the label-stamp logic
	// treats a mismatch as "shared dep" and omits the project label on purpose.
	svc.ContainerName = naming.Container(svc.ProjectName, svc.Name)
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })

	r := &ImageRunner{}
	path, err := r.generateCompose(svc)
	if err != nil {
		t.Fatalf("generateCompose: %v", err)
	}

	data, _ := os.ReadFile(path)
	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse compose: %v", err)
	}

	services, ok := parsed["services"].(map[string]any)
	if !ok {
		t.Fatal("expected services map")
	}
	pg, ok := services["postgres"].(map[string]any)
	if !ok {
		t.Fatal("expected postgres service")
	}

	if img, _ := pg["image"].(string); img != "postgres:16" {
		t.Errorf("expected image postgres:16, got %v", pg["image"])
	}

	if name, _ := pg["container_name"].(string); name != svc.ContainerName {
		t.Errorf("expected container_name %s, got %v", svc.ContainerName, pg["container_name"])
	}

	// Raioz labels must be emitted so down flows can sweep by label.
	rawLabels, ok := pg["labels"].(map[string]any)
	if !ok {
		t.Fatalf("expected labels map, got %T", pg["labels"])
	}
	if rawLabels[naming.LabelManaged] != "true" {
		t.Errorf("expected %s=true, got %v", naming.LabelManaged, rawLabels[naming.LabelManaged])
	}
	if rawLabels[naming.LabelProject] != svc.ProjectName {
		t.Errorf("expected project label %s, got %v", svc.ProjectName, rawLabels[naming.LabelProject])
	}
	if rawLabels[naming.LabelKind] != naming.KindDependency {
		t.Errorf("expected kind=dependency, got %v", rawLabels[naming.LabelKind])
	}
	if rawLabels[naming.LabelService] != svc.Name {
		t.Errorf("expected service=%s, got %v", svc.Name, rawLabels[naming.LabelService])
	}

	// Ports should be present
	if _, ok := pg["ports"]; !ok {
		t.Error("expected ports field")
	}

	// extra_hosts should include host-gateway
	extraHosts, ok := pg["extra_hosts"].([]any)
	if !ok || len(extraHosts) == 0 {
		t.Error("expected extra_hosts")
	}

	// Environment should contain user vars but not RAIOZ_ prefixed
	envList, ok := pg["environment"].([]any)
	if !ok {
		t.Fatal("expected environment list")
	}
	var joined string
	for _, e := range envList {
		joined += e.(string) + " "
	}
	if !strings.Contains(joined, "POSTGRES_USER=admin") {
		t.Errorf("expected POSTGRES_USER, got %s", joined)
	}
	if strings.Contains(joined, "RAIOZ_IMAGE") {
		t.Errorf("RAIOZ_IMAGE should be filtered, got %s", joined)
	}

	// Networks should be external
	networks, ok := parsed["networks"].(map[string]any)
	if !ok {
		t.Fatal("expected networks map")
	}
	net, _ := networks[svc.NetworkName].(map[string]any)
	if ext, _ := net["external"].(bool); !ext {
		t.Error("expected network external=true")
	}
}

func TestImageRunner_GenerateCompose_WithEnvFile(t *testing.T) {
	svc := makeImageSvc()
	svc.ProjectName = "genenv-" + t.Name()
	svc.EnvVars["RAIOZ_ENV_FILE"] = ".env.postgres"
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })

	r := &ImageRunner{}
	path, err := r.generateCompose(svc)
	if err != nil {
		t.Fatalf("generateCompose: %v", err)
	}

	data, _ := os.ReadFile(path)
	var parsed map[string]any
	_ = yaml.Unmarshal(data, &parsed)

	services := parsed["services"].(map[string]any)
	pg := services["postgres"].(map[string]any)

	envFile, ok := pg["env_file"].([]any)
	if !ok {
		t.Fatal("expected env_file list")
	}
	if envFile[0].(string) != ".env.postgres" {
		t.Errorf("expected .env.postgres, got %v", envFile[0])
	}
}

func TestImageRunner_GenerateCompose_NoPorts(t *testing.T) {
	svc := makeImageSvc()
	svc.ProjectName = "noports-" + t.Name()
	svc.Ports = nil
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })

	r := &ImageRunner{}
	path, err := r.generateCompose(svc)
	if err != nil {
		t.Fatalf("generateCompose: %v", err)
	}

	data, _ := os.ReadFile(path)
	var parsed map[string]any
	_ = yaml.Unmarshal(data, &parsed)

	services := parsed["services"].(map[string]any)
	pg := services["postgres"].(map[string]any)

	if _, ok := pg["ports"]; ok {
		t.Error("ports should not be set when Ports is empty")
	}
}

func TestImageRunner_Start(t *testing.T) {
	svc := makeImageSvc()
	svc.ProjectName = "start-" + t.Name()
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })

	called := false
	mock := &mocks.MockDockerRunner{
		UpWithContextFunc: func(_ context.Context, composePath string) error {
			called = true
			if !strings.HasSuffix(composePath, "docker-compose.yml") {
				t.Errorf("unexpected compose path: %s", composePath)
			}
			return nil
		},
	}
	r := &ImageRunner{docker: mock}

	if err := r.Start(context.Background(), svc); err != nil {
		t.Errorf("Start: %v", err)
	}
	if !called {
		t.Error("UpWithContext was not called")
	}
}

func TestImageRunner_Stop_NoFile(t *testing.T) {
	svc := makeImageSvc()
	svc.ProjectName = "stop-nofile-" + t.Name()
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })

	called := false
	mock := &mocks.MockDockerRunner{
		DownWithContextFunc: func(_ context.Context, _ string) error {
			called = true
			return nil
		},
	}
	r := &ImageRunner{docker: mock}

	// No compose file exists yet
	if err := r.Stop(context.Background(), svc); err != nil {
		t.Errorf("Stop: %v", err)
	}
	if called {
		t.Error("DownWithContext should not be called when file missing")
	}
}

func TestImageRunner_Stop_WithFile(t *testing.T) {
	svc := makeImageSvc()
	svc.ProjectName = "stop-file-" + t.Name()
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })

	// Generate the compose file first
	r := &ImageRunner{docker: &mocks.MockDockerRunner{}}
	if _, err := r.generateCompose(svc); err != nil {
		t.Fatalf("generateCompose: %v", err)
	}

	called := false
	r.docker = &mocks.MockDockerRunner{
		DownWithContextFunc: func(_ context.Context, _ string) error {
			called = true
			return nil
		},
	}

	if err := r.Stop(context.Background(), svc); err != nil {
		t.Errorf("Stop: %v", err)
	}
	if !called {
		t.Error("DownWithContext was not called")
	}
}

func TestImageRunner_Status_NotFound(t *testing.T) {
	svc := makeImageSvc()

	r := &ImageRunner{docker: &mocks.MockDockerRunner{
		GetContainerStatusByNameFunc: func(_ context.Context, _ string) (string, error) {
			return "", nil
		},
	}}
	status, err := r.Status(context.Background(), svc)
	if err != nil {
		t.Errorf("Status: %v", err)
	}
	if status != "stopped" {
		t.Errorf("expected stopped, got %s", status)
	}
}

func TestImageRunner_Status_Running(t *testing.T) {
	svc := makeImageSvc()

	r := &ImageRunner{docker: &mocks.MockDockerRunner{
		GetContainerStatusByNameFunc: func(_ context.Context, name string) (string, error) {
			if name != svc.ContainerName {
				t.Errorf("expected %s, got %s", svc.ContainerName, name)
			}
			return "running", nil
		},
	}}

	status, err := r.Status(context.Background(), svc)
	if err != nil {
		t.Errorf("Status: %v", err)
	}
	if status != "running" {
		t.Errorf("expected running, got %s", status)
	}
}

func TestImageRunner_Status_NonRunningState(t *testing.T) {
	svc := makeImageSvc()

	r := &ImageRunner{docker: &mocks.MockDockerRunner{
		GetContainerStatusByNameFunc: func(_ context.Context, _ string) (string, error) {
			return "exited", nil
		},
	}}

	status, err := r.Status(context.Background(), svc)
	if err != nil {
		t.Errorf("Status: %v", err)
	}
	if status != "exited" {
		t.Errorf("expected exited, got %s", status)
	}
}

func TestImageRunner_Status_Error(t *testing.T) {
	svc := makeImageSvc()

	r := &ImageRunner{docker: &mocks.MockDockerRunner{
		GetContainerStatusByNameFunc: func(_ context.Context, _ string) (string, error) {
			return "", os.ErrPermission
		},
	}}

	status, err := r.Status(context.Background(), svc)
	if err == nil {
		t.Error("expected error")
	}
	if status != "unknown" {
		t.Errorf("expected unknown, got %s", status)
	}
}

// TestImageRunner_GenerateCompose_SharedDepOmitsProjectLabel verifies that a
// workspace-shared dep is stamped WITHOUT com.raioz.project, so `raioz down`
// of a single project can't sweep it away while sibling projects still use it.
func TestImageRunner_GenerateCompose_SharedDepOmitsProjectLabel(t *testing.T) {
	naming.SetPrefix("acme")
	defer naming.SetPrefix("")

	svc := makeImageSvc()
	svc.ProjectName = "shared-" + t.Name()
	// Simulate orchestration.go resolving the container name via DepContainer
	// for a workspace — yields `acme-postgres`, distinct from `acme-<proj>-postgres`.
	svc.ContainerName = naming.SharedContainer(svc.Name)
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })

	r := &ImageRunner{}
	path, err := r.generateCompose(svc)
	if err != nil {
		t.Fatalf("generateCompose: %v", err)
	}

	data, _ := os.ReadFile(path)
	var parsed map[string]any
	_ = yaml.Unmarshal(data, &parsed)
	pg := parsed["services"].(map[string]any)["postgres"].(map[string]any)
	labels := pg["labels"].(map[string]any)

	if _, ok := labels[naming.LabelProject]; ok {
		t.Error("shared dep must NOT carry a project label (would break workspace sharing)")
	}
	if labels[naming.LabelWorkspace] != "acme" {
		t.Errorf("expected workspace=acme, got %v", labels[naming.LabelWorkspace])
	}
	if labels[naming.LabelKind] != naming.KindDependency {
		t.Errorf("expected kind=dependency, got %v", labels[naming.LabelKind])
	}
}

// TestImageRunner_BuildComposeSpec_ExternalCompose confirms that when the
// dep declares `compose:` in raioz.yaml, the runner uses those files
// (converted to absolute paths) plus a generated overlay — not the
// auto-generated image compose.
func TestImageRunner_BuildComposeSpec_ExternalCompose(t *testing.T) {
	naming.SetPrefix("acme")
	defer naming.SetPrefix("")

	projDir := t.TempDir()
	userCompose := filepath.Join(projDir, "postgres.yml")
	if err := os.WriteFile(userCompose, []byte("services:\n  postgres:\n    image: postgres\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := makeImageSvc()
	svc.ProjectName = "extcompose-" + t.Name()
	svc.ContainerName = naming.SharedContainer(svc.Name)
	svc.ExternalComposeFiles = []string{userCompose}
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })

	r := &ImageRunner{}
	spec, err := r.buildComposeSpec(svc)
	if err != nil {
		t.Fatalf("buildComposeSpec: %v", err)
	}

	if !strings.Contains(spec, "postgres.yml") {
		t.Errorf("user compose file missing from spec: %s", spec)
	}
	if !strings.Contains(spec, "raioz-overlay.yml") {
		t.Errorf("raioz overlay missing from spec: %s", spec)
	}

	// Overlay file must exist on disk with network + labels for the dep.
	overlayPath := r.overlayPath(svc)
	data, err := os.ReadFile(overlayPath)
	if err != nil {
		t.Fatalf("overlay not written: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, naming.LabelManaged) ||
		!strings.Contains(body, naming.LabelKind) {
		t.Errorf("overlay missing raioz labels:\n%s", body)
	}
	if !strings.Contains(body, svc.NetworkName) {
		t.Errorf("overlay missing network %q:\n%s", svc.NetworkName, body)
	}
}

func TestImageRunner_BuildComposeSpec_ImageMode(t *testing.T) {
	svc := makeImageSvc()
	svc.ProjectName = "imgmode-" + t.Name()
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })
	// no ExternalComposeFiles → image mode

	r := &ImageRunner{}
	spec, err := r.buildComposeSpec(svc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(spec, "raioz-overlay.yml") {
		t.Errorf("image mode must not emit overlay, spec=%s", spec)
	}
	if !strings.Contains(spec, "docker-compose.yml") {
		t.Errorf("image mode must use generated docker-compose.yml, spec=%s", spec)
	}
}

func TestImageRunner_Logs(t *testing.T) {
	svc := makeImageSvc()
	called := false
	mock := &mocks.MockDockerRunner{
		ViewLogsWithContextFunc: func(
			_ context.Context, _ string, opts interfaces.LogsOptions,
		) error {
			called = true
			if opts.Tail != 50 {
				t.Errorf("expected tail=50, got %d", opts.Tail)
			}
			return nil
		},
	}
	r := &ImageRunner{docker: mock}

	if err := r.Logs(context.Background(), svc, false, 50); err != nil {
		t.Errorf("Logs: %v", err)
	}
	if !called {
		t.Error("ViewLogsWithContext was not called")
	}
}

func TestImageRunner_Restart(t *testing.T) {
	svc := makeImageSvc()
	svc.ProjectName = "restart-" + t.Name()
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })

	ups := 0
	mock := &mocks.MockDockerRunner{
		UpWithContextFunc: func(_ context.Context, _ string) error {
			ups++
			return nil
		},
	}
	r := &ImageRunner{docker: mock}

	if err := r.Restart(context.Background(), svc); err != nil {
		t.Errorf("Restart: %v", err)
	}
	if ups != 1 {
		t.Errorf("expected 1 Up call, got %d", ups)
	}
}
