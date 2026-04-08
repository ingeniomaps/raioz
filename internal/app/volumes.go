package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// VolumesOptions contains options for the Volumes use case
type VolumesOptions struct {
	ConfigPath  string
	ProjectName string
}

// VolumesRemoveOptions contains options for the volumes remove operation
type VolumesRemoveOptions struct {
	ConfigPath  string
	ProjectName string
	All         bool
	Force       bool
	Volumes     []string // specific volume names to remove
}

// VolumeInfo represents a project volume with usage information
type VolumeInfo struct {
	Name      string
	InUseBy   []string // other projects using this volume
	Source    string   // "service" or "infra"
}

// VolumesUseCase handles the "volumes" use case
type VolumesUseCase struct {
	deps *Dependencies
}

// NewVolumesUseCase creates a new VolumesUseCase with injected dependencies
func NewVolumesUseCase(deps *Dependencies) *VolumesUseCase {
	return &VolumesUseCase{deps: deps}
}

// List lists all volumes for a project
func (uc *VolumesUseCase) List(ctx context.Context, opts VolumesOptions) error {
	projectName, ws, err := uc.resolveProject(ctx, opts.ConfigPath, opts.ProjectName)
	if err != nil {
		return err
	}

	volumes, err := uc.collectVolumes(ctx, projectName, ws, opts.ConfigPath)
	if err != nil {
		return err
	}

	if len(volumes) == 0 {
		output.PrintInfo(i18n.T("output.no_volumes_found"))
		return nil
	}

	output.PrintSectionHeader(i18n.T("output.volumes_list_header", projectName))

	for _, vol := range volumes {
		if len(vol.InUseBy) > 0 {
			output.PrintInfo(i18n.T("output.volume_list_shared", vol.Name, vol.Source, strings.Join(vol.InUseBy, ", ")))
		} else {
			output.PrintInfo(i18n.T("output.volume_list_item", vol.Name, vol.Source))
		}
	}

	return nil
}

// Remove removes volumes for a project
func (uc *VolumesUseCase) Remove(ctx context.Context, opts VolumesRemoveOptions) error {
	projectName, ws, err := uc.resolveProject(ctx, opts.ConfigPath, opts.ProjectName)
	if err != nil {
		return err
	}

	volumes, err := uc.collectVolumes(ctx, projectName, ws, opts.ConfigPath)
	if err != nil {
		return err
	}

	if len(volumes) == 0 {
		output.PrintInfo(i18n.T("output.no_volumes_found"))
		return nil
	}

	// Filter volumes to remove
	var toRemove []VolumeInfo
	var inUse []VolumeInfo

	if len(opts.Volumes) > 0 {
		// Specific volumes requested
		toRemove, inUse, err = uc.filterSpecificVolumes(volumes, opts.Volumes)
		if err != nil {
			return err
		}
	} else if opts.All {
		// All volumes
		for _, vol := range volumes {
			if len(vol.InUseBy) > 0 {
				inUse = append(inUse, vol)
			} else {
				toRemove = append(toRemove, vol)
			}
		}
	} else {
		return errors.New(
			errors.ErrCodeInvalidField,
			i18n.T("error.volumes_no_target"),
		)
	}

	// Show volumes in use by other projects
	if len(inUse) > 0 {
		output.PrintWarning(i18n.T("output.volumes_in_use_warning", len(inUse)))
		for _, vol := range inUse {
			output.PrintInfo(i18n.T("output.volume_detail", vol.Name, strings.Join(vol.InUseBy, ", ")))
		}
		fmt.Println()
	}

	if len(toRemove) == 0 {
		output.PrintInfo(i18n.T("output.all_volumes_in_use"))
		return nil
	}

	// Show what will be removed
	output.PrintInfo(i18n.T("output.found_volumes_removable", len(toRemove)))
	for _, vol := range toRemove {
		output.PrintInfo(i18n.T("output.volume_item", vol.Name))
	}
	fmt.Println()

	// Ask for confirmation
	if !opts.Force {
		fmt.Print(i18n.T("output.confirm_remove_volumes_prompt"))
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return errors.New(
				errors.ErrCodeWorkspaceError,
				i18n.T("error.read_input"),
			).WithError(err)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "yes" && response != "y" {
			output.PrintInfo(i18n.T("output.volumes_cancelled"))
			return nil
		}
	}

	// Remove volumes
	return uc.removeVolumes(ctx, toRemove)
}

// resolveProject resolves project name and workspace
func (uc *VolumesUseCase) resolveProject(ctx context.Context, configPath string, projectName string) (string, *interfaces.Workspace, error) {
	var workspaceName string
	if projectName == "" {
		deps, _, _ := uc.deps.ConfigLoader.LoadDeps(configPath)
		if deps != nil {
			projectName = deps.Project.Name
			workspaceName = deps.GetWorkspaceName()
		} else {
			return "", nil, errors.New(
				errors.ErrCodeInvalidConfig,
				i18n.T("error.no_project"),
			).WithSuggestion(i18n.T("error.no_project_suggestion"))
		}
	} else {
		deps, _, _ := uc.deps.ConfigLoader.LoadDeps(configPath)
		if deps != nil && deps.Project.Name == projectName {
			workspaceName = deps.GetWorkspaceName()
		} else {
			workspaceName = projectName
		}
	}

	ws, err := uc.deps.Workspace.Resolve(workspaceName)
	if err != nil {
		return "", nil, errors.New(
			errors.ErrCodeWorkspaceError,
			i18n.T("error.workspace_resolve"),
		).WithContext("project", projectName).WithError(err)
	}

	return projectName, ws, nil
}

