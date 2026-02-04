package upcase

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"raioz/internal/config"
	"raioz/internal/docker"
	"raioz/internal/domain/interfaces"
	"raioz/internal/output"
	"raioz/internal/state"
)

// WorkspaceProjectConflictResult is the result of resolving a workspace-vs-project conflict
type WorkspaceProjectConflictResult int

const (
	WorkspaceConflictProceed WorkspaceProjectConflictResult = iota
	WorkspaceConflictSkip
	WorkspaceConflictCancel
)

// mergeDeps builds a config that combines the already-running project (oldDeps) with the
// current project (deps). Volumes are resolved with the correct project dir each: old volumes
// with oldDeps.ProjectRoot, new volumes with currentProjectDir, so each project's relative paths
// become the right absolute paths (no single projectDir applied to all).
func mergeDeps(oldDeps, deps *config.Deps, currentProjectDir string) *config.Deps {
	oldProjectDir := oldDeps.ProjectRoot
	if oldProjectDir == "" {
		oldProjectDir = currentProjectDir
	}

	merged := &config.Deps{
		SchemaVersion: deps.SchemaVersion,
		Workspace:     deps.Workspace,
		Network:       deps.Network,
		Project:       deps.Project,
		ProjectRoot:   currentProjectDir,
		Services:      make(map[string]config.Service),
		Infra:         make(map[string]config.Infra),
		Env: config.EnvConfig{
			UseGlobal: deps.Env.UseGlobal,
			Files:     mergeSliceUnique(oldDeps.Env.Files, deps.Env.Files),
			Variables: mergeVariables(oldDeps.Env.Variables, deps.Env.Variables),
		},
	}

	// Services: union; resolve each project's volumes with its own project dir, then merge
	for name, svc := range oldDeps.Services {
		merged.Services[name] = cloneService(svc)
		if merged.Services[name].Docker != nil && len(svc.Docker.Volumes) > 0 {
			resolved, _ := docker.ResolveRelativeVolumes(svc.Docker.Volumes, oldProjectDir)
			merged.Services[name].Docker.Volumes = resolved
		}
	}
	for name, svc := range deps.Services {
		if existing, has := merged.Services[name]; has {
			var oldResolved, newResolved []string
			if existing.Docker != nil && len(existing.Docker.Volumes) > 0 {
				oldResolved, _ = docker.ResolveRelativeVolumes(existing.Docker.Volumes, oldProjectDir)
			}
			if svc.Docker != nil && len(svc.Docker.Volumes) > 0 {
				newResolved, _ = docker.ResolveRelativeVolumes(svc.Docker.Volumes, currentProjectDir)
			}
			mergedSvc := cloneService(svc)
			if mergedSvc.Docker != nil {
				mergedSvc.Docker.Volumes = mergeVolumesOnlyNew(oldResolved, newResolved)
			}
			merged.Services[name] = mergedSvc
		} else {
			merged.Services[name] = cloneService(svc)
			if merged.Services[name].Docker != nil && len(svc.Docker.Volumes) > 0 {
				resolved, _ := docker.ResolveRelativeVolumes(svc.Docker.Volumes, currentProjectDir)
				merged.Services[name].Docker.Volumes = resolved
			}
		}
	}

	for name, inf := range oldDeps.Infra {
		m := cloneInfra(inf)
		if len(inf.Volumes) > 0 {
			resolved, _ := docker.ResolveRelativeVolumes(inf.Volumes, oldProjectDir)
			m.Volumes = resolved
		}
		merged.Infra[name] = m
	}
	for name, inf := range deps.Infra {
		if existing, has := merged.Infra[name]; has {
			oldResolved, _ := docker.ResolveRelativeVolumes(existing.Volumes, oldProjectDir)
			newResolved, _ := docker.ResolveRelativeVolumes(inf.Volumes, currentProjectDir)
			m := cloneInfra(inf)
			m.Volumes = mergeVolumesOnlyNew(oldResolved, newResolved)
			merged.Infra[name] = m
		} else {
			m := cloneInfra(inf)
			if len(inf.Volumes) > 0 {
				resolved, _ := docker.ResolveRelativeVolumes(inf.Volumes, currentProjectDir)
				m.Volumes = resolved
			}
			merged.Infra[name] = m
		}
	}

	return merged
}

