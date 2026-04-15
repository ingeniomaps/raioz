package docker

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// These tests exercise the Docker-daemon-backed wrappers that pass through
// to *WithContext variants. They don't assert success of the Docker call;
// they only ensure the wrapper path is executed.

// --- ImageExists wrappers: call with nonexistent image ---

func TestImageExists_Nonexistent(t *testing.T) {
	requireDocker(t)
	exists, err := ImageExists("raioz-test-nonexistent:9.9.9-xyz")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if exists {
		t.Error("expected false for nonexistent image")
	}
}

func TestImageExistsWithContext_Nonexistent(t *testing.T) {
	requireDocker(t)
	exists, err := ImageExistsWithContext(
		context.Background(), "raioz-test-nonexistent:9.9.9-xyz",
	)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if exists {
		t.Error("expected false for nonexistent image")
	}
}

// --- GetImageInfo wrappers ---

func TestGetImageInfo_Nonexistent(t *testing.T) {
	requireDocker(t)
	_, err := GetImageInfo("raioz-test-nonexistent:9.9.9-xyz")
	if err == nil {
		t.Error("expected error for nonexistent image")
	}
}

func TestGetImageInfoWithContext_Nonexistent(t *testing.T) {
	requireDocker(t)
	_, err := GetImageInfoWithContext(
		context.Background(), "raioz-test-nonexistent:9.9.9-xyz",
	)
	if err == nil {
		t.Error("expected error for nonexistent image")
	}
}

// --- EnsureImage wrappers: wraps ImageExistsWithContext + Pull ---

func TestEnsureImage_Wrapper(t *testing.T) {
	requireDocker(t)
	// Call EnsureImage with a nonexistent image; it will try to pull,
	// which will fail. We just want to exercise the wrapper entry point.
	_ = EnsureImage("raioz-test-nonexistent-9999:bogus")
}

// --- Volume wrappers ---

func TestVolumeExistsWithContext_Nonexistent(t *testing.T) {
	requireDocker(t)
	exists, err := VolumeExistsWithContext(
		context.Background(), "raioz-test-nonexistent-vol-12345",
	)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if exists {
		t.Error("expected false for nonexistent volume")
	}
}

func TestEnsureVolume_Roundtrip(t *testing.T) {
	requireDocker(t)
	name := "raioz-test-ensure-vol-1"
	_ = RemoveVolume(name)

	if err := EnsureVolume(name); err != nil {
		t.Fatalf("EnsureVolume: %v", err)
	}
	exists, err := VolumeExists(name)
	if err != nil {
		t.Fatalf("VolumeExists: %v", err)
	}
	if !exists {
		t.Error("expected volume to exist")
	}
	// Idempotent
	if err := EnsureVolume(name); err != nil {
		t.Errorf("second EnsureVolume: %v", err)
	}
	if err := RemoveVolume(name); err != nil {
		t.Logf("cleanup: %v", err)
	}
}

func TestRemoveVolume_Nonexistent(t *testing.T) {
	requireDocker(t)
	// Removing a nonexistent volume should be a no-op.
	if err := RemoveVolume("raioz-test-nonexistent-vol-remove-12345"); err != nil {
		t.Errorf("RemoveVolume nonexistent: %v", err)
	}
}

// --- Network wrappers ---

func TestEnsureNetworkWithContext_New(t *testing.T) {
	requireDocker(t)
	name := "raioz-test-ens-net-ctx"
	_ = RemoveNetwork(name)
	if err := EnsureNetworkWithContext(context.Background(), name); err != nil {
		t.Fatalf("err: %v", err)
	}
	_ = RemoveNetwork(name)
}

func TestCreateNetworkWithContext_New(t *testing.T) {
	requireDocker(t)
	name := "raioz-test-create-net-ctx"
	_ = RemoveNetwork(name)
	if err := CreateNetworkWithContext(context.Background(), name); err != nil {
		t.Fatalf("err: %v", err)
	}
	_ = RemoveNetwork(name)
}

func TestIsNetworkInUse_Nonexistent(t *testing.T) {
	requireDocker(t)
	inUse, err := IsNetworkInUse("raioz-test-nonexistent-network-12345")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if inUse {
		t.Error("expected false for nonexistent network")
	}
}

func TestIsNetworkInUseWithContext_Empty(t *testing.T) {
	requireDocker(t)
	// Create a network, check it's not in use (0 containers), remove it
	name := "raioz-test-isuse-ctx"
	_ = RemoveNetwork(name)
	if err := CreateNetwork(name); err != nil {
		t.Fatalf("create: %v", err)
	}
	defer func() { _ = RemoveNetwork(name) }()

	inUse, err := IsNetworkInUseWithContext(context.Background(), name)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if inUse {
		t.Error("expected false for empty network")
	}
}

// --- CleanUnusedImages dry run ---

