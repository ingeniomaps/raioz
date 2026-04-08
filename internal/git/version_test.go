package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGetCommitSHA(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "raioz-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize a git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Skipf("Skipping test: git not available: %v", err)
	}

	// Create a test file and commit
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Configure git user (required for commit)
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

	// Add and commit
	exec.Command("git", "add", "test.txt").Run()
	cmd2 := exec.Command("git", "commit", "-m", "test commit")
	cmd2.Dir = tmpDir
	if err := cmd2.Run(); err != nil {
		t.Skipf("Skipping test: failed to create commit: %v", err)
	}

	// Test GetCommitSHA
	sha, err := GetCommitSHA(context.Background(), tmpDir)
	if err != nil {
		t.Errorf("GetCommitSHA() error = %v", err)
	}

	if sha == "" {
		t.Error("GetCommitSHA() returned empty string")
	}

	if len(sha) > 12 {
		t.Errorf("GetCommitSHA() returned SHA longer than 12 chars: %s", sha)
	}
}

func TestGetCommitSHA_NotGitRepo(t *testing.T) {
	// Create a temporary directory without git
	tmpDir, err := os.MkdirTemp("", "raioz-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test with non-git directory
	_, err = GetCommitSHA(context.Background(), tmpDir)
	if err == nil {
		t.Error("GetCommitSHA() should return error for non-git directory")
	}
}

func TestGetCommitDate(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "raioz-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize a git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Skipf("Skipping test: git not available: %v", err)
	}

	// Create a test file and commit
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Configure git user
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

	// Add and commit
	exec.Command("git", "add", "test.txt").Run()
	cmd2 := exec.Command("git", "commit", "-m", "test commit")
	cmd2.Dir = tmpDir
	if err := cmd2.Run(); err != nil {
		t.Skipf("Skipping test: failed to create commit: %v", err)
	}

	// Test GetCommitDate
	date, err := GetCommitDate(context.Background(), tmpDir)
	if err != nil {
		t.Errorf("GetCommitDate() error = %v", err)
	}

	if date == "" {
		t.Error("GetCommitDate() returned empty string")
	}
}

func TestGetCommitDate_NotGitRepo(t *testing.T) {
	// Create a temporary directory without git
	tmpDir, err := os.MkdirTemp("", "raioz-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test with non-git directory
	_, err = GetCommitDate(context.Background(), tmpDir)
	if err == nil {
		t.Error("GetCommitDate() should return error for non-git directory")
	}
}
