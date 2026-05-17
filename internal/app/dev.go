package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/audit"
	"raioz/internal/detect"
	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/errors"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/orchestrate"
	"raioz/internal/output"
	"raioz/internal/state"
)

// DevOptions contains options for the dev use case.
type DevOptions struct {
	ConfigPath string
	Name       string // dependency name to promote
	LocalPath  string // local path for the dependency
	Reset      bool   // reset back to image
	List       bool   // list current dev overrides
}

// DevUseCase handles promoting dependencies from image to local development.
type DevUseCase struct {
	deps *Dependencies
}

// NewDevUseCase creates a new DevUseCase.
func NewDevUseCase(deps *Dependencies) *DevUseCase {
	return &DevUseCase{deps: deps}
}

// Execute runs the dev use case.
func (uc *DevUseCase) Execute(ctx context.Context, opts DevOptions) error {
	projectDir, err := filepath.Abs(filepath.Dir(opts.ConfigPath))
	if err != nil {
		return fmt.Errorf("cannot resolve project directory: %w", err)
	}

	// List mode reads state but doesn't mutate; skip the lock for it.
	if opts.List {
		localState, err := state.LoadLocalState(projectDir)
		if err != nil {
			return fmt.Errorf("cannot load project state: %w", err)
		}
		return uc.listOverrides(localState)
	}

	// Load config to find the dependency.
	cfgDeps, warnings, err := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
	for _, w := range warnings {
		output.PrintWarning(w)
	}
	if err != nil {
		return fmt.Errorf("cannot load config: %w", err)
	}

	if opts.Name == "" {
		return fmt.Errorf("dependency name is required")
	}

	// Acquire workspace lock before reading state — `raioz dev` is a
	// state mutator (writes DevOverrides). Without the lock, a
	// concurrent `raioz up --watch` save-state can race and lose the
	// override. Issue 038. ADR-023 (state mirrors reality) implicitly
	// requires serialized writers; codified in CLAUDE.md invariants.
	releaseLock, err := uc.acquireWorkspaceLock(ctx, cfgDeps.Project.Name)
	if err != nil {
		return err
	}
	defer releaseLock()

	localState, err := state.LoadLocalState(projectDir)
	if err != nil {
		return fmt.Errorf("cannot load project state: %w", err)
	}

	// Reset mode
	if opts.Reset {
		return uc.resetOverride(ctx, opts.Name, cfgDeps, projectDir, localState)
	}

	// Promote mode
	return uc.promote(ctx, opts.Name, opts.LocalPath, cfgDeps, projectDir, localState)
}

// acquireWorkspaceLock takes the workspace lock for the given project.
// Returns a release func the caller must defer. Implements the state-
// writer invariant from issue 038. Mirrors upcase.acquireLock's
// behavior under recursive sibling spawn (no-op then).
func (uc *DevUseCase) acquireWorkspaceLock(
	ctx context.Context, projectName string,
) (func(), error) {
	ws, err := uc.deps.Workspace.Resolve(projectName)
	if err != nil {
		return func() {}, fmt.Errorf("resolve workspace: %w", err)
	}
	if ws == nil {
		// Test / no-workspace path — skip the lock. Mirrors the
		// recursive-sibling-spawn behaviour in upcase.acquireLock.
		return func() {}, nil
	}
	lock, err := uc.deps.LockManager.Acquire(ws)
	if err != nil {
		return func() {}, fmt.Errorf("acquire workspace lock: %w", err)
	}
	logging.DebugWithContext(ctx, "dev: workspace lock acquired",
		"workspace", ws.Root)
	return func() {
		if err := lock.Release(); err != nil {
			logging.WarnWithContext(ctx, "dev: failed to release workspace lock",
				"error", err.Error())
		}
	}, nil
}

// listOverrides shows all active dev overrides.
func (uc *DevUseCase) listOverrides(localState *models.LocalState) error {
	if len(localState.DevOverrides) == 0 {
		output.PrintInfo("No active dev overrides")
		return nil
	}

	output.PrintSectionHeader("Dev overrides")
	for name, override := range localState.DevOverrides {
		output.PrintKeyValue(name, fmt.Sprintf(
			"%s → %s (was: %s)",
			name, override.LocalPath, override.OriginalImage,
		))
	}
	return nil
}

