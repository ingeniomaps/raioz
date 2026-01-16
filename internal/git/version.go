package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	exectimeout "raioz/internal/exec"
)

// GetCommitSHA returns the current commit SHA of a repository
func GetCommitSHA(ctx context.Context, repoPath string) (string, error) {
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); os.IsNotExist(err) {
		return "", fmt.Errorf("not a git repository: %s", repoPath)
	}

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		if exectimeout.IsTimeoutError(ctx, err) {
			return "", exectimeout.HandleTimeoutError(ctx, err, "git rev-parse", exectimeout.DefaultTimeout)
		}
		return "", fmt.Errorf("failed to get commit SHA: %w", err)
	}

	sha := strings.TrimSpace(string(output))
	// Return first 12 characters (short SHA)
	if len(sha) > 12 {
		return sha[:12], nil
	}
	return sha, nil
}

// GetCommitDate returns the commit date of the current HEAD
func GetCommitDate(ctx context.Context, repoPath string) (string, error) {
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); os.IsNotExist(err) {
		return "", fmt.Errorf("not a git repository: %s", repoPath)
	}

	cmd := exec.CommandContext(ctx, "git", "log", "-1", "--format=%ci", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		if exectimeout.IsTimeoutError(ctx, err) {
			return "", exectimeout.HandleTimeoutError(ctx, err, "git log", exectimeout.DefaultTimeout)
		}
		return "", fmt.Errorf("failed to get commit date: %w", err)
	}

	date := strings.TrimSpace(string(output))
	return date, nil
}
