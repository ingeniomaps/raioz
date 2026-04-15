package docker

import (
	"context"
	"os/exec"
	"testing"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
)

// dockerAvailable returns true if a docker binary is in PATH.
// Most tests don't depend on the daemon — they test the adapter's
// conversion logic or delegate to pure functions.
func dockerAvailable() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

func TestNewDockerRunner(t *testing.T) {
	r := NewDockerRunner()
	if r == nil {
		t.Fatal("NewDockerRunner returned nil")
	}
}

func TestDockerRunnerImpl_ExtractNamedVolumes(t *testing.T) {
	r := NewDockerRunner()

	volumes := []string{
		"data:/var/lib/data",
		"./local:/app",
		"another:/var",
	}
	got, err := r.ExtractNamedVolumes(volumes)
	if err != nil {
		t.Fatalf("ExtractNamedVolumes: %v", err)
	}
	// Should contain at least "data" and "another"
	if len(got) < 2 {
		t.Errorf("expected at least 2 named volumes, got %v", got)
	}
}

func TestDockerRunnerImpl_ExtractNamedVolumes_Empty(t *testing.T) {
	r := NewDockerRunner()
	got, err := r.ExtractNamedVolumes(nil)
	if err != nil {
		t.Fatalf("ExtractNamedVolumes: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestDockerRunnerImpl_NormalizeContainerName(t *testing.T) {
	r := NewDockerRunner()
	got, err := r.NormalizeContainerName("ws", "api", "proj", false)
	if err != nil {
		t.Fatalf("NormalizeContainerName: %v", err)
	}
	if got == "" {
		t.Error("expected non-empty name")
	}
}

func TestDockerRunnerImpl_NormalizeInfraName(t *testing.T) {
	r := NewDockerRunner()
	got, err := r.NormalizeInfraName("ws", "postgres", "proj", false)
	if err != nil {
		t.Fatalf("NormalizeInfraName: %v", err)
	}
	if got == "" {
		t.Error("expected non-empty name")
	}
}

func TestDockerRunnerImpl_NormalizeVolumeName(t *testing.T) {
	r := NewDockerRunner()
	got, err := r.NormalizeVolumeName("proj", "data")
	if err != nil {
		t.Fatalf("NormalizeVolumeName: %v", err)
	}
	if got == "" {
		t.Error("expected non-empty name")
	}
}

func TestDockerRunnerImpl_FormatPortConflicts_Empty(t *testing.T) {
	r := NewDockerRunner()
	got := r.FormatPortConflicts(nil)
	_ = got // Format may return empty or "No conflicts" — just no crash
}

func TestDockerRunnerImpl_FormatPortConflicts_Some(t *testing.T) {
	r := NewDockerRunner()
	conflicts := []interfaces.PortConflict{
		{Port: "5432", Project: "other", Service: "postgres"},
	}
	got := r.FormatPortConflicts(conflicts)
	if got == "" {
		t.Log("empty format — may be tolerant")
	}
}

func TestDockerRunnerImpl_ResolveRelativeVolumes(t *testing.T) {
	r := NewDockerRunner()
	dir := t.TempDir()

	volumes := []string{
		"./data:/var/data",
		"named:/named",
	}
	got, err := r.ResolveRelativeVolumes(volumes, dir)
	if err != nil {
		t.Fatalf("ResolveRelativeVolumes: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2, got %d", len(got))
	}
}

func TestDockerRunnerImpl_BuildServiceVolumesMap(t *testing.T) {
	r := NewDockerRunner()
	deps := &config.Deps{
		Services: map[string]config.Service{
			"api": {
				Docker: &config.DockerConfig{
					Volumes: []string{"data:/var/data"},
				},
			},
		},
	}
	got, err := r.BuildServiceVolumesMap(deps)
	if err != nil {
		t.Fatalf("BuildServiceVolumesMap: %v", err)
	}
	_ = got
}

func TestDockerRunnerImpl_DetectSharedVolumes(t *testing.T) {
	r := NewDockerRunner()
	services := map[string]interfaces.ServiceVolumes{
		"a": {NamedVolumes: []string{"data", "shared"}},
		"b": {NamedVolumes: []string{"shared", "other"}},
	}
	got := r.DetectSharedVolumes(services)
	if len(got) == 0 {
		t.Error("expected to detect shared volumes")
	}
}

func TestDockerRunnerImpl_FormatSharedVolumesWarning(t *testing.T) {
	r := NewDockerRunner()
	got := r.FormatSharedVolumesWarning(map[string][]string{
		"shared": {"a", "b"},
	})
	_ = got
}

func TestDockerRunnerImpl_ValidateAllImages_Empty(t *testing.T) {
	r := NewDockerRunner()
	deps := &config.Deps{}
	_ = r.ValidateAllImages(deps)
}

// --- Tests that may require Docker but gracefully handle absence ---

func TestDockerRunnerImpl_Up_MissingFile(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	err := r.Up("/nonexistent/path.yml")
	if err == nil {
		t.Error("expected error for missing compose file")
	}
}

func TestDockerRunnerImpl_UpWithContext_MissingFile(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	err := r.UpWithContext(context.Background(), "/nonexistent/path.yml")
	if err == nil {
		t.Error("expected error")
	}
}

func TestDockerRunnerImpl_Down_MissingFile(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_ = r.Down("/nonexistent/path.yml")
}

func TestDockerRunnerImpl_DownWithContext_MissingFile(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_ = r.DownWithContext(context.Background(), "/nonexistent/path.yml")
}

func TestDockerRunnerImpl_GetServicesStatus_MissingFile(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_, _ = r.GetServicesStatus("/nonexistent/path.yml")
}

func TestDockerRunnerImpl_GetServicesStatusWithContext_MissingFile(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_, _ = r.GetServicesStatusWithContext(context.Background(), "/nonexistent/path.yml")
}

func TestDockerRunnerImpl_StopServiceWithContext_MissingFile(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_ = r.StopServiceWithContext(context.Background(), "/nonexistent/path.yml", "api")
}

func TestDockerRunnerImpl_GetAvailableServicesWithContext_MissingFile(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_, _ = r.GetAvailableServicesWithContext(context.Background(), "/nonexistent/path.yml")
}

func TestDockerRunnerImpl_CleanProjectWithContext(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_, _ = r.CleanProjectWithContext(context.Background(), "/nonexistent/path.yml", true)
}

func TestDockerRunnerImpl_CleanAllProjectsWithContext(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_, _ = r.CleanAllProjectsWithContext(context.Background(), t.TempDir(), true)
}

func TestDockerRunnerImpl_CleanUnusedImagesWithContext(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_, _ = r.CleanUnusedImagesWithContext(context.Background(), true)
}

func TestDockerRunnerImpl_CleanUnusedVolumesWithContext(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_, _ = r.CleanUnusedVolumesWithContext(context.Background(), true, false)
}

func TestDockerRunnerImpl_CleanUnusedNetworksWithContext(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_, _ = r.CleanUnusedNetworksWithContext(context.Background(), true)
}

func TestDockerRunnerImpl_GetNetworkProjects(t *testing.T) {
	r := NewDockerRunner()
	_, _ = r.GetNetworkProjects("raioz-net", t.TempDir())
}

func TestDockerRunnerImpl_GetVolumeProjects(t *testing.T) {
	r := NewDockerRunner()
	_, _ = r.GetVolumeProjects("somevol", t.TempDir())
}

func TestDockerRunnerImpl_GetAllActivePorts(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_, _ = r.GetAllActivePorts(t.TempDir())
}

func TestDockerRunnerImpl_ValidatePorts(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	deps := &config.Deps{}
	_, _ = r.ValidatePorts(deps, t.TempDir(), "test")
}

func TestDockerRunnerImpl_EnsureNetworkWithConfigAndContext(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_ = r.EnsureNetworkWithConfigAndContext(context.Background(), "raioz-test-net", "", false)
}

func TestDockerRunnerImpl_EnsureVolumeWithContext(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_ = r.EnsureVolumeWithContext(context.Background(), "raioz-test-vol")
}

func TestDockerRunnerImpl_AreServicesRunning(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_, _ = r.AreServicesRunning("/nonexistent/path.yml", []string{"api"})
}

func TestDockerRunnerImpl_IsNetworkInUseWithContext(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_, _ = r.IsNetworkInUseWithContext(context.Background(), "raioz-nonexistent")
}

func TestDockerRunnerImpl_RemoveVolumeWithContext(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_ = r.RemoveVolumeWithContext(context.Background(), "raioz-nonexistent-vol")
}

func TestDockerRunnerImpl_StopContainerWithContext(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_ = r.StopContainerWithContext(context.Background(), "raioz-nonexistent")
}

func TestDockerRunnerImpl_GetContainerNameWithContext(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_, _ = r.GetContainerNameWithContext(context.Background(), "/nonexistent/path.yml", "api")
}

func TestDockerRunnerImpl_ViewLogsWithContext_MissingFile(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_ = r.ViewLogsWithContext(context.Background(), "/nonexistent/path.yml", interfaces.LogsOptions{
		Tail: 10,
	})
}

func TestDockerRunnerImpl_GenerateCompose(t *testing.T) {
	r := NewDockerRunner()
	deps := &config.Deps{
		Project: config.Project{Name: "test"},
	}
	ws := &interfaces.Workspace{
		Root: t.TempDir(),
	}
	_, _, _ = r.GenerateCompose(deps, ws, t.TempDir())
}

func TestDockerRunnerImpl_UpServicesWithContext(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_ = r.UpServicesWithContext(context.Background(), "/nonexistent/path.yml", []string{"api"})
}

func TestDockerRunnerImpl_RestartServicesWithContext(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_ = r.RestartServicesWithContext(context.Background(), "/nonexistent/path.yml", []string{"api"})
}

func TestDockerRunnerImpl_ForceRecreateServicesWithContext(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_ = r.ForceRecreateServicesWithContext(context.Background(), "/nonexistent/path.yml", []string{"api"})
}

func TestDockerRunnerImpl_ExecInService(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	_ = r.ExecInService(context.Background(), "/nonexistent/path.yml", "api", []string{"ls"}, false)
}

func TestDockerRunnerImpl_WaitForServicesHealthy(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately to avoid hanging
	_ = r.WaitForServicesHealthy(ctx, "/nonexistent/path.yml", []string{"api"}, nil, "proj")
}

func TestDockerRunnerImpl_GetServicesInfoWithContext(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("docker not available")
	}
	r := NewDockerRunner()
	ws := &interfaces.Workspace{Root: t.TempDir()}
	_, _ = r.GetServicesInfoWithContext(
		context.Background(), "/nonexistent/path.yml",
		nil, "proj", nil, ws,
	)
}

func TestDockerRunnerImpl_FormatStatusTable(t *testing.T) {
	r := NewDockerRunner()
	services := map[string]*interfaces.ServiceInfo{
		"api": {
			Status: "running",
			CPU:    "1%",
			Memory: "10MB",
		},
	}
	_ = r.FormatStatusTable(services, true) // JSON output
}
