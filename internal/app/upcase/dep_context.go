package upcase

import (
	"context"

	"raioz/internal/detect"
	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/naming"
	"raioz/internal/state"
)

// buildDepContext assembles the ServiceContext the runner gets for one
// dependency. Kept in one place because up is not its only user: dev has
// to stop and restart the very same thing up started, and a context that
// differs in project, compose scope or volumes addresses something else.
func buildDepContext(
	deps *models.Deps,
	name string,
	entry models.InfraEntry,
	detection models.DetectResult,
	networkName, projectDir string,
	portAllocs *PortAllocResult,
) interfaces.ServiceContext {
	// Build image reference + env for the runner.
	envVars := map[string]string{}
	var nameOverride string
	if entry.Inline != nil {
		imageRef := entry.Inline.Image
		if entry.Inline.Tag != "" {
			imageRef += ":" + entry.Inline.Tag
		}
		envVars["RAIOZ_IMAGE"] = imageRef
		if entry.Inline.Env != nil {
			for k, v := range entry.Inline.Env.GetVariables() {
				envVars[k] = v
			}
			for _, filePath := range entry.Inline.Env.GetFilePaths() {
				if filePath != "" {
					envVars["RAIOZ_ENV_FILE"] = filePath
				}
			}
		}
		nameOverride = entry.Inline.Name
	}

	// Deps may be workspace-shared or have an explicit `name:` override —
	// both cases resolved by DepContainer.
	containerName := naming.DepContainer(deps.Project.Name, name, nameOverride)

	// Resolve what ports (if any) this dep should publish to the host.
	// Priority: allocator result (publish: …) → legacy ports: list →
	// nothing at all (internal-only, containers reach it by DNS).
	composePorts := resolveDepPublishPorts(name, entry, portAllocs)

	svcCtx := buildServiceContext(
		name, detection, networkName,
		envVars,
		composePorts,
		nil, // infra has no dependsOn
		containerName,
		"", // no path for images
		deps.Project.Name,
	)
	svcCtx.SharedDep = naming.IsSharedDep(nameOverride) // ADR-050
	if entry.Inline != nil {
		svcCtx.Resources = entry.Inline.Resources
	}

	// ProjectDir anchors relative bind-mount sources against the
	// project's raioz.yaml dir rather than the raioz process cwd.
	if entry.Inline != nil && len(entry.Inline.Volumes) > 0 {
		svcCtx.Volumes = append([]string(nil), entry.Inline.Volumes...)
		svcCtx.ProjectDir = projectDir
	}

	// When the dep declares `compose:`, hand its files + env files
	// straight to ImageRunner. ImageRunner branches on these: if set it
	// uses the user's compose with a network/labels overlay layered on
	// top; if not it generates a minimal compose from the `image:` field
	// (legacy behavior).
	if entry.Inline != nil && len(entry.Inline.Compose) > 0 {
		svcCtx.ExternalComposeFiles = append([]string(nil), entry.Inline.Compose...)
		if entry.Inline.Env != nil {
			for _, f := range entry.Inline.Env.GetFilePaths() {
				if f != "" {
					svcCtx.EnvFilePaths = append(svcCtx.EnvFilePaths, f)
				}
			}
		}
	}
	return svcCtx
}

// DependencyContext rebuilds, from the config alone, the context up used
// to start a dependency. ok is false when the name is not one.
func DependencyContext(
	ctx context.Context, deps *models.Deps, name, projectDir string,
) (svcCtx interfaces.ServiceContext, ok bool) {
	entry, isDep := deps.Infra[name]
	if !isDep {
		return interfaces.ServiceContext{}, false
	}
	detections := BuildDetectionMap(deps)
	portAllocs, err := AllocateHostPortsOwned(deps, detections, runningHostPorts(projectDir))
	if err != nil {
		portAllocs = nil
	}
	_ = ctx
	return buildDepContext(deps, name, entry, detections[name], deps.Network.GetName(), projectDir, portAllocs), true
}

// DevOverrideContext turns a dependency's context into the one that runs
// it from a local path instead. Name, container name, network and ports
// stay the dependency's own, so everything that reached the image reaches
// the local build the same way.
func DevOverrideContext(depCtx interfaces.ServiceContext, localPath string) interfaces.ServiceContext {
	local := depCtx
	local.Path = localPath
	local.Detection = detect.Detect(localPath)
	local.ExternalComposeFiles = nil
	env := make(map[string]string, len(depCtx.EnvVars))
	for k, v := range depCtx.EnvVars {
		if k != "RAIOZ_IMAGE" {
			env[k] = v
		}
	}
	local.EnvVars = env
	return local
}

// loadDevOverrides returns the dependencies promoted with `raioz dev`.
func loadDevOverrides(projectDir string) map[string]models.DevOverride {
	localState, err := state.LoadLocalState(projectDir)
	if err != nil || localState == nil {
		return nil
	}
	return localState.DevOverrides
}