func mergeSliceUnique(a, b []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func mergeVariables(oldV, newV map[string]string) map[string]string {
	out := make(map[string]string)
	for k, v := range oldV {
		out[k] = v
	}
	if newV != nil {
		for k, v := range newV {
			out[k] = v
		}
	}
	return out
}

func cloneService(s config.Service) config.Service {
	out := config.Service{
		Source:      s.Source,
		DependsOn:   append([]string(nil), s.DependsOn...),
		Env:         s.Env,
		Volumes:     append([]string(nil), s.Volumes...),
		Profiles:    append([]string(nil), s.Profiles...),
		Enabled:     s.Enabled,
		Mock:        s.Mock,
		FeatureFlag: s.FeatureFlag,
		Commands:    s.Commands,
	}
	if s.Docker != nil {
		out.Docker = &config.DockerConfig{
			Mode:       s.Docker.Mode,
			Ports:      append([]string(nil), s.Docker.Ports...),
			Volumes:    append([]string(nil), s.Docker.Volumes...),
			DependsOn:  append([]string(nil), s.Docker.DependsOn...),
			Dockerfile: s.Docker.Dockerfile,
			Command:    s.Docker.Command,
			Runtime:    s.Docker.Runtime,
			IP:         s.Docker.IP,
			EnvVolume:  s.Docker.EnvVolume,
		}
	}
	return out
}

// volumeContainerPath returns the path inside the container (destination) for deduplication.
// Format: "host:container" or "host:container:ro" -> returns "container" or "container:ro"
func volumeContainerPath(vol string) string {
	parts := strings.SplitN(vol, ":", 3)
	if len(parts) >= 2 {
		return strings.Join(parts[1:], ":")
	}
	return vol
}

// mergeVolumesOnlyNew adds to base only volumes from add that mount a container path not already in base
// (avoids duplicate mounts to the same path and repeated entries)
func mergeVolumesOnlyNew(base, add []string) []string {
	containerPaths := make(map[string]bool)
	out := make([]string, 0, len(base)+len(add))
	for _, v := range base {
		cp := volumeContainerPath(v)
		if !containerPaths[cp] {
			containerPaths[cp] = true
			out = append(out, v)
		}
	}
	for _, v := range add {
		cp := volumeContainerPath(v)
		if !containerPaths[cp] {
			containerPaths[cp] = true
			out = append(out, v)
		}
	}
	return out
}

func cloneInfra(inf config.Infra) config.Infra {
	out := config.Infra{
		Image:   inf.Image,
		Tag:     inf.Tag,
		Ports:   append([]string(nil), inf.Ports...),
		Volumes: append([]string(nil), inf.Volumes...),
		IP:      inf.IP,
		Env:     inf.Env,
	}
	return out
}

// checkWorkspaceProjectConflict detects when the same workspace is already running
// from a different project. Returns (result, mergedDeps, error).
// When result is Proceed and mergedDeps is non-nil, the caller must use mergedDeps (merged configs).
// When result is Proceed and mergedDeps is nil, the caller uses current deps (replace).
// currentProjectDir is the absolute path to the current project (where .raioz.json is); used to resolve relative volumes per project when merging.
func (uc *UseCase) checkWorkspaceProjectConflict(
	ctx context.Context,
	deps *config.Deps,
	ws *interfaces.Workspace,
	currentProjectDir string,
) (WorkspaceProjectConflictResult, *config.Deps, error) {
	workspaceName := deps.GetWorkspaceName()

	oldDeps, err := uc.deps.StateManager.Load(ws)
	if err != nil {
		return WorkspaceConflictProceed, nil, nil
	}
	if oldDeps == nil {
		return WorkspaceConflictProceed, nil, nil
	}
	if oldDeps.Project.Name == deps.Project.Name {
		return WorkspaceConflictProceed, nil, nil
	}

	overlap := false
	for name := range deps.Services {
		if _, has := oldDeps.Services[name]; has {
			overlap = true
			break
		}
	}
	if !overlap {
		for name := range deps.Infra {
			if _, has := oldDeps.Infra[name]; has {
				overlap = true
				break
			}
		}
	}
	if !overlap {
		return WorkspaceConflictProceed, nil, nil
	}

	pref, _ := state.GetWorkspaceProjectPreference(workspaceName)
	if pref != nil && !pref.AlwaysAsk {
		if pref.PreferredProject == deps.Project.Name {
			if pref.MergeWhenPreferred {
				return WorkspaceConflictProceed, mergeDeps(oldDeps, deps, currentProjectDir), nil
			}
			return WorkspaceConflictProceed, nil, nil
		}
		if pref.PreferredProject == oldDeps.Project.Name {
			output.PrintInfo(fmt.Sprintf("Workspace '%s' is already running project '%s' (saved preference). Skipping.", workspaceName, oldDeps.Project.Name))
			return WorkspaceConflictSkip, nil, nil
		}
	}

	changes, _ := state.CompareDeps(oldDeps, deps)
	changeSummary := state.FormatChanges(changes)

	output.PrintWarning("This workspace is already running a different project")
	output.PrintInfo("")
	output.PrintInfo(fmt.Sprintf("  Currently running: project '%s'", oldDeps.Project.Name))
	output.PrintInfo(fmt.Sprintf("  You are running:  project '%s'", deps.Project.Name))
	output.PrintInfo("")
	output.PrintInfo("You can merge both configs (volumes and variables from both apply together)")
	output.PrintInfo("or use only this project's config (replace). Summary of differences:")
	output.PrintInfo(changeSummary)
	output.PrintInfo("")
	output.PrintInfo("How do you want to proceed?")
	output.PrintInfo("  [1] Merge: keep current deployment and add this project's volumes and variables (both configs together)")
	output.PrintInfo("  [2] Replace: use only this project's config (current deployment will match this project only)")
	output.PrintInfo("  [3] Keep the already running project as-is (do nothing)")
	output.PrintInfo("  [4] Merge and remember for this workspace")
	output.PrintInfo("  [5] Replace and remember for this workspace")
	output.PrintInfo("  [6] Keep the other project and remember for this workspace")
	output.PrintInfo("  [7] Always ask me next time (do not remember)")
	output.PrintInfo("  [8] Cancel")
	fmt.Print("\nYour choice [1-8]: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return WorkspaceConflictCancel, nil, fmt.Errorf("failed to read user response: %w", err)
	}
	response = strings.TrimSpace(response)
	var choice int
	if _, err := fmt.Sscanf(response, "%d", &choice); err != nil || choice < 1 || choice > 8 {
		return WorkspaceConflictCancel, nil, fmt.Errorf("invalid choice: %s", response)
	}

	switch choice {
	case 1:
		return WorkspaceConflictProceed, mergeDeps(oldDeps, deps, currentProjectDir), nil
	case 2:
		return WorkspaceConflictProceed, nil, nil
	case 3:
		output.PrintInfo("Keeping current project. Nothing changed.")
		return WorkspaceConflictSkip, nil, nil
	case 4:
		if err := state.SetWorkspaceProjectPreference(workspaceName, state.WorkspaceProjectPreference{
			PreferredProject:   deps.Project.Name,
			AlwaysAsk:          false,
			MergeWhenPreferred: true,
		}); err != nil {
			output.PrintWarning("Could not save preference: " + err.Error())
		} else {
			output.PrintSuccess("Preference saved: merge with this project for workspace '" + workspaceName + "'")
		}
		return WorkspaceConflictProceed, mergeDeps(oldDeps, deps, currentProjectDir), nil
	case 5:
		if err := state.SetWorkspaceProjectPreference(workspaceName, state.WorkspaceProjectPreference{
			PreferredProject:   deps.Project.Name,
			AlwaysAsk:          false,
			MergeWhenPreferred: false,
		}); err != nil {
			output.PrintWarning("Could not save preference: " + err.Error())
		} else {
			output.PrintSuccess("Preference saved: use this project (replace) for workspace '" + workspaceName + "'")
		}
		return WorkspaceConflictProceed, nil, nil
	case 6:
		if err := state.SetWorkspaceProjectPreference(workspaceName, state.WorkspaceProjectPreference{
			PreferredProject: oldDeps.Project.Name,
			AlwaysAsk:        false,
		}); err != nil {
			output.PrintWarning("Could not save preference: " + err.Error())
		} else {
			output.PrintSuccess("Preference saved: keep project '" + oldDeps.Project.Name + "' for workspace '" + workspaceName + "'")
		}
		output.PrintInfo("Keeping current project. Nothing changed.")
		return WorkspaceConflictSkip, nil, nil
	case 7:
		if err := state.SetWorkspaceProjectPreference(workspaceName, state.WorkspaceProjectPreference{
			AlwaysAsk: true,
		}); err != nil {
			output.PrintWarning("Could not save preference: " + err.Error())
		} else {
			output.PrintSuccess("Next time you will be asked again for workspace '" + workspaceName + "'")
		}
		output.PrintInfo("Operation cancelled.")
		return WorkspaceConflictCancel, nil, fmt.Errorf("operation cancelled by user")
	case 8:
		output.PrintInfo("Operation cancelled.")
		return WorkspaceConflictCancel, nil, fmt.Errorf("operation cancelled by user")
	default:
		return WorkspaceConflictCancel, nil, fmt.Errorf("invalid choice: %d", choice)
	}
}
