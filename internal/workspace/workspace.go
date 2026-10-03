package workspace

import (
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/domain/models"
	"raioz/internal/logging"
	"raioz/internal/naming"
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

// GetBaseDir returns the base directory for raioz state.
// Location delegated to naming.RaiozStateDir() — ADR-022.
func GetBaseDir() (string, error) {
	base := naming.RaiozStateDir()
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", fmt.Errorf("failed to create workspace base dir %q: %w", base, err)
	}
	return base, nil
}

// Resolve resolves the workspace for a given project
// It automatically handles fallback if /opt permissions are not available
func Resolve(project string) (*Workspace, error) {
	base, err := GetBaseDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get base directory: %w", err)
	}

	// Using fallback path: debug-log so the dev can trace which root the
	// resolver picked without noising the happy path.
	if base != "/opt/raioz-proyecto" {
		logging.Debug("Using workspace directory (fallback from /opt)", "directory", base)
	}

	root := filepath.Join(base, "workspaces", project)
	services := filepath.Join(base, "services")
	localServices := filepath.Join(root, "local")
	readonlyServices := filepath.Join(root, "readonly")
	// EnvDir is now workspace-specific (was shared before)
	envDir := filepath.Join(root, "env")

	// Nothing is created here. Resolve answers "where would this project's
	// state live", and most callers only read: creating the tree on every
	// `status` or `check` left an empty directory per project ever looked
	// at. Writers create what they write into (EnsureDirs, or their own
	// MkdirAll).
	ws := &Workspace{
		Root:                root,
		ServicesDir:         services,
		LocalServicesDir:    localServices,
		ReadonlyServicesDir: readonlyServices,
		EnvDir:              envDir,
	}

	// Try to load .raioz.json to check for legacy services to migrate.
	// Hook injected by callers that can import internal/config; nil means
	// "no legacy migration available", which is fine for fresh workspaces.
	if LoadDepsForMigration != nil {
		if deps, _, err := LoadDepsForMigration(project); err == nil && deps != nil {
			if err := CheckAndMigrateLegacyServices(ws, deps); err != nil {
				logging.Warn("Migration warning", "error", err)
			}
		}
	}

	return ws, nil
}

// LoadDepsForMigration is an optional hook that, when set, lets Resolve
// detect a legacy .raioz.json at the project root and migrate its services
// automatically. The hook is set by callers that can afford to import
// internal/config (typically internal/infra/workspace via init()). Keeping
// it as a hook rather than a direct import preserves the domain isolation
// boundary required by ADR-009: nothing under internal/workspace pulls in
// internal/config, even transitively.
var LoadDepsForMigration func(project string) (*models.Deps, []string, error)

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

// EnsureDirs creates the workspace tree with owner-only permissions. Called
// by the flows that are about to write into it.
func EnsureDirs(ws *Workspace) error {
	if ws == nil {
		return nil
	}
	dirs := []string{
		ws.Root, ws.ServicesDir, ws.LocalServicesDir, ws.ReadonlyServicesDir,
		filepath.Join(ws.EnvDir, "services"), filepath.Join(ws.EnvDir, "projects"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create workspace directory %s: %w", dir, err)
		}
	}
	return nil
}
