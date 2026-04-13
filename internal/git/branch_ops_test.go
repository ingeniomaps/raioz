package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckoutBranch(t *testing.T) {
	skipIfNoGit(t)
	dir := initLocalRepo(t, filepath.Join(t.TempDir(), "repo"), "main")

	// Create a second branch on top of HEAD.
	runGit(t, dir, "branch", "feature")

	// Checkout to feature, then back to main.
	if err := CheckoutBranch(context.Background(), dir, "feature"); err != nil {
		t.Fatalf("CheckoutBranch(feature) error = %v", err)
	}
	cur, err := GetCurrentBranch(context.Background(), dir)
	if err != nil {
		t.Fatalf("GetCurrentBranch error = %v", err)
	}
	if cur != "feature" {
		t.Errorf("current branch = %q, want feature", cur)
	}

	if err := CheckoutBranch(context.Background(), dir, "main"); err != nil {
		t.Fatalf("CheckoutBranch(main) error = %v", err)
	}
}

func TestCheckoutBranch_Missing(t *testing.T) {
	skipIfNoGit(t)
	dir := initLocalRepo(t, filepath.Join(t.TempDir(), "repo"), "main")

	// Checking out a branch that does not exist should error.
	err := CheckoutBranch(context.Background(), dir, "does-not-exist")
	if err == nil {
		t.Error("CheckoutBranch(missing) should return an error")
	}
}

func TestPullBranch_NoChanges(t *testing.T) {
	skipIfNoGit(t)
	workdir, _ := initLocalRepoWithRemote(t, "main")

	// Setup upstream tracking so `git pull` without args works.
	runGit(t, workdir, "branch", "--set-upstream-to=origin/main", "main")

	if err := PullBranch(context.Background(), workdir); err != nil {
		t.Errorf("PullBranch() error = %v", err)
	}
}

func TestPullBranch_WithUncommittedChanges(t *testing.T) {
	skipIfNoGit(t)
	workdir, _ := initLocalRepoWithRemote(t, "main")
	runGit(t, workdir, "branch", "--set-upstream-to=origin/main", "main")

	// Introduce uncommitted changes to exercise the auto-stash path.
	if err := os.WriteFile(filepath.Join(workdir, "README.md"), []byte("# modified\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Also create an untracked file.
	if err := os.WriteFile(filepath.Join(workdir, "new.txt"), []byte("new\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := PullBranch(context.Background(), workdir); err != nil {
		t.Errorf("PullBranch() with uncommitted changes error = %v", err)
	}
}

func TestPullBranch_NotARepo(t *testing.T) {
	skipIfNoGit(t)
	// Pulling in a non-git directory must error.
	err := PullBranch(context.Background(), t.TempDir())
	if err == nil {
		t.Error("PullBranch() should error for non-git directory")
	}
}

func TestEnsureBranch_AlreadyOnExpected(t *testing.T) {
	skipIfNoGit(t)
	workdir, _ := initLocalRepoWithRemote(t, "main")
	runGit(t, workdir, "branch", "--set-upstream-to=origin/main", "main")

	if err := EnsureBranch(context.Background(), workdir, "main"); err != nil {
		t.Errorf("EnsureBranch(main) error = %v", err)
	}
}

func TestEnsureBranch_Switch(t *testing.T) {
	skipIfNoGit(t)
	workdir, _ := initLocalRepoWithRemote(t, "main")

	// Push a second branch to the remote so ValidateBranch finds it there,
	// and set its upstream so PullBranch succeeds after the checkout.
	runGit(t, workdir, "checkout", "-q", "-b", "feature")
	runGit(t, workdir, "push", "-q", "-u", "origin", "feature")
	// Return to main and set upstream.
	runGit(t, workdir, "checkout", "-q", "main")
	runGit(t, workdir, "branch", "--set-upstream-to=origin/main", "main")

	// Switch to feature.
	if err := EnsureBranch(context.Background(), workdir, "feature"); err != nil {
		t.Errorf("EnsureBranch(feature) error = %v", err)
	}
	cur, _ := GetCurrentBranch(context.Background(), workdir)
	if cur != "feature" {
		t.Errorf("current branch = %q, want feature", cur)
	}
}

func TestEnsureBranch_MissingBranch(t *testing.T) {
	skipIfNoGit(t)
	workdir, _ := initLocalRepoWithRemote(t, "main")

	// Requesting a branch that does not exist in the remote must error
	// (via ValidateBranch).
	err := EnsureBranch(context.Background(), workdir, "nope")
	if err == nil {
		t.Error("EnsureBranch(nope) should return an error")
	}
}
