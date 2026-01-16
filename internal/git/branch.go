package git

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"raioz/internal/config"
	exectimeout "raioz/internal/exec"
	"raioz/internal/logging"
)

// GetCurrentBranch returns the current branch of a git repository
func GetCurrentBranch(ctx context.Context, repoPath string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		if exectimeout.IsTimeoutError(ctx, err) {
			return "", exectimeout.HandleTimeoutError(ctx, err, "git rev-parse", exectimeout.DefaultTimeout)
		}
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	branch := strings.TrimSpace(string(output))
	return branch, nil
}

// CheckoutBranch checks out a specific branch in a git repository
func CheckoutBranch(ctx context.Context, repoPath string, branch string) error {
	cmd := exec.CommandContext(ctx, "git", "checkout", branch)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exectimeout.IsTimeoutError(ctx, err) {
			return exectimeout.HandleTimeoutError(ctx, err, "git checkout", exectimeout.GitCheckoutTimeout)
		}
		return fmt.Errorf("failed to checkout branch '%s': %w (output: %s)", branch, err, string(output))
	}
	return nil
}

// PullBranch pulls the latest changes for the current branch
// Automatically stashes uncommitted changes before pulling and restores them after
func PullBranch(ctx context.Context, repoPath string) error {
	// Check for uncommitted changes first
	hasChanges, err := HasUncommittedChanges(ctx, repoPath)
	if err != nil {
		// If we can't check, continue anyway
		hasChanges = false
	}

	// If there are uncommitted changes, stash them automatically
	// This is safe because these are typically generated files (node_modules, etc.)
	// from Docker operations, not actual code changes
	if hasChanges {
		logging.Info("Stashing uncommitted changes before pull",
			"repository", filepath.Base(repoPath))

		// Stash changes (including untracked files)
		stashCmd := exec.CommandContext(ctx, "git", "stash", "push", "-u", "-m", "raioz: auto-stash before pull")
		stashCmd.Dir = repoPath
		stashOutput, stashErr := stashCmd.CombinedOutput()
		if stashErr != nil {
			// If stash fails, log warning but continue (might be empty stash)
			logging.Warn("Failed to stash changes, continuing anyway",
				"repository", filepath.Base(repoPath),
				"error", stashErr.Error(),
				"output", string(stashOutput))
		} else {
			// Restore stash after pull
			defer func() {
				popCmd := exec.CommandContext(ctx, "git", "stash", "pop")
				popCmd.Dir = repoPath
				popOutput, popErr := popCmd.CombinedOutput()
				if popErr != nil {
					// If pop fails, log warning (might be conflicts or empty stash)
					logging.Warn("Failed to restore stashed changes",
						"repository", filepath.Base(repoPath),
						"error", popErr.Error(),
						"output", string(popOutput))
				} else {
					logging.Info("Restored stashed changes after pull",
						"repository", filepath.Base(repoPath))
				}
			}()
		}
	}

	cmd := exec.CommandContext(ctx, "git", "pull")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exectimeout.IsTimeoutError(ctx, err) {
			return exectimeout.HandleTimeoutError(ctx, err, "git pull", exectimeout.GitPullTimeout)
		}
		// Check if it's a merge conflict
		outputStr := string(output)
		if strings.Contains(outputStr, "CONFLICT") || strings.Contains(outputStr, "conflict") {
			return fmt.Errorf(
				"merge conflict detected during pull. "+
					"Please resolve conflicts manually or use --force-reclone option",
			)
		}
		return fmt.Errorf("failed to pull changes: %w (output: %s)", err, outputStr)
	}

	// Check for merge conflicts after pull
	hasConflicts, err := HasMergeConflicts(ctx, repoPath)
	if err == nil && hasConflicts {
		return fmt.Errorf(
			"merge conflicts detected after pull. "+
				"Please resolve conflicts manually or use --force-reclone option",
		)
	}

	return nil
}

// DetectBranchDrift detects if the current branch differs from expected
func DetectBranchDrift(ctx context.Context, repoPath string, expectedBranch string) (bool, string, error) {
	currentBranch, err := GetCurrentBranch(ctx, repoPath)
	if err != nil {
		return false, "", err
	}

	if currentBranch != expectedBranch {
		return true, currentBranch, nil
	}

	return false, currentBranch, nil
}

// EnsureBranch ensures that a repository is on the correct branch
func EnsureBranch(ctx context.Context, repoPath string, expectedBranch string) error {
	// Validate branch exists in remote
	if err := ValidateBranch(ctx, repoPath, expectedBranch); err != nil {
		return fmt.Errorf("branch validation failed: %w", err)
	}

	// Check current branch
	currentBranch, err := GetCurrentBranch(ctx, repoPath)
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	if currentBranch == expectedBranch {
		// Already on correct branch, just pull
		return PullBranch(ctx, repoPath)
	}

	// Branch changed, checkout and pull
	logging.Warn("Branch changed, updating to expected branch",
		"repository", filepath.Base(repoPath),
		"current_branch", currentBranch,
		"expected_branch", expectedBranch)

	if err := CheckoutBranch(ctx, repoPath, expectedBranch); err != nil {
		return fmt.Errorf("failed to checkout branch '%s': %w", expectedBranch, err)
	}

	if err := PullBranch(ctx, repoPath); err != nil {
		return fmt.Errorf("failed to pull branch '%s': %w", expectedBranch, err)
	}

	return nil
}

// UpdateReposIfBranchChanged updates repositories if their branch changed
// repoPathResolver is a function that returns the correct path for a service based on access mode
func UpdateReposIfBranchChanged(
	ctx context.Context,
	repoPathResolver func(string, config.Service) string,
	oldDeps *config.Deps,
	newDeps *config.Deps,
) error {
	if oldDeps == nil {
		// No previous state, nothing to update
		return nil
	}

	// Check each service for branch changes
	for name, newSvc := range newDeps.Services {
		if newSvc.Source.Kind != "git" {
			continue
		}

		oldSvc, exists := oldDeps.Services[name]
		if !exists {
			continue // New service, will be handled by EnsureRepo
		}

		// Check if branch changed
		if oldSvc.Source.Branch != newSvc.Source.Branch {
			// Use resolver to get correct path based on access mode
			repoPath := repoPathResolver(name, newSvc)

			// Verify repo exists before trying to update
			currentBranch, err := GetCurrentBranch(ctx, repoPath)
			if err != nil {
				// Repo might not exist yet, skip
				continue
			}

			if currentBranch != newSvc.Source.Branch {
				if err := EnsureBranch(ctx, repoPath, newSvc.Source.Branch); err != nil {
					return fmt.Errorf("failed to update branch for service %s: %w", name, err)
				}
			}
		}
	}

	return nil
}
