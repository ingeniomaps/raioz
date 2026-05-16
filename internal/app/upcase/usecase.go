package upcase

import (
	"context"
	"io"
	"os"
	"time"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/errors"
	"raioz/internal/host"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// Options contains options for the Up use case
type Options struct {
	ConfigPath   string
	Profile      string
	ForceReclone bool
	DryRun       bool
	Only         []string
	Host         string // Bind address for shared dev server (e.g., "0.0.0.0")
	// Attach: stream logs in the foreground without file watching.
	// Blocks until Ctrl+C. The lock is released before blocking.
	Attach bool
	// Watch: file-watch services with `watch: true` and auto-restart.
	// Blocks until Ctrl+C. The lock is released before blocking.
	// Default (Attach=false && Watch=false): raioz up exits cleanly after
	// services are healthy. Services keep running; use `raioz logs` / `raioz down`.
	Watch bool
	// RouterOff forces the bundled Caddy to start even when
	// RAIOZ_ROUTER_ACTIVE=1 is inherited from the shell. Use to
	// debug a consumer's own proxy in isolation from a meta run, or
	// to recover from a shell with a leaked env var (issue 030).
	RouterOff bool
}

// Dependencies contains the dependencies needed by the up use case
type Dependencies struct {
	ConfigLoader     interfaces.ConfigLoader
	Validator        interfaces.Validator
	DockerRunner     interfaces.DockerRunner
	GitRepository    interfaces.GitRepository
	Workspace        interfaces.WorkspaceManager
	StateManager     interfaces.StateManager
	LockManager      interfaces.LockManager
	HostRunner       interfaces.HostRunner
	EnvManager       interfaces.EnvManager
	ProxyManager     interfaces.ProxyManager     // Optional: nil if proxy not needed
	DiscoveryManager interfaces.DiscoveryManager // Optional: nil if discovery not needed
}

// UseCase handles the "up" use case - starting a project
type UseCase struct {
	deps *Dependencies
	Out  io.Writer
}

// NewUseCase creates a new UpUseCase with injected dependencies
func NewUseCase(deps *Dependencies) *UseCase {
	return &UseCase{
		deps: deps,
		Out:  os.Stdout,
	}
}

// out returns the output writer
func (uc *UseCase) out() io.Writer {
	if uc.Out != nil {
		return uc.Out
	}
	return os.Stdout
}

// Execute executes the up use case
func (uc *UseCase) Execute(ctx context.Context, opts Options) (err error) {
	startTime := time.Now()

	// Bootstrap: context, logging, config loading, overrides
	br, err := uc.bootstrap(ctx, opts.ConfigPath)
	if err != nil {
		return err
	}
	ctx = br.ctx
	deps := br.deps
	ws := br.ws
	appliedOverrides := br.appliedOverrides

	// Emit lifecycle audit events. Start fires after
	// bootstrap so the event carries project/workspace; complete is
	// deferred so every return path (success, error, panic-after-
	// recover) closes the pair with status + duration + error.
	emitLifecycleStart(ctx, deps)
	defer func() { emitLifecycleComplete(ctx, deps, startTime, err) }()

	projectDir, projectEnvPath, err := uc.resolveProjectContext(ctx, opts, deps, ws)
	if err != nil {
		return err
	}

	// Filters: profile, feature flags, ignore list, --only
	deps, err = uc.applyFilters(deps, opts.Profile, opts.Only)
	if err != nil {
		return err
	}

	// Save filtered deps for re-applying --only after merge
	var onlyFilteredDeps *models.Deps
	if len(opts.Only) > 0 {
		depsCopy := *deps
		onlyFilteredDeps = &depsCopy
	}

	// Check what we have: services, infra, or project commands
	hasServices := len(deps.Services) > 0
	hasInfra := len(deps.Infra) > 0
	hasProjectCommands := deps.Project.Commands != nil && (deps.Project.Commands.Up != "" ||
		(deps.Project.Commands.Dev != nil && deps.Project.Commands.Dev.Up != "") ||
		(deps.Project.Commands.Prod != nil && deps.Project.Commands.Prod.Up != ""))

	// If only project commands, skip services/infra processing
	onlyProjectCommands := !hasServices && !hasInfra && hasProjectCommands

	// Validation: validate.All, permissions, ports, dependency conflicts
	err = uc.validate(ctx, deps, ws, opts.DryRun)
	if err != nil {
		return err
	}

	// Dry-run: show what would happen and exit
	if opts.DryRun {
		uc.showDryRunSummary(deps, appliedOverrides)
		return nil
	}

	// Pre-hook: runs before anything else (env rendering, secrets fetch, etc.).
	// Must run before generateEnvFilesFromTemplates and docker prep so that files
	// the hook produces are visible to downstream steps. A failure aborts the run.
	if err := uc.preHookExec(ctx, deps, projectDir); err != nil {
		return err
	}

	// If only project commands, execute them directly and return
	if onlyProjectCommands {
		if err := uc.deps.EnvManager.WriteGlobalEnvVariables(ws, deps, projectDir); err != nil {
			return errors.New(
				errors.ErrCodeWorkspaceError,
				i18n.T("error.global_env_write"),
			).WithSuggestion(
				i18n.T("error.global_env_write_suggestion"),
			).WithError(err)
		}
		// Acquire lock
		lockInstance, err := uc.acquireLock(ctx, ws)
		if err != nil {
			return err
		}
		defer func() { _ = lockInstance.Release() }()

		// Execute project command directly
		err = uc.processLocalProject(ctx, opts.ConfigPath, deps, "up", ws)
		if err != nil {
			return err
		}

		// Post-hook (project-only path): runs after project command succeeds.
		uc.postHookExec(ctx, deps, projectDir)

		// Final summary (no services/infra)
		uc.showSummary(ctx, deps, []string{}, []string{}, startTime)
		return nil
	}

	// Check for dependencies on running projects (before processing services)
	if err := uc.checkDependencyProjects(ctx, deps); err != nil {
		return err
	}

	// Normal flow: process services and infra first
	// Acquire lock
	lockInstance, err := uc.acquireLock(ctx, ws)
	if err != nil {
		return err
	}
	defer func() { _ = lockInstance.Release() }()

	// Check if workspace is already running from a different project (same workspace, overlapping services)
	conflictResult, mergedDeps, err := uc.checkWorkspaceProjectConflict(ctx, deps, ws, projectDir)
	if err != nil {
		return err
	}
	if conflictResult == WorkspaceConflictSkip {
		return nil
	}
	if mergedDeps != nil {
		deps = mergedDeps
		// Re-apply --only filter from original config (merge may have overwritten with state data)
		if onlyFilteredDeps != nil {
			svcNames, infraNames := config.ResolveDependencies(onlyFilteredDeps, opts.Only)
			deps = config.FilterByServices(onlyFilteredDeps, svcNames, infraNames)
		}
	}
	// WorkspaceConflictProceed: continue (with deps or merged deps)

	// Write global.env as union of env.files + env.variables
	// (after merge so merged config includes all projects' variables)
	if err := uc.deps.EnvManager.WriteGlobalEnvVariables(ws, deps, projectDir); err != nil {
		return errors.New(
			errors.ErrCodeWorkspaceError,
			i18n.T("error.global_env_write"),
		).WithSuggestion(
			i18n.T("error.global_env_write_suggestion"),
		).WithError(err)
	}

	// State: load state, detect changes
	oldDeps, changes, addedServices, assistedServicesMap, err := uc.processState(ctx, deps, ws, opts.ConfigPath)
	if err != nil {
		return err
	}

	// Check if services are already running (if no changes)
	shouldSkip, err := uc.checkServicesRunning(ctx, deps, ws, changes, oldDeps)
	if err != nil {
		return err
	}
	if shouldSkip {
		// Services are running, but still execute project command if exists
		if hasProjectCommands {
			err = uc.processLocalProject(ctx, opts.ConfigPath, deps, "up", ws)
			if err != nil {
				logging.WarnWithContext(ctx, "Failed to execute local project command", "error", err.Error())
			}
		}
		// Post-hook still runs on skipped path — user scripts should be idempotent.
		uc.postHookExec(ctx, deps, projectDir)
		return nil
	}

	// Git: clone/update repos, handle branch changes
	err = uc.processGitRepos(ctx, deps, ws, oldDeps, opts.ForceReclone, projectDir)
	if err != nil {
		return err
	}

	// Generate .env files from templates for all services (after cloning, before starting)
	output.PrintProgress(i18n.T("up.generating_env_files"))
	err = uc.generateEnvFilesFromTemplates(ctx, deps, ws, projectEnvPath, projectDir)
	if err != nil {
		// Log but don't fail - template generation is optional
		logging.WarnWithContext(ctx, "Some .env files could not be generated from templates", "error", err.Error())
		output.PrintProgressError(i18n.T("up.env_files_error"))
	} else {
		output.PrintProgressDone(i18n.T("up.env_files_done"))
	}

	// Docker prepare: images, network, volumes
	// IMPORTANT: Network must be created BEFORE host services, as host services may need the network
	err = uc.prepareDockerResources(ctx, deps, ws)
	if err != nil {
		return err
	}

	var composePath string
	var serviceNames, infraNames []string
	var hostProcessInfo map[string]*host.ProcessInfo
	var orchResult *orchestrationResult

	if isYAMLMode(deps) {
		// New orchestrator flow: detect runtimes, start with native tools
		orchResult, err = uc.processOrchestration(ctx, deps, ws, projectDir, opts.ConfigPath, opts.RouterOff)
		if err != nil {
			return err
		}
		serviceNames = orchResult.serviceNames
		infraNames = orchResult.infraNames
	} else {
		// Legacy flow: host services + generate compose
		hostProcessInfo, err = uc.processHostServices(ctx, deps, ws, projectDir)
		if err != nil {
			return err
		}

		composePath, serviceNames, infraNames, err = uc.processCompose(ctx, deps, ws, projectDir)
		if err != nil {
			if len(hostProcessInfo) > 0 {
				_ = uc.stopHostServices(ctx, hostProcessInfo)
			}
			return err
		}
	}

	// State: save state, root config, drift detection, audit
	// (persist project root so merge can resolve volumes per project)
	deps.ProjectRoot = projectDir
	err = uc.saveState(ctx, deps, ws, composePath, serviceNames, addedServices, assistedServicesMap, appliedOverrides)
	if err != nil {
		return err
	}

	// Save host processes state
	if len(hostProcessInfo) > 0 {
		if err := uc.saveHostProcessesState(ctx, ws, hostProcessInfo); err != nil {
			// Log but don't fail - host processes state is optional
			logging.WarnWithContext(ctx, "Failed to save host processes state", "error", err.Error())
		}
	}

	// Update global state — best-effort; global state is optional.
	_ = uc.updateGlobalState(ctx, deps, ws, composePath, serviceNames)

	// Wait for services and infra to be healthy before executing project commands
	// This ensures that project.commands.up runs only after dependencies are ready
	if hasProjectCommands && (len(serviceNames) > 0 || len(infraNames) > 0) {
		output.PrintProgress(i18n.T("up.waiting_healthy_before_cmd"))
		err := uc.deps.DockerRunner.WaitForServicesHealthy(
			ctx, composePath, serviceNames, infraNames, deps.Project.Name,
		)
		if err != nil {
			logging.WarnWithContext(ctx, "Failed to wait for services to be healthy", "error", err.Error())
			output.PrintWarning(i18n.T("up.services_not_healthy_warning"))
			// Continue anyway - user may want to proceed even if health checks fail
		}
	}

	// Execute local project command as final step (if both services and commands exist)
	if hasProjectCommands {
		err = uc.processLocalProject(ctx, opts.ConfigPath, deps, "up", ws)
		if err != nil {
			// Log but don't fail - local project command is optional
			logging.WarnWithContext(ctx, "Failed to execute local project command", "error", err.Error())
		}
	}

	// Post-hook: runs after everything is up. Failures are warnings, not errors.
	uc.postHookExec(ctx, deps, projectDir)

	// Final summary
	uc.showSummary(ctx, deps, serviceNames, infraNames, startTime)

	// Foreground phase is opt-in. By default `raioz up` exits cleanly once
	// services are healthy so multiple projects in the same workspace can
	// coexist without lock contention. The lock is released explicitly here
	// so --attach / --watch don't hold it for the whole session.
	_ = lockInstance.Release()

	if orchResult != nil {
		switch {
		case opts.Attach:
			// Stream logs in the foreground without file watching.
			streamForeground(ctx, deps, orchResult.detections)
		case opts.Watch:
			// File-watch services with `watch: true` and auto-restart.
			startWatcher(ctx, deps, orchResult.dispatcher, orchResult.detections,
				orchResult.networkName, projectDir)
		}
	}

	return nil
}