// collectVolumes collects and normalizes all project volumes
func (uc *VolumesUseCase) collectVolumes(ctx context.Context, projectName string, ws *interfaces.Workspace, configPath string) ([]VolumeInfo, error) {
	var serviceVolumes []string
	var infraVolumes []string
	workspaceName := projectName

	// Try state first, then config
	if uc.deps.StateManager.Exists(ws) {
		stateDeps, err := uc.deps.StateManager.Load(ws)
		if err == nil {
			workspaceName = stateDeps.GetWorkspaceName()
			serviceVolumes, infraVolumes = extractVolumesFromDeps(stateDeps)
		}
	}

	if len(serviceVolumes) == 0 && len(infraVolumes) == 0 {
		deps, _, _ := uc.deps.ConfigLoader.LoadDeps(configPath)
		if deps != nil {
			workspaceName = deps.GetWorkspaceName()
			serviceVolumes, infraVolumes = extractVolumesFromDeps(deps)
		}
	}

	if len(serviceVolumes) == 0 && len(infraVolumes) == 0 {
		return nil, nil
	}

	// Normalize and deduplicate
	seen := make(map[string]bool)
	var volumes []VolumeInfo

	// Service volumes normalized with project name
	serviceNamed, err := uc.deps.DockerRunner.ExtractNamedVolumes(serviceVolumes)
	if err != nil {
		return nil, errors.New(errors.ErrCodeDockerNotRunning, i18n.T("error.volumes_extract_services")).WithError(err)
	}
	for _, volName := range serviceNamed {
		normalized, err := uc.deps.DockerRunner.NormalizeVolumeName(projectName, volName)
		if err != nil {
			logging.WarnWithContext(ctx, "Failed to normalize service volume", "volume", volName, "error", err.Error())
			continue
		}
		if !seen[normalized] {
			seen[normalized] = true
			volumes = append(volumes, VolumeInfo{Name: normalized, Source: "service"})
		}
	}

	// Infra volumes normalized with workspace name
	infraNamed, err := uc.deps.DockerRunner.ExtractNamedVolumes(infraVolumes)
	if err != nil {
		return nil, errors.New(errors.ErrCodeDockerNotRunning, i18n.T("error.volumes_extract_infra")).WithError(err)
	}
	for _, volName := range infraNamed {
		normalized, err := uc.deps.DockerRunner.NormalizeVolumeName(workspaceName, volName)
		if err != nil {
			logging.WarnWithContext(ctx, "Failed to normalize infra volume", "volume", volName, "error", err.Error())
			continue
		}
		if !seen[normalized] {
			seen[normalized] = true
			volumes = append(volumes, VolumeInfo{Name: normalized, Source: "infra"})
		}
	}

	// Check usage by other projects
	baseDir := uc.deps.Workspace.GetBaseDirFromWorkspace(ws)
	for i, vol := range volumes {
		projects, err := uc.deps.DockerRunner.GetVolumeProjects(vol.Name, baseDir)
		if err != nil {
			continue
		}
		var others []string
		for _, p := range projects {
			if p != projectName {
				others = append(others, p)
			}
		}
		volumes[i].InUseBy = others
	}

	return volumes, nil
}

// filterSpecificVolumes filters volumes by name
func (uc *VolumesUseCase) filterSpecificVolumes(volumes []VolumeInfo, names []string) (toRemove []VolumeInfo, inUse []VolumeInfo, err error) {
	volMap := make(map[string]VolumeInfo)
	for _, vol := range volumes {
		volMap[vol.Name] = vol
	}

	for _, name := range names {
		vol, ok := volMap[name]
		if !ok {
			return nil, nil, errors.New(
				errors.ErrCodeInvalidField,
				i18n.T("error.volume_not_found", name),
			).WithContext("volume", name)
		}
		if len(vol.InUseBy) > 0 {
			inUse = append(inUse, vol)
		} else {
			toRemove = append(toRemove, vol)
		}
	}
	return toRemove, inUse, nil
}

// removeVolumes removes the given volumes
func (uc *VolumesUseCase) removeVolumes(ctx context.Context, volumes []VolumeInfo) error {
	output.PrintProgress(i18n.T("output.removing_volumes", len(volumes)))
	removedCount := 0
	failedCount := 0

	for _, vol := range volumes {
		if err := uc.deps.DockerRunner.RemoveVolumeWithContext(ctx, vol.Name); err != nil {
			logging.WarnWithContext(ctx, "Failed to remove volume", "volume", vol.Name, "error", err.Error())
			output.PrintWarning(i18n.T("output.failed_remove_volume", vol.Name, err))
			failedCount++
		} else {
			output.PrintSuccess(i18n.T("output.removed_volume", vol.Name))
			removedCount++
		}
	}

	fmt.Println()
	if removedCount > 0 {
		output.PrintSuccess(i18n.T("output.volumes_removed_success", removedCount))
	}
	if failedCount > 0 {
		output.PrintWarning(i18n.T("output.volumes_removed_failed", failedCount))
	}
	return nil
}

// extractVolumesFromDeps extracts service and infra volumes from deps
func extractVolumesFromDeps(deps *config.Deps) (serviceVolumes []string, infraVolumes []string) {
	for _, svc := range deps.Services {
		if svc.Docker != nil {
			serviceVolumes = append(serviceVolumes, svc.Docker.Volumes...)
		}
	}
	for _, entry := range deps.Infra {
		if entry.Inline != nil {
			infraVolumes = append(infraVolumes, entry.Inline.Volumes...)
		}
	}
	return
}
