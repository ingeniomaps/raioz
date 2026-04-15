package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"raioz/internal/config"
	exectimeout "raioz/internal/exec"
	"raioz/internal/logging"
	"raioz/internal/resilience"
)

// EnsureReadonlyRepo ensures a readonly repository exists without updating it
// This function only clones if the repo doesn't exist, and never updates it
func EnsureReadonlyRepo(src config.SourceConfig, baseDir string) error {
	target := filepath.Join(baseDir, src.Path)

	// Check if repo already exists
	if _, err := os.Stat(target); err == nil {
		// Repo exists, don't update it (readonly behavior)
		logging.Info("Repository already exists (readonly), skipping update", "repository", filepath.Base(target))
		return nil
	}

	// Repo doesn't exist, clone it
	logging.Info("Cloning readonly repository", "path", src.Path)

	// Validate inputs to prevent command injection
	if err := validateGitInput(src.Branch, src.Repo); err != nil {
		return fmt.Errorf("invalid git input: %w", err)
	}
	if err := validatePath(target); err != nil {
		return fmt.Errorf("invalid target path: %w", err)
	}

	// Create context with timeout
	ctx, cancel := exectimeout.WithTimeout(exectimeout.GitCloneTimeout)
	defer cancel()

	// Use circuit breaker and retry logic for git clone
	gitCB := resilience.GetGitCircuitBreaker()
	retryConfig := resilience.GitRetryConfig()

	err := resilience.RetryWithContext(ctx, retryConfig, "git clone readonly", func(ctx context.Context) error {
		return gitCB.ExecuteWithContext(ctx, "git clone readonly", func(ctx context.Context) error {
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			cmd := exec.CommandContext(
				ctx, "git", "-c", "credential.helper=",
				"clone", "--depth", "1", "-b",
				src.Branch, src.Repo, target,
			)
			// Disable interactive prompts for public repos
			cmd.Env = append(
				os.Environ(),
				"GIT_TERMINAL_PROMPT=0",
				"GIT_ASKPASS=", "GIT_SSH_COMMAND=",
			)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				// Check for timeout
				if exectimeout.IsTimeoutError(ctx, err) {
					return exectimeout.HandleTimeoutError(ctx, err, "git clone", exectimeout.GitCloneTimeout)
				}

				// Check if error is about branch not existing (not retryable)
				output := err.Error()
				if strings.Contains(output, "could not find remote branch") ||
					strings.Contains(output, "fatal: Remote branch") {
					return fmt.Errorf(
						"branch '%s' does not exist in repository '%s'. "+
							"Please verify the branch name or create it in the repository",
						src.Branch, src.Repo,
					)
				}
				return fmt.Errorf("failed to clone readonly repository: %w", err)
			}
			return nil
		})
	})

	if err != nil {
		return err
	}

	logging.Info("Readonly repository cloned (will not be updated automatically)", "repository", filepath.Base(target))
	return nil
}

// EnsureEditableRepo ensures an editable repository exists and is up to date
// This function clones if needed, checks out the correct branch, and pulls updates
func EnsureEditableRepo(src config.SourceConfig, baseDir string) error {
	target := filepath.Join(baseDir, src.Path)

	if _, err := os.Stat(target); err == nil {
		// Repo exists, check for branch drift (manual changes) first
		// Use a context with default timeout for branch operations
		ctx, cancel := exectimeout.WithTimeout(exectimeout.DefaultTimeout)
		defer cancel()

		drift, currentBranch, err := DetectBranchDrift(ctx, target, src.Branch)
		if err != nil {
			// Not a git repo or error, skip branch check
			return nil
		}

		if drift {
			logging.Warn("Branch drift detected, updating to expected branch",
				"repository", filepath.Base(target),
				"expected_branch", src.Branch,
				"current_branch", currentBranch)
		}

		// Validate branch exists in remote
		if err := ValidateBranch(ctx, target, src.Branch); err != nil {
			return fmt.Errorf("branch validation failed: %w", err)
		}

		// Ensure correct branch and pull
		if err := EnsureBranch(ctx, target, src.Branch); err != nil {
			return fmt.Errorf("failed to ensure branch '%s': %w", src.Branch, err)
		}

		return nil
	}

	// Repo doesn't exist, clone it

	// Validate inputs to prevent command injection
	if err := validateGitInput(src.Branch, src.Repo); err != nil {
		return fmt.Errorf("invalid git input: %w", err)
	}
	if err := validatePath(target); err != nil {
		return fmt.Errorf("invalid target path: %w", err)
	}

	// Create context with timeout
	ctx, cancel := exectimeout.WithTimeout(exectimeout.GitCloneTimeout)
	defer cancel()

	// Use circuit breaker and retry logic for git clone
	gitCB := resilience.GetGitCircuitBreaker()
	retryConfig := resilience.GitRetryConfig()

	err := resilience.RetryWithContext(ctx, retryConfig, "git clone editable", func(ctx context.Context) error {
		return gitCB.ExecuteWithContext(ctx, "git clone editable", func(ctx context.Context) error {
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			cmd := exec.CommandContext(
				ctx, "git", "-c", "credential.helper=",
				"clone", "--depth", "1", "-b",
				src.Branch, src.Repo, target,
			)
			// Disable interactive prompts for public repos
			cmd.Env = append(
				os.Environ(),
				"GIT_TERMINAL_PROMPT=0",
				"GIT_ASKPASS=", "GIT_SSH_COMMAND=",
			)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				// Check for timeout
				if exectimeout.IsTimeoutError(ctx, err) {
					return exectimeout.HandleTimeoutError(ctx, err, "git clone", exectimeout.GitCloneTimeout)
				}

				// Check if error is about branch not existing (not retryable)
				output := err.Error()
				if strings.Contains(output, "could not find remote branch") ||
					strings.Contains(output, "fatal: Remote branch") {
					return fmt.Errorf(
						"branch '%s' does not exist in repository '%s'. "+
							"Please verify the branch name or create it in the repository",
						src.Branch, src.Repo,
					)
				}
				return fmt.Errorf("failed to clone repository: %w", err)
			}
			return nil
		})
	})

	if err != nil {
		return err
	}

	// After cloning, validate branch exists (use same context)
	if err := ValidateBranch(ctx, target, src.Branch); err != nil {
		return fmt.Errorf("branch validation failed after clone: %w", err)
	}

	return nil
}

// IsReadonly checks if a source config is readonly
// Only applies to git sources - image sources are never readonly
func IsReadonly(src config.SourceConfig) bool {
	// Only applies to git sources
	if src.Kind != "git" {
		return false
	}
	// Default to editable if not specified
	if src.Access == "" {
		return false
	}
	return src.Access == "readonly"
}
