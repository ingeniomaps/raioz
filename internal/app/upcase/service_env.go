package upcase

import (
	"context"
	"strconv"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/logging"
	"raioz/internal/naming"
)

// computedServiceEnv returns the vars raioz works out for a service: the
// discovery set, plus PORT for host runtimes so frameworks honoring $PORT
// (Next.js, Vite, Django, ...) bind the allocator's pick. Docker services
// get their port through the published config instead.
func computedServiceEnv(
	dm interfaces.DiscoveryManager,
	name string,
	detection models.DetectResult,
	endpoints map[string]interfaces.ServiceEndpoint,
	proxyEnabled bool,
	portAllocs *PortAllocResult,
) map[string]string {
	envVars := make(map[string]string)
	if dm != nil {
		envVars = dm.GenerateEnvVars(name, detection.Runtime, endpoints, proxyEnabled)
	}
	if portAllocs != nil {
		if alloc, ok := portAllocs.Services[name]; ok && alloc.IsHost() && alloc.Port > 0 {
			envVars["PORT"] = strconv.Itoa(alloc.Port)
		}
	}
	return envVars
}

// ComputedServiceEnv recomputes, from the config alone, the vars up injects
// into one service. Restart and the file watcher relaunch a single service
// long after up's own maps are gone; going through the same detection,
// allocation and endpoint steps keeps the relaunched process on the env it
// was started with. Nil when the service is unknown.
//
// Call it while the service is still running: a service without `port:`
// keeps the port it holds, which only its live process can tell.
func ComputedServiceEnv(
	ctx context.Context,
	dm interfaces.DiscoveryManager,
	lookup naming.ContainerLookup,
	deps *models.Deps,
	projectDir string,
	name string,
) map[string]string {
	detections := BuildDetectionMap(deps)
	detection, ok := detections[name]
	if !ok {
		return nil
	}
	inferDepExpose(ctx, deps)
	portAllocs, err := AllocateHostPortsOwned(deps, detections, runningHostPorts(projectDir))
	if err != nil {
		// Discovery still works without the allocation; only PORT and the
		// published dep ports are lost.
		logging.WarnWithContext(ctx, "Port allocation failed while recomputing service env",
			"service", name, "error", err.Error())
		portAllocs = nil
	}
	if portAllocs != nil {
		reuseRunningDepHostPorts(ctx, deps, portAllocs)
	}
	applyPortAllocs(detections, portAllocs)
	endpoints := buildEndpoints(ctx, lookup, deps, detections, portAllocs)
	return computedServiceEnv(dm, name, detection, endpoints, deps.Proxy, portAllocs)
}

// applyPortAllocs copies the allocator's picks onto the detections, which
// is where endpoints, proxy routes and the watcher read a port from.
func applyPortAllocs(detections DetectionMap, portAllocs *PortAllocResult) {
	if portAllocs == nil {
		return
	}
	for name, alloc := range portAllocs.Services {
		det := detections[name]
		det.Port = alloc.Port
		detections[name] = det
	}
	// For published deps, write the *first* host mapping into detection.Port
	// so the proxy/discovery path can reach the dependency from the host.
	// Container→container traffic still uses the DNS name + container port,
	// handled by the discovery package.
	for name, alloc := range portAllocs.Deps {
		if len(alloc.Mappings) == 0 {
			continue
		}
		det := detections[name]
		det.Port = alloc.Mappings[0].HostPort
		detections[name] = det
	}
}
