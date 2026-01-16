package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	exectimeout "raioz/internal/exec"
)

// BranchExists checks if a branch exists in the remote repository
func BranchExists(ctx context.Context, repoPath string, branch string) (bool, error) {
	// First, fetch to get latest remote branches (short timeout, best effort)
	fetchCtx, fetchCancel := exectimeout.WithTimeoutFromContext(ctx, 30*time.Second)
	defer fetchCancel()
	fetchCmd := exec.CommandContext(fetchCtx, "git", "fetch", "origin")
	fetchCmd.Dir = repoPath
	fetchCmd.Stderr = nil // Suppress fetch output
	_ = fetchCmd.Run()    // Ignore fetch errors, continue anyway

	// Check if branch exists in remote
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", "origin", branch)
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		if exectimeout.IsTimeoutError(ctx, err) {
			return false, exectimeout.HandleTimeoutError(ctx, err, "git ls-remote", exectimeout.DefaultTimeout)
		}
		return false, fmt.Errorf("failed to check remote branch: %w", err)
	}

	return strings.TrimSpace(string(output)) != "", nil
}

// ValidateBranch validates that a branch exists in the remote repository
func ValidateBranch(ctx context.Context, repoPath string, branch string) error {
	// Check if repo exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		// For new repos, we can't validate yet
		return nil
	}

	exists, err := BranchExists(ctx, repoPath, branch)
	if err != nil {
		return fmt.Errorf("failed to validate branch '%s': %w", branch, err)
	}

	if !exists {
		return fmt.Errorf(
			"branch '%s' does not exist in remote repository. "+
				"Please verify the branch name or create it in the repository",
			branch,
		)
	}

	return nil
}

// HasUncommittedChanges checks if repository has uncommitted changes
func HasUncommittedChanges(ctx context.Context, repoPath string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		if exectimeout.IsTimeoutError(ctx, err) {
			return false, exectimeout.HandleTimeoutError(ctx, err, "git status", exectimeout.DefaultTimeout)
		}
		return false, fmt.Errorf("failed to check git status: %w", err)
	}

	return strings.TrimSpace(string(output)) != "", nil
}

// HasMergeConflicts checks if there are merge conflicts after a pull
func HasMergeConflicts(ctx context.Context, repoPath string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--check")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		if exectimeout.IsTimeoutError(ctx, err) {
			return false, exectimeout.HandleTimeoutError(ctx, err, "git diff", exectimeout.DefaultTimeout)
		}
		// If error, assume no conflicts (conservative approach)
		return false, nil
	}

	return strings.Contains(string(output), "<<<<<<<"), nil
}

// ForceReclone removes the repository directory and clones it fresh
func ForceReclone(ctx context.Context, repoPath string, repo string, branch string) error {
	// Validate inputs to prevent command injection
	if err := validateGitInput(branch, repo); err != nil {
		return fmt.Errorf("invalid git input: %w", err)
	}
	if err := validatePath(repoPath); err != nil {
		return fmt.Errorf("invalid repo path: %w", err)
	}

	// Remove existing directory
	if err := os.RemoveAll(repoPath); err != nil {
		return fmt.Errorf("failed to remove existing repository: %w", err)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(repoPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Clone fresh with timeout (shallow clone to save space)
	cmd := exec.CommandContext(ctx, "git", "-c", "credential.helper=", "clone", "--depth", "1", "-b", branch, repo, repoPath)
	// Disable interactive prompts and credential helpers for public repos
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "GIT_SSH_COMMAND=")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exectimeout.IsTimeoutError(ctx, err) {
			return exectimeout.HandleTimeoutError(ctx, err, "git clone", exectimeout.GitCloneTimeout)
		}
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	return nil
}
