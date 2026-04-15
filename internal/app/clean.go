package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// CleanOptions contains options for the Clean use case
type CleanOptions struct {
	ConfigPath  string
	ProjectName string
	All         bool
	Images      bool
	Volumes     bool
	Networks    bool
	DryRun      bool
	Force       bool
}

// CleanUseCase handles the "clean" use case
type CleanUseCase struct {
	deps *Dependencies
}

// NewCleanUseCase creates a new CleanUseCase with injected dependencies
func NewCleanUseCase(deps *Dependencies) *CleanUseCase {
	return &CleanUseCase{deps: deps}
}

// Execute executes the clean use case
func (uc *CleanUseCase) Execute(ctx context.Context, opts CleanOptions) error {
	projectName := opts.ProjectName
	if !opts.All && projectName == "" {
		deps, warnings, _ := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
		for _, w := range warnings {
			output.PrintWarning(w)
		}
		if deps != nil {
			projectName = deps.Project.Name
		} else {
			return errors.New(
				errors.ErrCodeInvalidConfig,
				i18n.T("error.no_project"),
			).WithSuggestion(i18n.T("error.no_project_suggestion"))
		}
	}

	var actions []string

	// Clean projects
	if opts.All {
		baseDir, err := uc.deps.Workspace.GetBaseDir()
		if err != nil {
			return errors.New(
				errors.ErrCodeWorkspaceError,
				i18n.T("error.base_dir"),
			).WithSuggestion(i18n.T("error.base_dir_suggestion")).WithError(err)
		}
		logging.Info("Cleaning all projects")
		projectActions, err := uc.deps.DockerRunner.CleanAllProjectsWithContext(ctx, baseDir, opts.DryRun)
		if err != nil {
			return errors.New(
				errors.ErrCodeDockerNotRunning,
				i18n.T("error.clean_all_failed"),
			).WithSuggestion(i18n.T("error.clean_all_suggestion")).WithError(err)
		}
		actions = append(actions, projectActions...)
	} else {
		projectActions, err := uc.cleanProject(ctx, projectName, opts)
		if err != nil {
			return err
		}
		actions = append(actions, projectActions...)
	}

	// Clean images
	if opts.Images {
		logging.Info("Cleaning unused images")
		imageActions, err := uc.deps.DockerRunner.CleanUnusedImagesWithContext(ctx, opts.DryRun)
		if err != nil {
			return errors.New(
				errors.ErrCodeDockerNotRunning,
				i18n.T("error.clean_images_failed"),
			).WithSuggestion(i18n.T("error.clean_images_suggestion")).WithError(err)
		}
		actions = append(actions, imageActions...)
	}

	// Clean volumes
	if opts.Volumes {
		volumeActions, err := uc.cleanVolumes(ctx, opts)
		if err != nil {
			return err
		}
		if volumeActions == nil {
			return nil // User cancelled
		}
		actions = append(actions, volumeActions...)
	}

	// Clean networks
	if opts.Networks {
		logging.Info("Cleaning unused networks")
		networkActions, err := uc.deps.DockerRunner.CleanUnusedNetworksWithContext(ctx, opts.DryRun)
		if err != nil {
			return errors.New(
				errors.ErrCodeNetworkError,
				i18n.T("error.clean_networks_failed"),
			).WithSuggestion(i18n.T("error.clean_networks_suggestion")).WithError(err)
		}
		actions = append(actions, networkActions...)
	}

	// Display actions
	if opts.DryRun {
		output.PrintSectionHeader(i18n.T("output.dry_run_header"))
	} else {
		output.PrintSectionHeader(i18n.T("output.actions_header"))
	}

	if len(actions) == 0 {
		output.PrintInfo(i18n.T("output.nothing_to_clean"))
	} else {
		output.PrintList(actions, 0)
	}

	return nil
}

func (uc *CleanUseCase) cleanProject(ctx context.Context, projectName string, opts CleanOptions) ([]string, error) {
	ws, err := uc.deps.Workspace.Resolve(projectName)
	if err != nil {
		return nil, errors.New(
			errors.ErrCodeWorkspaceError,
			i18n.T("error.workspace_resolve"),
		).WithSuggestion(i18n.T("error.workspace_resolve_suggestion")).WithError(err)
	}

	composePath := uc.deps.Workspace.GetComposePath(ws)
	logging.Info("Cleaning project", "project", projectName)

	projectActions, err := uc.deps.DockerRunner.CleanProjectWithContext(ctx, composePath, opts.DryRun)
	if err != nil {
		return nil, errors.New(
			errors.ErrCodeDockerNotRunning,
			i18n.T("error.clean_project_failed"),
		).WithSuggestion(i18n.T("error.clean_project_suggestion")).WithError(err)
	}

	// Remove state file if exists
	statePath := uc.deps.Workspace.GetStatePath(ws)
	if _, err := os.Stat(statePath); err == nil {
		if opts.DryRun {
			projectActions = append(projectActions, fmt.Sprintf("Would remove state file: %s", statePath))
		} else {
			if err := os.Remove(statePath); err != nil {
				projectActions = append(projectActions, fmt.Sprintf("Failed to remove state file: %v", err))
			} else {
				projectActions = append(projectActions, fmt.Sprintf("Removed state file: %s", statePath))
			}
		}
	}

	return projectActions, nil
}

func (uc *CleanUseCase) cleanVolumes(ctx context.Context, opts CleanOptions) ([]string, error) {
	logging.Info("Cleaning unused volumes")

	if !opts.Force && !opts.DryRun {
		logging.Warn("Volume removal requires confirmation. Use --force to proceed.")
		output.PrintPrompt(i18n.T("output.confirm_remove_volumes"))
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return nil, errors.New(
				errors.ErrCodeInternalError,
				i18n.T("error.read_input"),
			).WithError(err)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "yes" && response != "y" {
			logging.Info("Volume cleanup cancelled")
			return nil, nil // nil signals cancellation
		}
	}

	volumeActions, err := uc.deps.DockerRunner.CleanUnusedVolumesWithContext(ctx, opts.DryRun, opts.Force || opts.DryRun)
	if err != nil {
		return nil, errors.New(
			errors.ErrCodeVolumeError,
			i18n.T("error.clean_volumes_failed"),
		).WithSuggestion(i18n.T("error.clean_volumes_suggestion")).WithError(err)
	}
	return volumeActions, nil
}
