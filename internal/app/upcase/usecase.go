package upcase

import (
	"context"
	"path/filepath"
	"time"

	"raioz/internal/docker"
	"raioz/internal/domain/interfaces"
	"raioz/internal/env"
	"raioz/internal/errors"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// Options contains options for the Up use case
type Options struct {
	ConfigPath   string
	Profile      string
	ForceReclone bool
	DryRun       bool
}

// Dependencies contains the dependencies needed by the up use case
type Dependencies struct {
	ConfigLoader   interfaces.ConfigLoader
	Validator      interfaces.Validator
	DockerRunner   interfaces.DockerRunner
	GitRepository  interfaces.GitRepository
	Workspace      interfaces.WorkspaceManager
	StateManager   interfaces.StateManager
	LockManager    interfaces.LockManager
}

// UseCase handles the "up" use case - starting a project
type UseCase struct {
	deps *Dependencies
}

// NewUseCase creates a new UpUseCase with injected dependencies
func NewUseCase(deps *Dependencies) *UseCase {
	return &UseCase{
		deps: deps,
	}
}

// Execute executes the up use case
func (uc *UseCase) Execute(ctx context.Context, opts Options) error {
	startTime := time.Now()

	// Bootstrap: context, logging, config loading
	ctx, deps, ws, err := uc.bootstrap(ctx, opts.ConfigPath)
	if err != nil {
		return err
	}

	// Get project directory (where .raioz.json is located)
	projectDir, err := filepath.Abs(filepath.Dir(opts.ConfigPath))
	if err != nil {
		return errors.New(
			errors.ErrCodeWorkspaceError,
			"Failed to get project directory",
		).WithError(err)
	}

	// Write global env variables from env.variables to global.env
	// This must happen before filters to ensure variables are available
	if err := env.WriteGlobalEnvVariables(ws, deps); err != nil {
		return errors.New(
			errors.ErrCodeWorkspaceError,
			"Failed to write global environment variables",
		).WithSuggestion(
			"Check that you have write permissions for the env directory.",
		).WithError(err)
	}

	// Resolve project.env (if project.env is ["."], uses .env in project directory as primary)
	projectEnvPath, err := env.ResolveProjectEnv(ws, deps, projectDir)
	if err != nil {
		return errors.New(
			errors.ErrCodeWorkspaceError,
			"Failed to resolve project environment",
		).WithSuggestion(
			"Check that you have write permissions for the env directory.",
		).WithError(err)
	}

	// Filters: profile, feature flags, ignore list
	deps, err = uc.applyFilters(deps, opts.Profile)
	if err != nil {
		return err
	}

	// Check what we have: services, infra, or project commands
	hasServices := len(deps.Services) > 0
	hasInfra := len(deps.Infra) > 0
	hasProjectCommands := deps.Project.Commands != nil && (
		deps.Project.Commands.Up != "" ||
		(deps.Project.Commands.Dev != nil && deps.Project.Commands.Dev.Up != "") ||
		(deps.Project.Commands.Prod != nil && deps.Project.Commands.Prod.Up != ""))

	// If only project commands, skip services/infra processing
	onlyProjectCommands := !hasServices && !hasInfra && hasProjectCommands

	// Validation: validate.All, permissions, ports, dependency conflicts
	// Skip some validations if we only have project commands
	err = uc.validate(ctx, deps, ws, opts.DryRun)
	if err != nil {
		return err
	}

	// If only project commands, execute them directly and return
	if onlyProjectCommands {
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
		return nil
	}

	// Git: clone/update repos, handle branch changes
	err = uc.processGitRepos(ctx, deps, ws, oldDeps, opts.ForceReclone, projectDir)
	if err != nil {
		return err
	}

	// Generate .env files from templates for all services (after cloning, before starting)
	output.PrintProgress("Generating .env files from templates...")
	err = uc.generateEnvFilesFromTemplates(ctx, deps, ws, projectEnvPath, projectDir)
	if err != nil {
		// Log but don't fail - template generation is optional
		logging.WarnWithContext(ctx, "Some .env files could not be generated from templates", "error", err.Error())
		output.PrintProgressError("Some .env files could not be generated from templates")
	} else {
		output.PrintProgressDone(".env files generated from templates")
	}

	// Host services: start services that run directly on the host (without Docker)
	hostProcessInfo, err := uc.processHostServices(ctx, deps, ws, projectDir)
	if err != nil {
		return err
	}

	// Docker prepare: images, network, volumes
	err = uc.prepareDockerResources(ctx, deps, ws)
	if err != nil {
		// If Docker prepare fails, stop host services that were started
		if len(hostProcessInfo) > 0 {
			_ = uc.stopHostServices(ctx, hostProcessInfo)
		}
		return err
	}

	// Compose: generate compose, docker.Up
	composePath, serviceNames, infraNames, err := uc.processCompose(ctx, deps, ws, projectDir)
	if err != nil {
		// If compose fails, stop host services that were started
		if len(hostProcessInfo) > 0 {
			_ = uc.stopHostServices(ctx, hostProcessInfo)
		}
		return err
	}

	// State: save state, root config, drift detection, audit
	err = uc.saveState(ctx, deps, ws, composePath, serviceNames, addedServices, assistedServicesMap)
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
		output.PrintProgress("Waiting for services to be healthy before executing project command...")
		if err := docker.WaitForServicesHealthy(ctx, composePath, serviceNames, infraNames, deps.Project.Name); err != nil {
			logging.WarnWithContext(ctx, "Failed to wait for services to be healthy", "error", err.Error())
			output.PrintWarning("Some services may not be healthy yet, proceeding with project command anyway")
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

	// Final summary
	uc.showSummary(ctx, deps, serviceNames, infraNames, startTime)

	return nil
}
