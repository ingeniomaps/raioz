package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/config"
)

// setupLocalRemote creates a bare repository with one commit on `branch` and
// returns a file:// URL that can be used as SourceConfig.Repo. It uses an
// intermediate work tree to produce the initial commit, then pushes to the
// bare repo. Shallow cloning (`git clone --depth 1`) over file:// requires
// the remote to have at least one commit, which this helper guarantees.
func setupLocalRemote(t *testing.T, branch string) string {
	t.Helper()
	_, remotePath := initLocalRepoWithRemote(t, branch)
	return "file://" + remotePath
}

func TestEnsureReadonlyRepo_Clone(t *testing.T) {
	skipIfNoGit(t)
	url := setupLocalRemote(t, "main")
	baseDir := t.TempDir()

	src := config.SourceConfig{
		Kind:   "git",
		Repo:   url,
		Branch: "main",
		Path:   "svc",
		Access: "readonly",
	}
	if err := EnsureReadonlyRepo(src, baseDir); err != nil {
		t.Fatalf("EnsureReadonlyRepo error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "svc", ".git")); err != nil {
		t.Errorf("expected cloned repo to exist: %v", err)
	}
}

func TestEnsureReadonlyRepo_AlreadyExistsSkipsUpdate(t *testing.T) {
	skipIfNoGit(t)
	baseDir := t.TempDir()
	// Pre-create the target so EnsureReadonlyRepo takes the short-circuit.
	target := filepath.Join(baseDir, "svc")
	initLocalRepo(t, target, "main")

	src := config.SourceConfig{
		Kind:   "git",
		Repo:   "https://github.com/never/called.git",
		Branch: "main",
		Path:   "svc",
		Access: "readonly",
	}
	// Should NOT attempt to clone/fetch because the path exists already.
	if err := EnsureReadonlyRepo(src, baseDir); err != nil {
		t.Errorf("EnsureReadonlyRepo error = %v", err)
	}
}

func TestEnsureReadonlyRepo_InvalidInputs(t *testing.T) {
	skipIfNoGit(t)
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
				Access: "readonly",
			},
		},
		{
			name: "invalid repo",
			src: config.SourceConfig{
				Kind:   "git",
				Repo:   "https://github.com/example/repo.git | cat",
				Branch: "main",
				Path:   "svc",
				Access: "readonly",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EnsureReadonlyRepo(tt.src, t.TempDir())
			if err == nil {
				t.Error("expected validation error, got nil")
			}
			if !strings.Contains(err.Error(), "invalid") {
				t.Errorf("expected 'invalid' in error, got: %v", err)
			}
		})
	}
}

func TestEnsureEditableRepo_Clone(t *testing.T) {
	skipIfNoGit(t)
	url := setupLocalRemote(t, "main")
	baseDir := t.TempDir()

	src := config.SourceConfig{
		Kind:   "git",
		Repo:   url,
		Branch: "main",
		Path:   "svc",
		Access: "editable",
	}
	if err := EnsureEditableRepo(src, baseDir); err != nil {
		t.Fatalf("EnsureEditableRepo error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "svc", ".git")); err != nil {
		t.Errorf("expected cloned repo to exist: %v", err)
	}
}

func TestEnsureEditableRepo_InvalidInputs(t *testing.T) {
	skipIfNoGit(t)
	src := config.SourceConfig{
		Kind:   "git",
		Repo:   "https://github.com/example/repo.git",
		Branch: "main; rm -rf",
		Path:   "svc",
		Access: "editable",
	}
	err := EnsureEditableRepo(src, t.TempDir())
	if err == nil {
		t.Error("expected validation error, got nil")
	}
}