func TestCleanUnusedImages_DryRun(t *testing.T) {
	requireDocker(t)
	actions, err := CleanUnusedImages(true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// Should have at least one action (either "Would remove" or "No unused")
	if len(actions) == 0 {
		t.Error("expected actions for dry run")
	}
}

func TestCleanUnusedVolumes_DryRun(t *testing.T) {
	requireDocker(t)
	actions, err := CleanUnusedVolumes(true, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(actions) == 0 {
		t.Error("expected actions for dry run")
	}
}

func TestCleanUnusedVolumes_NoForce(t *testing.T) {
	_, err := CleanUnusedVolumes(false, false)
	if err == nil {
		t.Error("expected error when neither dryRun nor force")
	}
}

func TestCleanUnusedNetworks_DryRun(t *testing.T) {
	requireDocker(t)
	actions, err := CleanUnusedNetworks(true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(actions) == 0 {
		t.Error("expected actions for dry run")
	}
}

// --- DownWithContext: invalid path ---

func TestDownWithContext_InvalidPath(t *testing.T) {
	// A nonexistent path returns nil (early return)
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "missing.yml")
	if err := DownWithContext(context.Background(), missing); err != nil {
		t.Errorf("missing: %v", err)
	}
	// Existing file with dangerous char (simulated by making the file then
	// passing a bad path string is impossible; skip).
}

func TestDownWithContext_BadCharExistingFile(t *testing.T) {
	tmp := t.TempDir()
	badName := filepath.Join(tmp, "bad;rm.yml")
	if err := os.WriteFile(badName, []byte("x"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := DownWithContext(context.Background(), badName); err == nil {
		t.Error("expected validation error for dangerous chars")
	}
}

// --- GetServicesStatusWithContext: bad path ---

func TestGetServicesStatusWithContext_BadChar(t *testing.T) {
	tmp := t.TempDir()
	badName := filepath.Join(tmp, "bad;rm.yml")
	if err := os.WriteFile(badName, []byte("x"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := GetServicesStatusWithContext(context.Background(), badName)
	if err == nil {
		t.Error("expected validation error")
	}
}

// --- AreServicesRunning: compose exists but no services ---

func TestAreServicesRunning_BadChar(t *testing.T) {
	tmp := t.TempDir()
	badName := filepath.Join(tmp, "bad;rm.yml")
	if err := os.WriteFile(badName, []byte("x"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := AreServicesRunning(badName, []string{"api"})
	if err == nil {
		t.Error("expected error")
	}
}

// --- GetServiceNamesWithContext bad char ---

func TestGetServiceNamesWithContext_BadChar(t *testing.T) {
	tmp := t.TempDir()
	badName := filepath.Join(tmp, "bad;rm.yml")
	if err := os.WriteFile(badName, []byte("x"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := GetServiceNamesWithContext(context.Background(), badName)
	if err == nil {
		t.Error("expected error")
	}
}

// --- GetAvailableServicesWithContext bad char ---

func TestGetAvailableServicesWithContext_BadChar(t *testing.T) {
	tmp := t.TempDir()
	badName := filepath.Join(tmp, "bad;rm.yml")
	if err := os.WriteFile(badName, []byte("x"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := GetAvailableServicesWithContext(context.Background(), badName)
	if err == nil {
		t.Error("expected error")
	}
}

// --- ViewLogsWithContext paths (only validation) ---

func TestViewLogsWithContext_BadChar(t *testing.T) {
	tmp := t.TempDir()
	badName := filepath.Join(tmp, "bad;rm.yml")
	if err := os.WriteFile(badName, []byte("x"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	err := ViewLogsWithContext(context.Background(), badName, LogsOptions{Tail: 10})
	if err == nil {
		t.Error("expected error")
	}
}

// --- GetContainerNameWithContext bad char ---

func TestGetContainerNameWithContext_BadChar(t *testing.T) {
	tmp := t.TempDir()
	badName := filepath.Join(tmp, "bad;rm.yml")
	if err := os.WriteFile(badName, []byte("x"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := GetContainerNameWithContext(context.Background(), badName, "svc")
	if err == nil {
		t.Error("expected error")
	}
}

// --- ExecInService: invalid path ---

func TestExecInService_InvalidPath(t *testing.T) {
	err := ExecInService(
		context.Background(), "/tmp/bad;rm.yml", "svc",
		[]string{"ls"}, false,
	)
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

// --- RestartServicesWithContext: invalid path ---

func TestRestartServicesWithContext_InvalidPath(t *testing.T) {
	err := RestartServicesWithContext(
		context.Background(), "/tmp/bad;rm.yml", []string{"svc"},
	)
	if err == nil {
		t.Error("expected error")
	}
}

// --- ForceRecreateServicesWithContext: invalid path ---

func TestForceRecreateServicesWithContext_InvalidPath(t *testing.T) {
	err := ForceRecreateServicesWithContext(
		context.Background(), "/tmp/bad;rm.yml", []string{"svc"},
	)
	if err == nil {
		t.Error("expected error")
	}
}

// --- StopContainerWithContext: empty name ---

func TestStopContainerWithContext_Empty(t *testing.T) {
	if err := StopContainerWithContext(context.Background(), ""); err != nil {
		t.Errorf("empty: %v", err)
	}
}

func TestStopContainerWithContext_Nonexistent(t *testing.T) {
	requireDocker(t)
	// Stopping a nonexistent container should return nil (handled message)
	err := StopContainerWithContext(
		context.Background(), "raioz-test-nonexistent-container-xyz",
	)
	if err != nil {
		t.Errorf("nonexistent: %v", err)
	}
}

// --- FindAlternativePort: happy path ---

func TestFindAlternativePort(t *testing.T) {
	// High port is likely free; we just exercise the function flow.
	port, err := FindAlternativePort("65530", 5)
	if err == nil {
		if port == 0 {
			t.Error("expected non-zero port")
		}
	}
	// Invalid input
	if _, err := FindAlternativePort("bad", 3); err == nil {
		t.Error("expected error for bad port")
	}
}

// --- StopServiceWithContext: existing path, running docker ---

func TestStopServiceWithContext_NonexistentCompose(t *testing.T) {
	requireDocker(t)
	// File exists but service doesn't — exercises real docker call
	tmp := t.TempDir()
	compose := filepath.Join(tmp, "docker-compose.yml")
	content := `services:
  fake:
    image: alpine
`
	if err := os.WriteFile(compose, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// This will call docker compose stop which will fail since no container exists
	// We don't care about the error, just exercise the path
	_ = StopServiceWithContext(context.Background(), compose, "fake")
}
