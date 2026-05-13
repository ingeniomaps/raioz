package upcase

import (
	"context"
	"strings"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
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
func (uc *UseCase) mergeDeps(oldDeps, deps *models.Deps, currentProjectDir string) *models.Deps {
	oldProjectDir := oldDeps.ProjectRoot
	if oldProjectDir == "" {
		oldProjectDir = currentProjectDir
	}

	merged := &models.Deps{
		SchemaVersion: deps.SchemaVersion,
		Workspace:     deps.Workspace,
		Network:       deps.Network,
		Project:       deps.Project,
		Profiles:      deps.Profiles,
		ProjectRoot:   currentProjectDir,
		Services:      make(map[string]models.Service),
		Infra:         make(map[string]models.InfraEntry),
		Env: models.EnvConfig{
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
	for k, v := range newV {
		out[k] = v
	}
	return out
}

func cloneService(s models.Service) models.Service {
	out := models.Service{
		Source:          s.Source,
		DependsOn:       append([]string(nil), s.DependsOn...),
		Env:             s.Env,
		Volumes:         append([]string(nil), s.Volumes...),
		Profiles:        append([]string(nil), s.Profiles...),
		Enabled:         s.Enabled,
		Mock:            s.Mock,
		FeatureFlag:     s.FeatureFlag,
		Commands:        s.Commands,
		Watch:           s.Watch,
		Hostname:        s.Hostname,
		HostnameAliases: append([]string(nil), s.HostnameAliases...),
		Routing:         s.Routing,
		ProxyOverride:   s.ProxyOverride, // BUG: previously missing — proxy override silently dropped on workspace merge
		Port:            s.Port,
		HealthEndpoint:  s.HealthEndpoint,
	}
	if s.Docker != nil {
		out.Docker = &models.DockerConfig{
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

func cloneInfraEntry(entry models.InfraEntry) models.InfraEntry {
	out := models.InfraEntry{Path: entry.Path}
	if entry.Inline != nil {
		inf := *entry.Inline
		out.Inline = &models.Infra{
			Name:             inf.Name,
			Image:            inf.Image,
			Tag:              inf.Tag,
			Compose:          append([]string(nil), inf.Compose...),
			Ports:            append([]string(nil), inf.Ports...),
			Volumes:          append([]string(nil), inf.Volumes...),
			IP:               inf.IP,
			Env:              inf.Env,
			Profiles:         append([]string(nil), inf.Profiles...),
			Seed:             append([]string(nil), inf.Seed...),
			Healthcheck:      inf.Healthcheck,
			Expose:           append([]int(nil), inf.Expose...),
			Publish:          inf.Publish,
			Routing:          inf.Routing,
			ProxyOverride:    inf.ProxyOverride,
			Hostname:         inf.Hostname,
			HostnameAliases:  append([]string(nil), inf.HostnameAliases...),
			Project:          inf.Project,
			SiblingProject:   inf.SiblingProject,
			RequiredHostname: inf.RequiredHostname,
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
	deps *models.Deps,
	ws *interfaces.Workspace,
	currentProjectDir string,
) (WorkspaceProjectConflictResult, *models.Deps, error) {
	// ADR-011 Phase 3: the conflict prompt used to compare the current
	// project's services/infra against the previous project's
	// deserialized from .state.json. With the legacy snapshot gone there
	// is no way to materialize the "other project's" config without
	// knowing where its raioz.yaml lives — a piece of data the
	// snapshot uniquely provided.
	//
	// The feature is dropped rather than partially reconstructed:
	// reconstructing it from Docker labels alone would let us name
	// "project P is also up here" but not diff its services, so the
	// merge prompt would be unactionable. Users who hit a multi-project
	// workspace collision can resolve it by running `raioz down
	// --conflicting` (which already sweeps cross-project containers via
	// labels) before re-running `raioz up`. Documented in ADR-011.
	_ = ctx
	_ = deps
	_ = ws
	_ = currentProjectDir
	return WorkspaceConflictProceed, nil, nil
}
