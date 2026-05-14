package host

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"raioz/internal/domain/models"
	"raioz/internal/env"
	"raioz/internal/logging"
	"raioz/internal/workspace"
)

// resolveEnvVars resolves environment variables for a host service
func resolveEnvVars(
	ctx context.Context, ws *workspace.Workspace,
	deps *models.Deps, serviceName string,
	svc models.Service, projectDir string, servicePath string,
) ([]string, error) {
	// Resolve env file path (same logic as Docker)
	envFilePath, err := env.ResolveEnvFileForService(ws, deps, serviceName, svc.Env, projectDir, servicePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve env file: %w", err)
	}

	var envVars []string
	if envFilePath != "" {
		// Read env file and parse into key=value pairs
		data, err := os.ReadFile(envFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read env file: %w", err)
		}

		// Simple parsing: split by lines, skip comments and empty lines
		lines := parseEnvFile(string(data))
		envVars = append(envVars, lines...)
	}

	return envVars, nil
}

// parseCommand parses a command string into command and arguments
// Uses shell-like parsing: splits by spaces, handles quoted strings
func parseCommand(cmdStr string) []string {
	if cmdStr == "" {
		return nil
	}

	// For now, use simple split (can be enhanced later for quoted strings)
	// This works for most common cases: "npm run dev", "go run main.go", etc.
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return nil
	}

	return parts
}

// createVolumeSymlinks creates symbolic links for host services
// volumes format: ["SRC:DEST", ...] where:
// - SRC is relative to projectDir (or absolute path)
// - DEST is relative to servicePath
func createVolumeSymlinks(volumes []string, projectDir string, servicePath string) error {
	for _, vol := range volumes {
		if vol == "" {
			continue
		}

		// Parse SRC:DEST format
		parts := strings.SplitN(vol, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid volume format '%s': expected 'SRC:DEST'", vol)
		}

		src := strings.TrimSpace(parts[0])
		dest := strings.TrimSpace(parts[1])

		if src == "" || dest == "" {
			return fmt.Errorf("invalid volume format '%s': SRC and DEST cannot be empty", vol)
		}

		// Resolve SRC to absolute path (relative to projectDir if not absolute)
		var srcAbs string
		if filepath.IsAbs(src) {
			srcAbs = src
		} else {
			if projectDir == "" {
				return fmt.Errorf("cannot resolve relative path '%s': projectDir is not provided", src)
			}
			srcAbs = filepath.Join(projectDir, src)
		}

		// Create source path if it doesn't exist (file or directory)
		if _, err := os.Stat(srcAbs); os.IsNotExist(err) {
			// Source doesn't exist, determine if it should be a file or directory
			// Check if destination suggests it's a file (has file extension or doesn't end with common dir patterns)
			destBase := filepath.Base(dest)
			hasExtension := filepath.Ext(destBase) != ""
			isLikelyFile := hasExtension && destBase != "." && destBase != ".."

			if isLikelyFile {
				// Likely a file - ensure parent directory exists and create empty file
				parentDir := filepath.Dir(srcAbs)
				if err := os.MkdirAll(parentDir, 0755); err != nil {
					return fmt.Errorf("failed to create parent directory for source file: %w", err)
				}
				// Create empty file
				file, err := os.Create(srcAbs)
				if err != nil {
					return fmt.Errorf("failed to create source file: %w", err)
				}
				file.Close()
			} else {
				// Likely a directory - create directory
				if err := os.MkdirAll(srcAbs, 0755); err != nil {
					return fmt.Errorf("failed to create source directory: %w", err)
				}
			}
		} else if err != nil {
			return fmt.Errorf("failed to check source path: %w", err)
		}

		// Resolve DEST to absolute path (relative to servicePath)
		destAbs := filepath.Join(servicePath, dest)

		// Ensure parent directory of destination exists
		destParent := filepath.Dir(destAbs)
		if err := os.MkdirAll(destParent, 0755); err != nil {
			return fmt.Errorf("failed to create parent directory for destination: %w", err)
		}

		// Check if destination already exists
		if _, err := os.Lstat(destAbs); err == nil {
			// Destination exists, check if it's already a symlink pointing to the same target
			if linkInfo, err := os.Readlink(destAbs); err == nil {
				// It's a symlink, check if it points to the same target
				linkAbs, err := filepath.Abs(filepath.Join(filepath.Dir(destAbs), linkInfo))
				if err == nil {
					srcAbsResolved, err := filepath.Abs(srcAbs)
					if err == nil && linkAbs == srcAbsResolved {
						// Already linked to the same target, skip
						continue
					}
				}
				// Remove existing symlink to recreate it
				if err := os.Remove(destAbs); err != nil {
					return fmt.Errorf("failed to remove existing symlink: %w", err)
				}
			} else {
				// Destination exists but is not a symlink
				return fmt.Errorf("destination path already exists and is not a symlink: %s", destAbs)
			}
		}

		// Resolve source to absolute path for symlink
		srcAbsResolved, err := filepath.Abs(srcAbs)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path for source: %w", err)
		}

		// Create symlink (use absolute path for source)
		if err := os.Symlink(srcAbsResolved, destAbs); err != nil {
			return fmt.Errorf("failed to create symlink from %s to %s: %w", srcAbsResolved, destAbs, err)
		}
	}

	return nil
}

