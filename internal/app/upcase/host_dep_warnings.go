package upcase

import (
	"sort"

	"raioz/internal/domain/models"
)

// unreachableHostDeps lists, per host service, the image dependencies it
// declares in dependsOn that publish no host port. A container reaches
// such a dependency by its name on the Docker network; a host process is
// not on that network, and the X_HOST=localhost it is handed points at
// nothing. Sibling-project and compose dependencies are left out: raioz
// does not decide what those publish.
func unreachableHostDeps(
	deps *models.Deps, detections DetectionMap, portAllocs *PortAllocResult,
) map[string][]string {
	out := map[string][]string{}
	for name, svc := range deps.Services {
		det, ok := detections[name]
		if !ok || det.IsDocker() {
			continue
		}
		for _, depName := range svc.GetDependsOn() {
			entry, isDep := deps.Infra[depName]
			if !isDep || entry.Inline == nil || entry.Inline.Image == "" || entry.Inline.Project != "" {
				continue
			}
			if portAllocs != nil {
				if alloc, published := portAllocs.Deps[depName]; published && len(alloc.Mappings) > 0 {
					continue
				}
			}
			out[name] = append(out[name], depName)
		}
		sort.Strings(out[name])
	}
	return out
}
