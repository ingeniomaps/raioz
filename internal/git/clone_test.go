package git

import (
	"os/exec"
	"strings"
	"testing"

	"raioz/internal/config"
)

// isGitAvailable reports whether the `git` binary is installed on the host.
func isGitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func TestEnsureRepoWithForce_ReadonlyRejected(t *testing.T) {
	// Force re-clone on a readonly git repo must be rejected immediately,
	// before any filesystem/network operations are attempted.
	src := config.SourceConfig{
		Kind:   "git",
		Repo:   "https://github.com/example/repo.git",
		Branch: "main",
		Path:   "svc",
		Access: "readonly",
	}

	err := EnsureRepoWithForce(src, t.TempDir(), true)
	if err == nil {
		t.Fatal("EnsureRepoWithForce(force=true, readonly) should return error")
	}
	if !strings.Contains(err.Error(), "readonly") {
		t.Errorf("error should mention readonly, got: %v", err)
	}
}

func TestEnsureRepoWithForce_NonGitSourceNotReadonly(t *testing.T) {
	// Non-git sources are never readonly, so force path should not be blocked
	// by the readonly guard. It will fail later (invalid repo URL), but not
	// because of the readonly check.
	src := config.SourceConfig{
		Kind:   "image",
		Access: "readonly",
		Path:   "svc",
	}

	// We pass force=false so it goes through EnsureRepo -> EnsureEditableRepo.
	// For this test we only care that the readonly branch is not taken.
	err := EnsureRepoWithForce(src, t.TempDir(), true)
	if err != nil && strings.Contains(err.Error(), "readonly") {
		t.Errorf("non-git source should not trigger readonly guard, got: %v", err)
	}
}

func TestEnsureRepo_Dispatch(t *testing.T) {
	// This test verifies dispatch by EnsureRepo: readonly -> EnsureReadonlyRepo,
	// editable -> EnsureEditableRepo. Both paths will fail at clone time (we
	// point at an invalid local URL) but they take different code paths.
	tests := []struct {
		name   string
		source config.SourceConfig
	}{
		{
			name: "readonly git",
			source: config.SourceConfig{
				Kind:   "git",
				Repo:   "file:///nonexistent/repo",
				Branch: "main",
				Path:   "svc-ro",
				Access: "readonly",
			},
		},
		{
			name: "editable git",
			source: config.SourceConfig{
				Kind:   "git",
				Repo:   "file:///nonexistent/repo",
				Branch: "main",
				Path:   "svc-rw",
				Access: "editable",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !isGitAvailable() {
				t.Skip("git not available")
			}
			// Will fail (no such repo) but must not panic and must return
			// an error, proving the dispatch path is exercised.
			err := EnsureRepo(tt.source, t.TempDir())
			if err == nil {
				t.Errorf("EnsureRepo should fail when cloning nonexistent repo")
			}
		})
	}
}

func TestEnsureRepoWithForce_NoForceCallsEnsureRepo(t *testing.T) {
	// When force=false, EnsureRepoWithForce should delegate to EnsureRepo.
	if !isGitAvailable() {
		t.Skip("git not available")
	}
	src := config.SourceConfig{
		Kind:   "git",
		Repo:   "file:///nonexistent/repo",
		Branch: "main",
		Path:   "svc",
		Access: "editable",
	}
	err := EnsureRepoWithForce(src, t.TempDir(), false)
	if err == nil {
		t.Error("EnsureRepoWithForce(force=false) should fail for nonexistent repo")
	}
}

func TestEnsureRepoWithForce_InvalidInput(t *testing.T) {
	// ForceReclone is invoked for editable repos with force=true. It validates
	// inputs, so malformed branches/repos must fail fast with a validation error.
	tests := []struct {
		name string
		src  config.SourceConfig
	}{
		{
			name: "invalid branch",
			src: config.SourceConfig{
				Kind:   "git",
				Repo:   "https://github.com/example/repo.git",
				Branch: "main; rm -rf /",
				Path:   "svc",
				Access: "editable",
			},
		},
		{
			name: "invalid repo",
			src: config.SourceConfig{
				Kind:   "git",
				Repo:   "https://github.com/example/repo.git; rm -rf",
				Branch: "main",
				Path:   "svc",
				Access: "editable",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EnsureRepoWithForce(tt.src, t.TempDir(), true)
			if err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}
