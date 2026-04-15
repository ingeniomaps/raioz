package upcase

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
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
	Attach       bool   // Stay attached and stream logs without file watching
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
func (uc *UseCase) Execute(ctx context.Context, opts Options) error {
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

	// Get project directory (where .raioz.json is located)
	projectDir, err := filepath.Abs(filepath.Dir(opts.ConfigPath))
	if err != nil {
		return errors.New(
			errors.ErrCodeWorkspaceError,
			i18n.T("error.project_dir"),
		).WithError(err)
	}

	// Resolve project.env (if project.env is ["."], uses .env in project directory as primary)
	projectEnvPath, err := uc.deps.EnvManager.ResolveProjectEnv(ws, deps, projectDir)
	if err != nil {
		return errors.New(
			errors.ErrCodeWorkspaceError,
			i18n.T("error.project_env_resolve"),
		).WithSuggestion(
			i18n.T("error.project_env_resolve_suggestion"),
		).WithError(err)
	}

	// Filters: profile, feature flags, ignore list, --only
	deps, err = uc.applyFilters(deps, opts.Profile, opts.Only)
	if err != nil {
		return err
	}

	// Save filtered deps for re-applying --only after merge
	var onlyFilteredDeps *config.Deps
	if len(opts.Only) > 0 {
		copy := *deps
		onlyFilteredDeps = &copy
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
		defer func() {
			if err := lockInstance.Release(); err != nil {
				// Log error but don't fail - lock release is best-effort
			}
		}()

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
	defer func() {
		if err := lockInstance.Release(); err != nil {
			// Log error but don't fail - lock release is best-effort
		}
	}()

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
		orchResult, err = uc.processOrchestration(ctx, deps, ws, projectDir, opts.ConfigPath)
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

	// Update global state
	err = uc.updateGlobalState(ctx, deps, ws, composePath, serviceNames)
	if err != nil {
		// Log but don't fail - global state is optional
	}

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

	// Foreground mode: watch + logs (blocks until Ctrl+C)
	if orchResult != nil {
		if opts.Attach {
			// --attach: stream logs without file watching
			streamForeground(ctx, deps, orchResult.detections)
		} else {
			// watch: true services get file watching + logs
			startWatcher(ctx, deps, orchResult.dispatcher, orchResult.detections,
				orchResult.networkName, projectDir)
		}
	}

	return nil
}
