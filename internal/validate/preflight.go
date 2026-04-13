package validate

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"raioz/internal/errors"
	exectimeout "raioz/internal/exec"
	"raioz/internal/runtime"
)

// PreflightCheck performs all preflight checks before executing commands
func PreflightCheck() error {
	return PreflightCheckWithContext(context.Background())
}

// PreflightCheckWithContext performs all preflight checks before executing commands with context support
func PreflightCheckWithContext(ctx context.Context) error {
	var checkErrors []error

	// Check Docker installation
	if err := checkDockerInstalledWithContext(ctx); err != nil {
		checkErrors = append(checkErrors, err)
	}

	// Check Docker daemon
	if err := checkDockerRunningWithContext(ctx); err != nil {
		checkErrors = append(checkErrors, err)
	}

	// Check Git installation (for git-based services)
	if err := checkGitInstalledWithContext(ctx); err != nil {
		checkErrors = append(checkErrors, err)
	}

	// Check disk space
	if err := checkDiskSpace(); err != nil {
		checkErrors = append(checkErrors, err)
	}

	// Check network connectivity (basic check)
	if err := checkNetworkConnectivityWithContext(ctx); err != nil {
		checkErrors = append(checkErrors, err)
	}

	if len(checkErrors) > 0 {
		return fmt.Errorf(
			"preflight checks failed:\n%s",
			errors.FormatMultipleErrors(checkErrors),
		)
	}

	return nil
}

// checkDockerInstalled verifies that Docker is installed
// Also exported as CheckDockerInstalled for CI
func checkDockerInstalled() error {
	return checkDockerInstalledWithContext(context.Background())
}

// checkDockerInstalledWithContext verifies that Docker is installed with context support
func checkDockerInstalledWithContext(ctx context.Context) error {
	// Create context with short timeout (version check should be fast)
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), "--version")
	if err := cmd.Run(); err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return errors.New(
				errors.ErrCodeDockerNotInstalled,
				"Docker version check timed out",
			).WithSuggestion(
				"Ensure Docker is installed and accessible. " +
					"Install Docker from https://docs.docker.com/get-docker/",
			)
		}
		return errors.New(
			errors.ErrCodeDockerNotInstalled,
			"Docker is not installed or not in PATH",
		).WithSuggestion(
			"Install Docker from https://docs.docker.com/get-docker/ "+
				"or ensure Docker is in your PATH",
		).WithContext("command", "docker --version")
	}
	return nil
}

// checkDockerRunning verifies that Docker daemon is running
// Also exported as CheckDockerRunning for CI
func checkDockerRunning() error {
	return checkDockerRunningWithContext(context.Background())
}

// checkDockerRunningWithContext verifies that Docker daemon is running with context support
func checkDockerRunningWithContext(ctx context.Context) error {
	// Create context with timeout
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, exectimeout.DockerInspectTimeout)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, runtime.Binary(), "info")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return errors.New(
				errors.ErrCodeDockerNotRunning,
				"Docker daemon check timed out",
			).WithSuggestion(
				"Start Docker daemon: " +
					"• Linux: sudo systemctl start docker\n" +
					"• macOS: Open Docker Desktop\n" +
					"• Windows: Start Docker Desktop",
			)
		}
		return errors.New(
			errors.ErrCodeDockerNotRunning,
			"Docker daemon is not running",
		).WithSuggestion(
			"Start Docker daemon: "+
				"• Linux: sudo systemctl start docker\n"+
				"• macOS: Open Docker Desktop\n"+
				"• Windows: Start Docker Desktop",
		).WithContext("output", string(output)).WithError(err)
	}
	return nil
}

// checkGitInstalled verifies that Git is installed
func checkGitInstalled() error {
	return checkGitInstalledWithContext(context.Background())
}

// checkGitInstalledWithContext verifies that Git is installed with context support
func checkGitInstalledWithContext(ctx context.Context) error {
	// Create context with short timeout (version check should be fast)
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, "git", "--version")
	if err := cmd.Run(); err != nil {
		if exectimeout.IsTimeoutError(timeoutCtx, err) {
			return errors.New(
				errors.ErrCodeGitNotInstalled,
				"Git version check timed out",
			).WithSuggestion(
				"Ensure Git is installed and accessible. " +
					"Install Git from https://git-scm.com/downloads",
			)
		}
		return errors.New(
			errors.ErrCodeGitNotInstalled,
			"Git is not installed or not in PATH",
		).WithSuggestion(
			"Install Git from https://git-scm.com/downloads "+
				"or ensure Git is in your PATH",
		).WithContext("command", "git --version")
	}
	return nil
}

