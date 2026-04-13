package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitRepoHelper centralizes the setup of real on-disk git repositories for
// tests. It creates repos without touching the network so tests run offline
// and deterministically. All commands are executed inside t.TempDir()
// subdirectories created per test.

// runGit runs a git command in dir and fails the test if it errors.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	// Use a deterministic environment so the host user's git config cannot
	// interfere (e.g. signing hooks, global hooksPath, pager).
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=raioz-test",
		"GIT_AUTHOR_EMAIL=test@raioz.local",
		"GIT_COMMITTER_NAME=raioz-test",
		"GIT_COMMITTER_EMAIL=test@raioz.local",
		"GIT_TERMINAL_PROMPT=0",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed in %s: %v\noutput: %s", args, dir, err, string(out))
	}
}

// initLocalRepo creates a new repo at dir with an initial commit on the
// requested branch. Returns dir for convenience.
func initLocalRepo(t *testing.T, dir, branch string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runGit(t, dir, "init", "-q", "-b", branch)
	runGit(t, dir, "config", "user.email", "test@raioz.local")
	runGit(t, dir, "config", "user.name", "raioz-test")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", "initial")
	return dir
}

// initLocalRepoWithRemote creates two repos: a "remote" (bare) repo and a
// working clone of it. The clone is what is returned; the remote path is
// available via the origin URL. Both repos use `branch` as the default.
func initLocalRepoWithRemote(t *testing.T, branch string) (workdir, remotePath string) {
	t.Helper()
	root := t.TempDir()
	remotePath = filepath.Join(root, "remote.git")
	workdir = filepath.Join(root, "work")

	// Create the work repo with an initial commit on the requested branch.
	initLocalRepo(t, workdir, branch)

	// Initialise a bare repo that will act as the remote.
	if err := os.MkdirAll(remotePath, 0755); err != nil {
		t.Fatalf("mkdir remote: %v", err)
	}
	runGit(t, remotePath, "init", "-q", "--bare", "-b", branch)

	// Wire up origin and push the initial commit.
	runGit(t, workdir, "remote", "add", "origin", remotePath)
	runGit(t, workdir, "push", "-q", "origin", branch)
	return workdir, remotePath
}

// skipIfNoGit skips the current test if the git binary is not installed.
func skipIfNoGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
}
