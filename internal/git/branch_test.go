package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"raioz/internal/config"
)

func TestGetCurrentBranch(t *testing.T) {
	// Create a temporary git repo for testing
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Skip("git not available or failed to init repo")
		return
	}

	// Create a file and commit
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "test")
	cmd.Dir = tmpDir
	cmd.Run()

	// Checkout a branch
	cmd = exec.Command("git", "checkout", "-b", "test-branch")
	cmd.Dir = tmpDir
	cmd.Run()

	// Test GetCurrentBranch
	branch, err := GetCurrentBranch(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("GetCurrentBranch() error = %v", err)
	}

	if branch != "test-branch" {
		t.Errorf("GetCurrentBranch() = %v, want test-branch", branch)
	}
}

func TestDetectBranchDrift(t *testing.T) {
	// Create a temporary git repo for testing
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Skip("git not available or failed to init repo")
		return
	}

	// Create a file and commit
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "test")
	cmd.Dir = tmpDir
	cmd.Run()

	// Create branches
	cmd = exec.Command("git", "checkout", "-b", "main")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "checkout", "-b", "develop")
	cmd.Dir = tmpDir
	cmd.Run()

	// Test with drift
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
