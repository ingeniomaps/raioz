package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/app/upcase"
	"raioz/internal/audit"
	"raioz/internal/detect"
	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/errors"
	"raioz/internal/host"
	"raioz/internal/i18n"
	"raioz/internal/logging"
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
		return errors.New(errors.ErrCodeInvalidField, i18n.T("error.dev_name_required")).
			WithSuggestion(i18n.T("error.dev_name_required_suggestion"))
	}

	// Acquire workspace lock before reading state — `raioz dev` is a
	// state mutator (writes DevOverrides). Without the lock, a
	// concurrent `raioz up --watch` save-state can race and lose the
	// override. ADR-023 (state mirrors reality) implicitly
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
// writer invariant from ADR-023 (state mirrors reality). Mirrors upcase.acquireLock's
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
		output.PrintInfo(i18n.T("output.dev_no_overrides"))
		return nil
	}

	output.PrintSectionHeader(i18n.T("output.dev_overrides_header"))
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
			i18n.T("error.dev_not_a_dependency", name),
		).WithSuggestion(i18n.T("error.dev_not_a_dependency_suggestion", infraNames(cfgDeps)))
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

	// The swap replaces a running dependency. With the project down there
	// is nothing to swap and no network to start the local build on.
	if uc.liveDependency(ctx, cfgDeps, name) == "" {
		return errors.New(errors.ErrCodeInvalidConfig,
			i18n.T("error.dev_dependency_not_running", name),
		).WithSuggestion(i18n.T("error.dev_dependency_not_running_suggestion"))
	}

	depCtx, _ := upcase.DependencyContext(ctx, cfgDeps, name, projectDir)
	localCtx := upcase.DevOverrideContext(depCtx, absPath)

	output.PrintInfo(i18n.T("output.dev_promoting", name, string(detection.Runtime)))

	// Stop the dependency through the context up started it with — a
	// context missing the project or the compose scope stops nothing, and
	// the local start then finds the name taken and "reuses" the image.
	dispatcher := orchestrate.NewDispatcher(uc.deps.DockerRunner)
	if err := dispatcher.Stop(ctx, depCtx); err != nil {
		return fmt.Errorf("could not stop %s: %w", name, err)
	}

	// Start the local version. If it does not come up, the dependency
	// goes back to its image: a failed promotion must not leave the
	// project without the dependency.
	if err := dispatcher.Start(ctx, localCtx); err != nil {
		_ = dispatcher.Stop(ctx, localCtx)
		if restoreErr := dispatcher.Start(ctx, depCtx); restoreErr != nil {
			output.PrintWarning(i18n.T("warning.dev_restore_failed", name, restoreErr.Error()))
		}
		return fmt.Errorf("failed to start local %s: %w", name, err)
	}

	// A local version that runs on the host is a process only its PID can
	// reach: recorded like any host service's, so `dev --reset`, `down` and
	// the next `up` find it.
	if pid := dispatcher.GetHostPID(name); pid > 0 {
		if localState.HostPIDs == nil {
			localState.HostPIDs = make(map[string]int)
		}
		localState.HostPIDs[name] = pid
	}

	// Save override in state
	localState.AddDevOverride(name, originalImage, absPath)
	if err := state.SaveLocalState(projectDir, localState); err != nil {
		output.PrintWarning(i18n.T("warning.dev_save_state_failed", err.Error()))
	}

	// Audit the promotion. Failure is logged at debug only —
	// dev mode is already up; an audit miss is not user-visible.
	if auditErr := audit.LogDevPromoted(ctx, name, absPath, originalImage); auditErr != nil {
		logging.DebugWithContext(ctx, "audit LogDevPromoted failed",
			"error", auditErr.Error())
	}

	output.PrintSuccess(i18n.T("output.dev_promoted", name, absPath))
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
		return fmt.Errorf("%s", i18n.T("error.dev_not_in_dev_mode", name))
	}

	output.PrintInfo(i18n.T("output.dev_resetting", name, override.OriginalImage))

	depCtx, _ := upcase.DependencyContext(ctx, cfgDeps, name, projectDir)
	localCtx := upcase.DevOverrideContext(depCtx, override.LocalPath)
	dispatcher := orchestrate.NewDispatcher(uc.deps.DockerRunner)

	// Stop the local version. A host process is stopped by the PID recorded
	// at promotion — this dispatcher is new and never saw it start.
	if pid := localState.HostPIDs[name]; pid > 0 {
		if err := stopHostProcessFn(ctx, pid, "", override.LocalPath); err != nil {
			output.PrintWarning(i18n.T("warning.dev_stop_local_failed", name, err.Error()))
		}
		delete(localState.HostPIDs, name)
	}
	if err := dispatcher.Stop(ctx, localCtx); err != nil {
		output.PrintWarning(i18n.T("warning.dev_stop_local_failed", name, err.Error()))
	}

	// Restart the dependency container
	if err := dispatcher.Start(ctx, depCtx); err != nil {
		return fmt.Errorf("failed to restart %s container: %w", name, err)
	}

	// Remove override from state
	localState.RemoveDevOverride(name)
	if err := state.SaveLocalState(projectDir, localState); err != nil {
		output.PrintWarning(i18n.T("warning.dev_save_state_failed", err.Error()))
	}

	// Audit the revert. Best-effort; same rationale as promote.
	if auditErr := audit.LogDevReverted(ctx, name, override.OriginalImage); auditErr != nil {
		logging.DebugWithContext(ctx, "audit LogDevReverted failed",
			"error", auditErr.Error())
	}

	output.PrintSuccess(i18n.T("output.dev_restored", name, override.OriginalImage))
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

