package upcase

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/i18n"
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
func (uc *UseCase) mergeDeps(oldDeps, deps *config.Deps, currentProjectDir string) *config.Deps {
	oldProjectDir := oldDeps.ProjectRoot
	if oldProjectDir == "" {
		oldProjectDir = currentProjectDir
	}

	merged := &config.Deps{
		SchemaVersion: deps.SchemaVersion,
		Workspace:     deps.Workspace,
		Network:       deps.Network,
		Project:       deps.Project,
		Profiles:      deps.Profiles,
		ProjectRoot:   currentProjectDir,
		Services:      make(map[string]config.Service),
		Infra:         make(map[string]config.InfraEntry),
		Env: config.EnvConfig{
			UseGlobal: deps.Env.UseGlobal,
			Files:     mergeSliceUnique(oldDeps.Env.Files, deps.Env.Files),
			Variables: mergeVariables(oldDeps.Env.Variables, deps.Env.Variables),
		},
		Proxy:       deps.Proxy,
		ProxyConfig: deps.ProxyConfig,
		PreHook:     deps.PreHook,
		PostHook:    deps.PostHook,
	}

	// Services: union; resolve each project's volumes with its own project dir, then merge
	for name, svc := range oldDeps.Services {
		merged.Services[name] = cloneService(svc)
		if merged.Services[name].Docker != nil && len(svc.Docker.Volumes) > 0 {
			resolved, _ := uc.deps.DockerRunner.ResolveRelativeVolumes(svc.Docker.Volumes, oldProjectDir)
			merged.Services[name].Docker.Volumes = resolved
		}
	}
	for name, svc := range deps.Services {
		if existing, has := merged.Services[name]; has {
			var oldResolved, newResolved []string
			if existing.Docker != nil && len(existing.Docker.Volumes) > 0 {
				oldResolved, _ = uc.deps.DockerRunner.ResolveRelativeVolumes(existing.Docker.Volumes, oldProjectDir)
			}
			if svc.Docker != nil && len(svc.Docker.Volumes) > 0 {
				newResolved, _ = uc.deps.DockerRunner.ResolveRelativeVolumes(svc.Docker.Volumes, currentProjectDir)
			}
			mergedSvc := cloneService(svc)
			if mergedSvc.Docker != nil {
				mergedSvc.Docker.Volumes = mergeVolumesOnlyNew(oldResolved, newResolved)
			}
			merged.Services[name] = mergedSvc
		} else {
			merged.Services[name] = cloneService(svc)
			if merged.Services[name].Docker != nil && len(svc.Docker.Volumes) > 0 {
				resolved, _ := uc.deps.DockerRunner.ResolveRelativeVolumes(svc.Docker.Volumes, currentProjectDir)
				merged.Services[name].Docker.Volumes = resolved
			}
		}
	}

	for name, entry := range oldDeps.Infra {
		m := cloneInfraEntry(entry)
		if m.Inline != nil && len(m.Inline.Volumes) > 0 {
			resolved, _ := uc.deps.DockerRunner.ResolveRelativeVolumes(m.Inline.Volumes, oldProjectDir)
			m.Inline.Volumes = resolved
		}
		merged.Infra[name] = m
	}
	for name, entry := range deps.Infra {
		if existing, has := merged.Infra[name]; has {
			if entry.Inline != nil && existing.Inline != nil {
				oldResolved, _ := uc.deps.DockerRunner.ResolveRelativeVolumes(existing.Inline.Volumes, oldProjectDir)
				newResolved, _ := uc.deps.DockerRunner.ResolveRelativeVolumes(entry.Inline.Volumes, currentProjectDir)
				m := cloneInfraEntry(entry)
				m.Inline.Volumes = mergeVolumesOnlyNew(oldResolved, newResolved)
				merged.Infra[name] = m
			} else {
				merged.Infra[name] = cloneInfraEntry(entry)
			}
		} else {
			m := cloneInfraEntry(entry)
			if m.Inline != nil && len(m.Inline.Volumes) > 0 {
				resolved, _ := uc.deps.DockerRunner.ResolveRelativeVolumes(m.Inline.Volumes, currentProjectDir)
				m.Inline.Volumes = resolved
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
		Source:         s.Source,
		DependsOn:      append([]string(nil), s.DependsOn...),
		Env:            s.Env,
		Volumes:        append([]string(nil), s.Volumes...),
		Profiles:       append([]string(nil), s.Profiles...),
		Enabled:        s.Enabled,
		Mock:           s.Mock,
		FeatureFlag:    s.FeatureFlag,
		Commands:       s.Commands,
		Watch:          s.Watch,
		Hostname:       s.Hostname,
		Routing:        s.Routing,
		ProxyOverride:  s.ProxyOverride, // BUG: previously missing — proxy override silently dropped on workspace merge
		Port:           s.Port,
		HealthEndpoint: s.HealthEndpoint,
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

func cloneInfraEntry(entry config.InfraEntry) config.InfraEntry {
	out := config.InfraEntry{Path: entry.Path}
	if entry.Inline != nil {
		inf := *entry.Inline
		out.Inline = &config.Infra{
			Name:          inf.Name,
			Image:         inf.Image,
			Tag:           inf.Tag,
			Compose:       append([]string(nil), inf.Compose...),
			Ports:         append([]string(nil), inf.Ports...),
			Volumes:       append([]string(nil), inf.Volumes...),
			IP:            inf.IP,
			Env:           inf.Env,
			Profiles:      append([]string(nil), inf.Profiles...),
			Healthcheck:   inf.Healthcheck,
			Expose:        append([]int(nil), inf.Expose...),
			Publish:       inf.Publish,
			Routing:       inf.Routing,
			ProxyOverride: inf.ProxyOverride,
		}
	}
	return out
}

// checkWorkspaceProjectConflict detects when the same workspace is already running
// from a different project. Returns (result, mergedDeps, error).
// When result is Proceed and mergedDeps is non-nil, the caller must use mergedDeps (merged configs).
// When result is Proceed and mergedDeps is nil, the caller uses current deps (replace).
// currentProjectDir is the absolute path to the current project (where .raioz.json is);
// used to resolve relative volumes per project when merging.
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
	// Same project: state may hold a merged config (multiple projects). Use merge of state + current
	// so we preserve all services/infra from the workspace and update current project's from file.
	if oldDeps.Project.Name == deps.Project.Name {
		return WorkspaceConflictProceed, uc.mergeDeps(oldDeps, deps, currentProjectDir), nil
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

	pref, _ := uc.deps.StateManager.GetWorkspaceProjectPreference(workspaceName)
	if pref != nil && !pref.AlwaysAsk {
		if pref.PreferredProject == deps.Project.Name {
			if pref.MergeWhenPreferred {
				return WorkspaceConflictProceed, uc.mergeDeps(oldDeps, deps, currentProjectDir), nil
			}
			return WorkspaceConflictProceed, nil, nil
		}
		// When preferred is the old project, do not auto-skip: show the menu so the user
		// can choose Merge to add this project to the same workspace (e.g. ui-core + accounts).
	}

	changes, _ := uc.deps.StateManager.CompareDeps(oldDeps, deps)
	changeSummary := uc.deps.StateManager.FormatChanges(changes)

	output.PrintWarning(i18n.T("up.conflict.workspace_already_running"))
	output.PrintInfo("")
	output.PrintInfo(i18n.T("up.conflict.currently_running", oldDeps.Project.Name))
	output.PrintInfo(i18n.T("up.conflict.you_are_running", deps.Project.Name))
	output.PrintInfo("")
	output.PrintInfo(i18n.T("up.conflict.merge_explanation"))
	output.PrintInfo(i18n.T("up.conflict.replace_explanation"))
	output.PrintInfo(changeSummary)
	output.PrintInfo("")
	output.PrintInfo(i18n.T("up.conflict.how_to_proceed"))
	output.PrintInfo(i18n.T("up.conflict.opt_merge"))
	output.PrintInfo(i18n.T("up.conflict.opt_replace"))
	output.PrintInfo(i18n.T("up.conflict.opt_keep"))
	output.PrintInfo(i18n.T("up.conflict.opt_merge_remember"))
	output.PrintInfo(i18n.T("up.conflict.opt_replace_remember"))
	output.PrintInfo(i18n.T("up.conflict.opt_keep_remember"))
	output.PrintInfo(i18n.T("up.conflict.opt_always_ask"))
	output.PrintInfo(i18n.T("up.conflict.opt_cancel_ws"))
	output.PrintPrompt(i18n.T("up.conflict.ws_choice_prompt"))

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
		return WorkspaceConflictProceed, uc.mergeDeps(oldDeps, deps, currentProjectDir), nil
	case 2:
		return WorkspaceConflictProceed, nil, nil
	case 3:
		output.PrintInfo(i18n.T("up.conflict.keeping_current_project"))
		return WorkspaceConflictSkip, nil, nil
	case 4:
		if err := uc.deps.StateManager.SetWorkspaceProjectPreference(workspaceName, state.WorkspaceProjectPreference{
			PreferredProject:   deps.Project.Name,
			AlwaysAsk:          false,
			MergeWhenPreferred: true,
		}); err != nil {
			output.PrintWarning(i18n.T("up.conflict.pref_save_error", err.Error()))
		} else {
			output.PrintSuccess(i18n.T("up.conflict.pref_saved_merge", workspaceName))
		}
		return WorkspaceConflictProceed, uc.mergeDeps(oldDeps, deps, currentProjectDir), nil
	case 5:
		if err := uc.deps.StateManager.SetWorkspaceProjectPreference(workspaceName, state.WorkspaceProjectPreference{
			PreferredProject:   deps.Project.Name,
			AlwaysAsk:          false,
			MergeWhenPreferred: false,
		}); err != nil {
			output.PrintWarning(i18n.T("up.conflict.pref_save_error", err.Error()))
		} else {
			output.PrintSuccess(i18n.T("up.conflict.pref_saved_replace", workspaceName))
		}
		return WorkspaceConflictProceed, nil, nil
	case 6:
		if err := uc.deps.StateManager.SetWorkspaceProjectPreference(workspaceName, state.WorkspaceProjectPreference{
			PreferredProject: oldDeps.Project.Name,
			AlwaysAsk:        false,
		}); err != nil {
			output.PrintWarning(i18n.T("up.conflict.pref_save_error", err.Error()))
		} else {
			output.PrintSuccess(i18n.T("up.conflict.pref_saved_keep", oldDeps.Project.Name, workspaceName))
		}
		output.PrintInfo(i18n.T("up.conflict.keeping_current_project"))
		return WorkspaceConflictSkip, nil, nil
	case 7:
		if err := uc.deps.StateManager.SetWorkspaceProjectPreference(workspaceName, state.WorkspaceProjectPreference{
			AlwaysAsk: true,
		}); err != nil {
			output.PrintWarning(i18n.T("up.conflict.pref_save_error", err.Error()))
		} else {
			output.PrintSuccess(i18n.T("up.conflict.pref_saved_always_ask", workspaceName))
		}
		output.PrintInfo(i18n.T("output.operation_cancelled"))
		return WorkspaceConflictCancel, nil, fmt.Errorf("operation cancelled by user")
	case 8:
		output.PrintInfo(i18n.T("output.operation_cancelled"))
		return WorkspaceConflictCancel, nil, fmt.Errorf("operation cancelled by user")
	default:
		return WorkspaceConflictCancel, nil, fmt.Errorf("invalid choice: %d", choice)
	}
}
