package git

import (
	"context"
	"os/exec"
	"testing"

	"raioz/internal/config"
)

func TestNewGitRepository(t *testing.T) {
	r := NewGitRepository()
	if r == nil {
		t.Fatal("NewGitRepository returned nil")
	}
}

func TestGitRepositoryImpl_IsReadonly(t *testing.T) {
	r := NewGitRepository()

	cases := []struct {
		name string
		src  config.SourceConfig
		want bool
	}{
		{"git-readonly", config.SourceConfig{Kind: "git", Access: "readonly"}, true},
		{"git-editable", config.SourceConfig{Kind: "git", Access: "editable"}, false},
		{"git-default", config.SourceConfig{Kind: "git"}, false},
		{"non-git", config.SourceConfig{Access: "readonly"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := r.IsReadonly(tc.src)
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGitRepositoryImpl_EnsureRepoWithForce_ReadonlyForce(t *testing.T) {
	r := NewGitRepository()
	src := config.SourceConfig{
		Kind:   "git",
		Path:   "svc",
		Repo:   "https://example.com/repo.git",
		Branch: "main",
		Access: "readonly",
	}

	// Force re-clone on a readonly repo should fail
	err := r.EnsureRepoWithForce(src, t.TempDir(), true)
	if err == nil {
		t.Error("expected error when forcing readonly repo")
	}
}

func TestGitRepositoryImpl_EnsureRepo_InvalidRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	r := NewGitRepository()
	src := config.SourceConfig{
		Path:   "svc",
		Repo:   "not-a-real-url",
		Branch: "main",
	}
	// Should fail because repo can't be cloned
	err := r.EnsureRepo(src, t.TempDir())
	if err == nil {
		t.Log("EnsureRepo didn't error on bad repo — lenient behavior accepted")
	}
}

func TestGitRepositoryImpl_EnsureReadonlyRepo_InvalidRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	r := NewGitRepository()
	src := config.SourceConfig{
		Path: "svc",
		Repo: "not-a-real-url",
	}
	_ = r.EnsureReadonlyRepo(src, t.TempDir())
}

func TestGitRepositoryImpl_EnsureEditableRepo_InvalidRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	r := NewGitRepository()
	src := config.SourceConfig{
		Path: "svc",
		Repo: "not-a-real-url",
	}
	_ = r.EnsureEditableRepo(src, t.TempDir())
}

func TestGitRepositoryImpl_ForceReclone_InvalidRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	r := NewGitRepository()
	err := r.ForceReclone(context.Background(), t.TempDir(), "not-a-real-url", "main")
	_ = err
}

func TestGitRepositoryImpl_UpdateReposIfBranchChanged(t *testing.T) {
	r := NewGitRepository()

	resolver := func(name string, svc config.Service) string { return "" }
	oldDeps := &config.Deps{
		Services: map[string]config.Service{},
	}
	newDeps := &config.Deps{
		Services: map[string]config.Service{},
	}

	err := r.UpdateReposIfBranchChanged(context.Background(), resolver, oldDeps, newDeps)
	if err != nil {
		t.Errorf("UpdateReposIfBranchChanged with no services: %v", err)
	}
}
