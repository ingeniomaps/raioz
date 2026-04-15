package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
)

func TestGetCurrentBranch(t *testing.T) {
	tmpDir := t.TempDir()

	runGit(t, tmpDir, "init")
	if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "test")
	runGit(t, tmpDir, "checkout", "-b", "test-branch")

	branch, err := GetCurrentBranch(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("GetCurrentBranch() error = %v", err)
	}

	if branch != "test-branch" {
		t.Errorf("GetCurrentBranch() = %v, want test-branch", branch)
	}
}

func TestDetectBranchDrift(t *testing.T) {
	tmpDir := t.TempDir()

	runGit(t, tmpDir, "init")
	if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "test")
	runGit(t, tmpDir, "checkout", "-b", "main")
	runGit(t, tmpDir, "checkout", "-b", "develop")

	drift, current, err := DetectBranchDrift(context.Background(), tmpDir, "main")
	if err != nil {
		t.Fatalf("DetectBranchDrift() error = %v", err)
	}

	if !drift {
		t.Error("DetectBranchDrift() should detect drift")
	}

	if current != "develop" {
		t.Errorf("DetectBranchDrift() current = %v, want develop", current)
	}

	// Test without drift
	drift, current, err = DetectBranchDrift(context.Background(), tmpDir, "develop")
	if err != nil {
		t.Fatalf("DetectBranchDrift() error = %v", err)
	}

	if drift {
		t.Error("DetectBranchDrift() should not detect drift")
	}

	if current != "develop" {
		t.Errorf("DetectBranchDrift() current = %v, want develop", current)
	}
}

func TestUpdateReposIfBranchChanged(t *testing.T) {
	tmpDir := t.TempDir()
	servicesDir := filepath.Join(tmpDir, "services")

	// Simple resolver that uses old structure for test
	repoPathResolver := func(svc config.Service) string {
		return filepath.Join(servicesDir, svc.Source.Path)
	}

	oldDeps := &config.Deps{
		Services: map[string]config.Service{
			"service1": {
				Source: config.SourceConfig{
					Kind:   "git",
					Branch: "main",
					Path:   "test-service",
				},
			},
		},
	}

	newDeps := &config.Deps{
		Services: map[string]config.Service{
			"service1": {
				Source: config.SourceConfig{
					Kind:   "git",
					Branch: "develop",
					Path:   "test-service",
				},
			},
		},
	}

	// Test with no existing repos (should not error)
	err := UpdateReposIfBranchChanged(
		context.Background(),
		func(_ string, svc config.Service) string { // adapt signature to match UpdateReposIfBranchChanged
			return repoPathResolver(svc)
		},
		oldDeps, newDeps)
	if err != nil {
		t.Errorf("UpdateReposIfBranchChanged() error = %v, want nil", err)
	}
}
