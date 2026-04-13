package upcase

import (
	"context"

	"raioz/internal/config"
	"raioz/internal/detect"
	"raioz/internal/domain/interfaces"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/output"
)

// startProxy wires Caddy routes for every service/dependency and starts the
// proxy container. Extracted from processOrchestration to keep that file
// under the 400-line repo cap.
//
// The routing rules match the current proxy behavior:
//
//  1. Docker services resolve via container DNS on the shared network.
//  2. Host services resolve via host.docker.internal so the proxy (which
//     runs inside Docker) can reach the developer's host process.
//  3. Ports come from detections — which at this point already contain the
//     allocator's resolved port for host services.
func (uc *UseCase) startProxy(
	ctx context.Context,
	deps *config.Deps,
	detections DetectionMap,
	serviceNames []string,
	networkName string,
) {
	uc.deps.ProxyManager.SetProjectName(deps.Project.Name)
	if deps.ProxyConfig != nil {
		uc.deps.ProxyManager.SetDomain(deps.ProxyConfig.Domain)
		uc.deps.ProxyManager.SetTLSMode(deps.ProxyConfig.TLS)
	}

	output.PrintProgress("Starting proxy...")

	for name := range detections {
		detection := detections[name]
		route := buildProxyRoute(deps, name, &detection)
		if err := uc.deps.ProxyManager.AddRoute(ctx, route); err != nil {
			logging.WarnWithContext(ctx, "Failed to add proxy route",
				"service", name, "error", err.Error())
		}
	}

	if err := uc.deps.ProxyManager.Start(ctx, networkName); err != nil {
		logging.WarnWithContext(ctx, "Failed to start proxy", "error", err.Error())
		output.PrintWarning("Proxy failed to start: " + err.Error())
		return
	}

	output.PrintProgressDone("Proxy started")
	for _, name := range serviceNames {
		url := uc.deps.ProxyManager.GetURL(name)
		if url != "" {
			output.PrintInfo(name + " -> " + url)
		}
	}
}

// buildProxyRoute turns (service name, detection) into a ProxyRoute, honoring
// custom hostnames and routing config (ws/sse/grpc) from raioz.yaml.
func buildProxyRoute(
	deps *config.Deps,
	name string,
	detection *detect.DetectResult,
) interfaces.ProxyRoute {
	hostname := name
	var target string
	var port int

	if detection.IsDocker() {
		target = naming.Container(deps.Project.Name, name)
		port = detection.Port
	} else {
		target = "host.docker.internal"
		port = detection.Port
	}

	// Fallback port inference for edge cases where detection.Port is 0
	// (e.g. obscure runtimes the allocator also couldn't resolve).
	if port == 0 {
		if svc, ok := deps.Services[name]; ok {
			port = inferServicePort(svc, *detection)
		}
		if entry, ok := deps.Infra[name]; ok && entry.Inline != nil && len(entry.Inline.Ports) > 0 {
			port = parseFirstPort(entry.Inline.Ports[0])
		}
	}

	if svc, ok := deps.Services[name]; ok && svc.Hostname != "" {
		hostname = svc.Hostname
	}

	route := interfaces.ProxyRoute{
		ServiceName: name,
		Hostname:    hostname,
		Target:      target,
		Port:        port,
	}

	if svc, ok := deps.Services[name]; ok && svc.Routing != nil {
		route.WebSocket = svc.Routing.WS
		route.Stream = svc.Routing.Stream
		route.GRPC = svc.Routing.GRPC
	}

	return route
}