// shouldWaitForCommand determines if a command should be executed synchronously
// Commands that should wait: make launch, make stop, docker-compose up, scripts, etc.
// Commands that should run in background: npm run dev, go run main.go, etc.
func shouldWaitForCommand(command string) bool {
	// Commands that should complete before continuing
	waitCommands := []string{
		"make launch",
		"make stop",
		"docker-compose up",
		"docker-compose down",
		"docker compose up",
		"docker compose down",
	}

	commandLower := strings.ToLower(command)
	for _, waitCmd := range waitCommands {
		if strings.Contains(commandLower, waitCmd) {
			return true
		}
	}

	// Scripts (installer.sh, setup.sh, etc.) should execute synchronously to catch errors
	// These are typically deployment/setup scripts that should complete before continuing
	isScript := strings.HasSuffix(commandLower, ".sh") ||
		strings.HasPrefix(commandLower, "./") ||
		strings.HasPrefix(commandLower, "sh ")
	if isScript {
		// Exclude long-running scripts that should run in background
		// If it's a simple script execution (not npm run, go run, etc.), wait for it
		if !strings.Contains(commandLower, "npm run") &&
			!strings.Contains(commandLower, "go run") &&
			!strings.Contains(commandLower, "python") &&
			!strings.Contains(commandLower, "node") {
			return true
		}
	}

	// Default: run in background for long-running services
	return false
}

// formatEarlyExitError builds the error returned when a host process exits
// inside the settle window. Includes the tail of the stderr log
// so the user sees the real reason without having to dig through log files.
func formatEarlyExitError(name string, window time.Duration, exitErr error, stderrPath string) error {
	tail := ReadLogTail(stderrPath, 8)
	if tail == "" {
		return fmt.Errorf("service %q exited within %s: %w", name, window, exitErr)
	}
	return fmt.Errorf(
		"service %q exited within %s: %w\n--- stderr tail ---\n%s",
		name, window, exitErr, tail,
	)
}

// FormatEarlyExitError is the exported form of formatEarlyExitError, used by
// other packages (orchestrate.HostRunner) that share the settle-window
// behavior.
func FormatEarlyExitError(name string, window time.Duration, exitErr error, stderrPath string) error {
	return formatEarlyExitError(name, window, exitErr, stderrPath)
}

// SettleWindow returns the configured settle window. Exported so other
// packages (orchestrate) use the same default and tests can read it.
func SettleWindow() time.Duration {
	return startSettleWindow
}

// SetSettleWindowForTest overrides the settle window for the duration of a
// test. Returns a restore func the caller must defer. Exported so tests in
// other packages can shrink the window without touching package internals.
func SetSettleWindowForTest(d time.Duration) (restore func()) {
	prev := startSettleWindow
	startSettleWindow = d
	return func() { startSettleWindow = prev }
}

const (
	launcherWaitTimeoutEnv  = "RAIOZ_LAUNCHER_TIMEOUT"
	launcherDrainTimeoutEnv = "RAIOZ_LAUNCHER_DRAIN_TIMEOUT"
)