// promote stops the dependency container and starts a local service in its place.
func (uc *DevUseCase) promote(
	ctx context.Context,
	name, localPath string,
	cfgDeps *models.Deps,
	projectDir string,
	localState *models.LocalState,
) error {
	// Validate dependency exists
	entry, ok := cfgDeps.Infra[name]
	if !ok {
		return errors.New(errors.ErrCodeNotADependency,
			"'"+name+"' is not a dependency",
		).WithSuggestion(
			"Available dependencies: " + infraNames(cfgDeps) + "\n" +
				"  Only items in 'dependencies:' can be promoted to local.\n" +
				"  Items in 'services:' are already local.",
		)
	}

	// Validate local path
	if localPath == "" {
		return fmt.Errorf("local path is required: raioz dev %s <path>", name)
	}
	absPath, err := filepath.Abs(localPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("path does not exist: %s", absPath)
	}

	// Build original image ref
	originalImage := ""
	if entry.Inline != nil {
		originalImage = entry.Inline.Image
		if entry.Inline.Tag != "" {
			originalImage += ":" + entry.Inline.Tag
		}
	}

	// Detect runtime of the local path
	detection := detect.Detect(absPath)
	if detection.Runtime == models.RuntimeUnknown {
		return errors.RuntimeNotDetected(name, absPath)
	}

	output.PrintInfo(fmt.Sprintf("Promoting %s: image → local (%s)", name, detection.Runtime))

	// Stop the dependency container
	dispatcher := orchestrate.NewDispatcher(uc.deps.DockerRunner)
	networkName := cfgDeps.Network.GetName()
	containerName := naming.Container(cfgDeps.Project.Name, name)

	stopCtx := interfaces.ServiceContext{
		Name:          name,
		ContainerName: containerName,
		NetworkName:   networkName,
		Detection:     detect.ForImage(originalImage),
	}
	if err := dispatcher.Stop(ctx, stopCtx); err != nil {
		output.PrintWarning(fmt.Sprintf("Could not stop %s container: %s", name, err))
	}

	// Start the local version
	startCtx := interfaces.ServiceContext{
		Name:          name,
		Path:          absPath,
		Detection:     detection,
		NetworkName:   networkName,
		ContainerName: containerName,
		EnvVars:       map[string]string{},
		Ports:         infraPorts(entry),
	}
	if err := dispatcher.Start(ctx, startCtx); err != nil {
		return fmt.Errorf("failed to start local %s: %w", name, err)
	}

	// Save override in state
	localState.AddDevOverride(name, originalImage, absPath)
	if err := state.SaveLocalState(projectDir, localState); err != nil {
		output.PrintWarning("Failed to save state: " + err.Error())
	}

	// Audit the promotion. Failure is logged at debug only —
	// dev mode is already up; an audit miss is not user-visible.
	if auditErr := audit.LogDevPromoted(ctx, name, absPath, originalImage); auditErr != nil {
		logging.DebugWithContext(ctx, "audit LogDevPromoted failed",
			"error", auditErr.Error())
	}

	output.PrintSuccess(fmt.Sprintf("%s: now running from %s", name, absPath))
	return nil
}

// resetOverride stops the local service and restarts the dependency container.
func (uc *DevUseCase) resetOverride(
	ctx context.Context,
	name string,
	cfgDeps *models.Deps,
	projectDir string,
	localState *models.LocalState,
) error {
	override, ok := localState.GetDevOverride(name)
	if !ok {
		return fmt.Errorf("'%s' is not in dev mode", name)
	}

	output.PrintInfo(fmt.Sprintf("Resetting %s: local → image (%s)", name, override.OriginalImage))

	dispatcher := orchestrate.NewDispatcher(uc.deps.DockerRunner)
	networkName := cfgDeps.Network.GetName()
	containerName := naming.Container(cfgDeps.Project.Name, name)

	// Stop the local version
	localDetection := detect.Detect(override.LocalPath)
	stopCtx := interfaces.ServiceContext{
		Name:          name,
		Path:          override.LocalPath,
		Detection:     localDetection,
		NetworkName:   networkName,
		ContainerName: containerName,
	}
	if err := dispatcher.Stop(ctx, stopCtx); err != nil {
		output.PrintWarning(fmt.Sprintf("Could not stop local %s: %s", name, err))
	}

	// Restart the dependency container
	entry := cfgDeps.Infra[name]
	envVars := map[string]string{}
	if entry.Inline != nil {
		envVars["RAIOZ_IMAGE"] = override.OriginalImage
	}

	startCtx := interfaces.ServiceContext{
		Name:          name,
		ContainerName: containerName,
		NetworkName:   networkName,
		Detection:     detect.ForImage(override.OriginalImage),
		EnvVars:       envVars,
		Ports:         infraPorts(entry),
	}
	if err := dispatcher.Start(ctx, startCtx); err != nil {
		return fmt.Errorf("failed to restart %s container: %w", name, err)
	}

	// Remove override from state
	localState.RemoveDevOverride(name)
	if err := state.SaveLocalState(projectDir, localState); err != nil {
		output.PrintWarning("Failed to save state: " + err.Error())
	}

	// Audit the revert. Best-effort; same rationale as promote.
	if auditErr := audit.LogDevReverted(ctx, name, override.OriginalImage); auditErr != nil {
		logging.DebugWithContext(ctx, "audit LogDevReverted failed",
			"error", auditErr.Error())
	}

	output.PrintSuccess(fmt.Sprintf("%s: restored to %s", name, override.OriginalImage))
	return nil
}

func infraNames(deps *models.Deps) string {
	names := ""
	for name := range deps.Infra {
		if names != "" {
			names += ", "
		}
		names += name
	}
	return names
}

func infraPorts(entry models.InfraEntry) []string {
	if entry.Inline != nil {
		return entry.Inline.Ports
	}
	return nil
}