// checkDiskSpace checks if there's sufficient disk space
func checkDiskSpace() error {
	var stat syscall.Statfs_t

	// Check current directory
	wd, err := os.Getwd()
	if err != nil {
		return nil // Skip check if can't get working directory
	}

	if err := syscall.Statfs(wd, &stat); err != nil {
		return nil // Skip check if can't stat filesystem
	}

	// Calculate available space (in bytes)
	availableBytes := stat.Bavail * uint64(stat.Bsize)
	availableGB := float64(availableBytes) / (1024 * 1024 * 1024)

	// Warn if less than 1GB available
	if availableGB < 1.0 {
		return errors.New(
			errors.ErrCodeDiskSpaceLow,
			fmt.Sprintf("Low disk space: %.2f GB available", availableGB),
		).WithSuggestion(
			"Free up disk space before continuing. "+
				"At least 1GB is recommended.",
		).WithContext("available_gb", availableGB).WithContext("path", wd)
	}

	return nil
}

// checkNetworkConnectivity performs a basic network connectivity check
func checkNetworkConnectivity() error {
	return checkNetworkConnectivityWithContext(context.Background())
}

// checkNetworkConnectivityWithContext performs a basic network connectivity check with context support
func checkNetworkConnectivityWithContext(ctx context.Context) error {
	// Create context with timeout (3 seconds for network check)
	timeoutCtx, cancel := exectimeout.WithTimeoutFromContext(ctx, 3*time.Second)
	defer cancel()

	// Try to ping a well-known host (using curl/wget to check connectivity)
	// This is a basic check - we'll use curl if available, otherwise skip
	cmd := exec.CommandContext(timeoutCtx, "curl", "-s", "--max-time", "3", "https://www.google.com")
	if err := cmd.Run(); err != nil {
		// If curl fails, try with wget
		cmd2 := exec.CommandContext(timeoutCtx, "wget", "--spider", "--timeout=3", "--quiet", "https://www.google.com")
		if err2 := cmd2.Run(); err2 != nil {
			// If both fail, return warning (not error) as network might not be needed
			// Don't treat timeout as error for network check (it's just a warning)
			return errors.New(
				errors.ErrCodeNetworkUnavailable,
				"Network connectivity check failed (may be needed for git clones)",
			).WithSuggestion(
				"Ensure you have internet connectivity if using git-based services. "+
					"This check can be skipped if using only image-based services.",
			).WithContext("check", "connectivity test")
		}
	}
	return nil
}

// CheckWorkspacePermissions checks if workspace directories are writable
func CheckWorkspacePermissions(workspacePath string) error {
	// Check if parent directory exists and is writable
	parentDir := filepath.Dir(workspacePath)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		// Try to create parent directory
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return errors.New(
				errors.ErrCodePermissionDenied,
				fmt.Sprintf("Cannot create workspace directory: %s", parentDir),
			).WithSuggestion(
				fmt.Sprintf(
					"Ensure you have write permissions for %s, "+
						"or run with appropriate permissions",
					parentDir,
				),
			).WithError(err)
		}
	}

	// Check if directory is writable
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		return errors.New(
			errors.ErrCodePermissionDenied,
			fmt.Sprintf("Cannot write to workspace: %s", workspacePath),
		).WithSuggestion(
			fmt.Sprintf(
				"Ensure you have write permissions for %s, "+
					"or run with appropriate permissions",
				workspacePath,
			),
		).WithError(err)
	}

	// Test write access
	testFile := filepath.Join(workspacePath, ".raioz_write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return errors.New(
			errors.ErrCodePermissionDenied,
			fmt.Sprintf("Cannot write to workspace: %s", workspacePath),
		).WithSuggestion(
			fmt.Sprintf(
				"Ensure you have write permissions for %s",
				workspacePath,
			),
		).WithError(err)
	}
	os.Remove(testFile) // Clean up test file

	return nil
}
