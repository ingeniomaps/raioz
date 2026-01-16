package workspace

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"raioz/internal/config"
	"raioz/internal/logging"
)

const (
	stateFileName   = ".state.json"
	composeFileName = "docker-compose.generated.yml"
)

type Workspace struct {
	Root                string
	ServicesDir         string
	LocalServicesDir    string
	ReadonlyServicesDir string
	EnvDir              string
}

// GetBaseDir determines the base directory for raioz workspace
// It tries /opt/raioz-proyecto first, then falls back to user home directory
// Can be overridden with RAIOZ_HOME environment variable
func GetBaseDir() (string, error) {
	// Check for override environment variable
	if home := os.Getenv("RAIOZ_HOME"); home != "" {
		// Try to create directory to verify permissions
		if err := os.MkdirAll(home, 0755); err != nil {
			return "", fmt.Errorf("failed to create RAIOZ_HOME directory '%s': %w", home, err)
		}
		return home, nil
	}

	// Try /opt/raioz-proyecto first (preferred location)
	optBase := "/opt/raioz-proyecto"
	if err := os.MkdirAll(optBase, 0755); err == nil {
		// Successfully created/accessed /opt/raioz-proyecto
		return optBase, nil
	}

	// Failed to create in /opt, use fallback
	fallbackBase, err := getFallbackBaseDir()
	if err != nil {
		return "", fmt.Errorf(
			"failed to use /opt/raioz-proyecto and fallback directory: %w",
			err,
		)
	}

	// Try to create fallback directory
	if err := os.MkdirAll(fallbackBase, 0755); err != nil {
		return "", fmt.Errorf(
			"failed to create fallback directory '%s': %w",
			fallbackBase, err,
		)
	}

	return fallbackBase, nil
}

// getFallbackBaseDir returns the fallback base directory based on OS
func getFallbackBaseDir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}

	homeDir := usr.HomeDir
	if homeDir == "" {
		return "", fmt.Errorf("home directory is empty")
	}

	// Use .raioz in user home directory
	fallbackBase := filepath.Join(homeDir, ".raioz")

	// Normalize path separators for Windows
	if runtime.GOOS == "windows" {
		fallbackBase = filepath.Join(homeDir, ".raioz")
	}

	return fallbackBase, nil
}

// Resolve resolves the workspace for a given project
// It automatically handles fallback if /opt permissions are not available
func Resolve(project string) (*Workspace, error) {
	base, err := GetBaseDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get base directory: %w", err)
	}

	// Log which base directory is being used (for debugging only)
	if base == "/opt/raioz-proyecto" {
		// Using preferred location - no need to log
	} else {
		// Using fallback - only log at debug level (not useful for end users)
		logging.Debug("Using workspace directory (fallback from /opt)", "directory", base)
	}

	root := filepath.Join(base, "workspaces", project)
	services := filepath.Join(base, "services")
	localServices := filepath.Join(root, "local")
	readonlyServices := filepath.Join(root, "readonly")
	envDir := filepath.Join(base, "env")

	// Use 0700 permissions (read/write/execute for owner only) for security
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, fmt.Errorf("failed to create workspace root: %w", err)
	}
	if err := os.MkdirAll(services, 0700); err != nil {
		return nil, fmt.Errorf("failed to create services directory: %w", err)
	}
	if err := os.MkdirAll(localServices, 0700); err != nil {
		return nil, fmt.Errorf("failed to create local services directory: %w", err)
	}
	if err := os.MkdirAll(readonlyServices, 0700); err != nil {
		return nil, fmt.Errorf("failed to create readonly services directory: %w", err)
	}
	if err := os.MkdirAll(envDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create env directory: %w", err)
	}

	// Create env subdirectories (use 0700 for security)
	envServices := filepath.Join(envDir, "services")
	envProjects := filepath.Join(envDir, "projects")
	if err := os.MkdirAll(envServices, 0700); err != nil {
		return nil, fmt.Errorf("failed to create env services directory: %w", err)
	}
	if err := os.MkdirAll(envProjects, 0700); err != nil {
		return nil, fmt.Errorf("failed to create env projects directory: %w", err)
	}

	ws := &Workspace{
		Root:                root,
		ServicesDir:         services,
		LocalServicesDir:    localServices,
		ReadonlyServicesDir: readonlyServices,
		EnvDir:              envDir,
	}

	// Try to load .raioz.json to check for legacy services to migrate
	// This is a best-effort migration, errors are non-fatal
	if deps, _, err := tryLoadDepsForMigration(project); err == nil && deps != nil {
		if err := CheckAndMigrateLegacyServices(ws, deps); err != nil {
			// Log but don't fail - migration is best-effort
			logging.Warn("Migration warning", "error", err)
		}
	}

	return ws, nil
}

// tryLoadDepsForMigration attempts to load .raioz.json for migration purposes
// Returns nil if .raioz.json cannot be found or loaded (non-fatal)
func tryLoadDepsForMigration(project string) (*config.Deps, []string, error) {
	// Try common locations for .raioz.json (and legacy deps.json for backward compatibility)
	possiblePaths := []string{
		".raioz.json",
		"./.raioz.json",
		filepath.Join(".", ".raioz.json"),
		"deps.json",        // Legacy support
		"./deps.json",      // Legacy support
		filepath.Join(".", "deps.json"), // Legacy support
	}

	for _, path := range possiblePaths {
		if deps, warnings, err := config.LoadDeps(path); err == nil {
			return deps, warnings, nil
		}
	}

	return nil, nil, fmt.Errorf(".raioz.json not found for migration")
}

// GetBaseDirFromWorkspace extracts the base directory from a workspace
// This is useful when you need to know the base directory after workspace is resolved
func GetBaseDirFromWorkspace(ws *Workspace) string {
	// Workspace.Root is base/workspaces/project
	// Go up two levels: project -> workspaces -> base
	return filepath.Dir(filepath.Dir(ws.Root))
}

func GetStatePath(ws *Workspace) string {
	return filepath.Join(ws.Root, stateFileName)
}

func GetComposePath(ws *Workspace) string {
	return filepath.Join(ws.Root, composeFileName)
}

func GetEnvDir(ws *Workspace) string {
	return ws.EnvDir
}

// GetLocalServicesDir returns the directory for editable services
func GetLocalServicesDir(ws *Workspace) string {
	return ws.LocalServicesDir
}

// GetReadonlyServicesDir returns the directory for readonly services
func GetReadonlyServicesDir(ws *Workspace) string {
	return ws.ReadonlyServicesDir
}
