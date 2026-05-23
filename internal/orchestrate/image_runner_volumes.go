package orchestrate

import (
	"fmt"

	"raioz/internal/docker"
	"raioz/internal/domain/interfaces"
)

// applyDepVolumes resolves the dependency's `volumes:` declaration into the
// compose `services.<name>.volumes:` block and returns the map of
// original→project-prefixed names for any named volumes encountered. The
// map drives the top-level `volumes:` declaration in declareTopLevelVolumes.
//
// Bind mounts resolve relative source paths against svc.ProjectDir (the
// project's raioz.yaml dir), NOT the raioz process cwd — without that
// anchor, `./infra/foo.yml` would land wherever the user invoked raioz
// from. Named volumes are prefixed with the project name to keep them
// scoped per project; workspace-shared named volumes remain a follow-up.
//
// Returns an empty map when the dep declares no volumes so callers can
// pass the result straight to declareTopLevelVolumes without a nil check.
func applyDepVolumes(svc interfaces.ServiceContext, service map[string]any) (map[string]string, error) {
	namedVolumeMap := map[string]string{}
	if len(svc.Volumes) == 0 {
		return namedVolumeMap, nil
	}
	resolved, err := docker.ResolveRelativeVolumes(svc.Volumes, svc.ProjectDir)
	if err != nil {
		return nil, fmt.Errorf("resolve volumes for dependency '%s': %w", svc.Name, err)
	}
	normalized, err := docker.NormalizeVolumeNamesInStrings(resolved, svc.ProjectName, namedVolumeMap)
	if err != nil {
		return nil, fmt.Errorf("normalize volume names for dependency '%s': %w", svc.Name, err)
	}
	service["volumes"] = normalized
	return namedVolumeMap, nil
}

// declareTopLevelVolumes adds a top-level `volumes:` block to the compose
// map for every named volume referenced by the service. Without a
// matching top-level declaration, docker compose rejects service-level
// named-volume references.
func declareTopLevelVolumes(compose map[string]any, namedVolumeMap map[string]string) {
	if len(namedVolumeMap) == 0 {
		return
	}
	topLevel := make(map[string]any, len(namedVolumeMap))
	for _, prefixed := range namedVolumeMap {
		topLevel[prefixed] = map[string]any{}
	}
	compose["volumes"] = topLevel
}