// liveDependency returns the container currently running the dependency,
// "" when there is none.
func (uc *DevUseCase) liveDependency(ctx context.Context, cfgDeps *models.Deps, name string) string {
	proj := &YAMLProject{ProjectName: cfgDeps.Project.Name, Deps: cfgDeps}
	container := proj.liveContainerName(ctx, name)
	if container == "" {
		return ""
	}
	if st, ok := dockerStateProbe(ctx, container); !ok || st.Status != statusRunning {
		return ""
	}
	return container
}

// stopHostProcessFn stops a host process by PID. A package var so tests can
// observe the call without a live process.
var stopHostProcessFn = host.StopServiceWithCommandAndPath

// startHostServiceFn starts a host service through the orchestrator's host
// runner and returns its PID (0 for a launcher that keeps none). A package
// var so tests can observe the call without spawning a process.
var startHostServiceFn = func(
	ctx context.Context, runner interfaces.DockerRunner, svcCtx interfaces.ServiceContext,
) (int, error) {
	dispatcher := orchestrate.NewDispatcher(runner)
	if err := dispatcher.Start(ctx, svcCtx); err != nil {
		return 0, err //nolint:wrapcheck // the caller adds the context
	}
	return dispatcher.GetHostPID(svcCtx.Name), nil
}

// recreateTargetFn tears a container target down and starts it again from
// a freshly built context, so config changes reach it. build is called
// twice on purpose: once for the context to stop, and again after the stop
// so the ports the target itself was holding read as free.
//
// Lives here because this file already imports the orchestrator; a package
// var so tests can observe the call without docker.
var recreateTargetFn = func(
	ctx context.Context, runner interfaces.DockerRunner,
	build func() (interfaces.ServiceContext, bool),
) error {
	svcCtx, ok := build()
	if !ok {
		return fmt.Errorf("not a service or dependency of this project")
	}
	dispatcher := orchestrate.NewDispatcher(runner)
	if err := dispatcher.Stop(ctx, svcCtx); err != nil {
		return fmt.Errorf("stop: %w", err)
	}
	if fresh, ok := build(); ok {
		svcCtx = fresh
	}
	if err := dispatcher.Start(ctx, svcCtx); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	return nil
}