// LauncherWaitTimeout — post-launcher container-appearance wait
// during `raioz up`. ADR-025.
func LauncherWaitTimeout() time.Duration {
	return durationFromEnv(launcherWaitTimeoutEnv, 60*time.Second)
}

// LauncherDrainTimeout — wait for an in-progress launcher build
// during `raioz down` before invoking `stop:`. ADR-025.
func LauncherDrainTimeout() time.Duration {
	return durationFromEnv(launcherDrainTimeoutEnv, 30*time.Second)
}

// EnvDurationStatus snapshots how a duration-typed env var resolved.
// Used by `raioz doctor` to surface user overrides and malformed
// values that durationFromEnv silently masked behind the default.
// See ADR-035.
type EnvDurationStatus struct {
	Name      string        // env var name (e.g. RAIOZ_LAUNCHER_TIMEOUT)
	Raw       string        // user-supplied value; "" when unset
	Resolved  time.Duration // value actually used
	Default   time.Duration
	Malformed bool // true when Raw was set but couldn't parse / was negative
}

// InspectDurationEnv reads a duration env var WITHOUT logging side
// effects. Inverse of durationFromEnv when callers need to render
// the resolution state (`raioz doctor`) instead of just consuming
// the resolved value.
func InspectDurationEnv(name string, def time.Duration) EnvDurationStatus {
	raw := osGetenv(name)
	if raw == "" {
		return EnvDurationStatus{Name: name, Resolved: def, Default: def}
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		return EnvDurationStatus{
			Name: name, Raw: raw, Resolved: def, Default: def, Malformed: true,
		}
	}
	return EnvDurationStatus{Name: name, Raw: raw, Resolved: d, Default: def}
}

// KnownDurationEnvs enumerates every duration-typed env var raioz
// reads. `raioz doctor` walks this list to print resolution state.
// New duration env vars MUST be appended here so the doctor surfaces
// them — otherwise a typo'd value stays silent. See ADR-035.
func KnownDurationEnvs() []EnvDurationStatus {
	return []EnvDurationStatus{
		InspectDurationEnv(launcherWaitTimeoutEnv, 60*time.Second),
		InspectDurationEnv(launcherDrainTimeoutEnv, 30*time.Second),
	}
}

// warnedEnvOnce tracks env vars we've already warned about so a
// hot loop reading the same malformed var doesn't spam the log.
// Per-process scope; tests reset via ResetMalformedEnvWarningsForTest.
var warnedEnvOnce sync.Map

// ResetMalformedEnvWarningsForTest clears the once-per-process
// dedup so tests can verify the warning fires on the first hit
// without depending on test ordering. Test-only.
func ResetMalformedEnvWarningsForTest() {
	warnedEnvOnce = sync.Map{}
}

// "0s" is honored as explicit opt-out; "" falls back to def; an
// unparseable or negative value also returns def but logs a warning
// once per (process, var name) so the user spots typos like
// "RAIOZ_LAUNCHER_TIMEOUT=60" (missing "s") instead of seeing the
// default silently. See ADR-035.
func durationFromEnv(name string, def time.Duration) time.Duration {
	s := InspectDurationEnv(name, def)
	if s.Malformed {
		if _, loaded := warnedEnvOnce.LoadOrStore(name, true); !loaded {
			logging.Warn("invalid duration env var; using default",
				"var", name,
				"value", s.Raw,
				"default", def.String(),
				"hint", "expected Go duration like 60s, 2m, 1h",
			)
		}
	}
	return s.Resolved
}

// Indirection seam for tests; never reassigned in non-test code.
var osGetenv = os.Getenv

// ReadLogTail returns the last `lines` lines of a file as a single string,
// or empty if the file is missing or unreadable. Best-effort — never errors.
func ReadLogTail(path string, lines int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	s := strings.TrimRight(string(data), "\n")
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "\n")
	if len(parts) <= lines {
		return s
	}
	return strings.Join(parts[len(parts)-lines:], "\n")
}

// parseEnvFile parses an env file content into key=value pairs
func parseEnvFile(content string) []string {
	var vars []string
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		// Trim whitespace
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Add to vars (assumes format: KEY=VALUE)
		vars = append(vars, line)
	}

	return vars
}
