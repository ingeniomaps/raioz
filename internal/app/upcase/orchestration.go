package upcase

import (
	"context"
	"time"

	"raioz/internal/docker"
	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/orchestrate"
	"raioz/internal/output"
)

// processOrchestration handles the new meta-orchestrator flow:
// 1. Detect runtimes for each service/dependency
// 2. Start dependencies (images) first
// 3. Start services (local) in dependency order
// Returns composePath (empty for orchestrated), serviceNames, infraNames, error.
func (uc *UseCase) processOrchestration(
	ctx context.Context,
	deps *models.Deps,
	ws *interfaces.Workspace,
	projectDir string,
	configPath string,
	routerOff bool,
) (*orchestrationResult, error) {
	// Step 0 — find the host services an earlier up left running, scoped
	// to the services this `up` touches (full or `--only` subset). They
	// are adopted, not restarted.
	scope := make(map[string]struct{}, len(deps.Services))
	for name := range deps.Services {
		scope[name] = struct{}{}
	}
	alreadyRunning := runningHostServices(ctx, projectDir, deps, scope)

	// Step 1: Detect runtimes
	output.PrintProgress(i18n.T("up.detecting_runtimes"))
	detections := detectRuntimes(ctx, deps)
	output.PrintProgressDone(i18n.T("up.runtimes_detected"))

	// Step 1b: allocate host ports under the global ports flock so
	// concurrent `raioz up` in different workspaces can't race on the
	// same host port. allocatePortsLocked drops the lock before the
	// sibling dispatch phase (see its doc for the deadlock it avoids).
	for _, name := range inferDepExpose(ctx, deps) {
		output.PrintWarning(i18n.T("up.publish_without_expose", name))
	}
	portAllocs, err := allocatePortsLocked(ctx, deps, detections, configPath)
	if err != nil {
		return nil, err
	}

	applyPortAllocs(detections, portAllocs)
	for name, pid := range alreadyRunning {
		if portAllocs.RunningHost == nil {
			portAllocs.RunningHost = make(map[string]int)
		}
		portAllocs.RunningHost[name] = pid
	}

	// Create dispatcher
	dispatcher := orchestrate.NewDispatcher(uc.deps.DockerRunner)
	networkName := deps.Network.GetName()

	// Step 2: Start dependencies (infra) first
	var infraNames []string
	for name := range deps.Infra {
		infraNames = append(infraNames, name)
	}

	// deferredDeps: sibling-owned deps skipped at dispatch (ADR-008
	// mode B). Persisted into LocalState so `down` matches the skip.
	var deferredDeps []string
	// dispatchedInfra: subset of infraNames with a container in this
	// project's namespace. Health/endpoints/proxy iterate this.
	var dispatchedInfra []string

	if len(infraNames) > 0 {
		verdicts, toDispatch, err := resolveSiblingVerdicts(ctx, infraNames, deps)
		if err != nil {
			return nil, err
		}
		if err := verifySiblingsStillUp(ctx, verdicts); err != nil {
			return nil, err
		}
		if toDispatch > 0 {
			output.PrintProgress(i18n.T("up.starting_infra", toDispatch))
		}
		infraStart := time.Now()
		devOverrides := loadDevOverrides(projectDir)

		for _, name := range infraNames {
			detection := detections[name]
			entry := deps.Infra[name]

			// Sibling-deps gate (ADR-008). applySiblingVerdict deletes
			// sibling-mode deps from `detections` (so endpoints / proxy /
			// health auto-skip), spawns recursive raioz up for mode A,
			// and stamps mode B defers for the matching down.
			skip, err := applySiblingVerdict(
				ctx, name, verdicts[name], projectDir, detections, &deferredDeps)
			if err != nil {
				return nil, err
			}
			if skip {
				continue
			}
			dispatchedInfra = append(dispatchedInfra, name)

			svcCtx := buildDepContext(deps, name, entry, detection, networkName, projectDir, portAllocs)
			// A dependency promoted with `raioz dev` runs from its local
			// path until it is reset; up brings back what was promoted,
			// not the image.
			if override, ok := devOverrides[name]; ok {
				svcCtx = DevOverrideContext(svcCtx, override.LocalPath)
			}

			if err := dispatcher.Start(ctx, svcCtx); err != nil {
				return nil, errors.DependencyStartFailed(name, svcCtx.EnvVars["RAIOZ_IMAGE"], err)
			}
			output.PrintInfraStarted(name)
		}

		logging.InfoWithContext(ctx, "Dependencies started",
			"count", len(dispatchedInfra),
			"sibling_skipped", len(infraNames)-len(dispatchedInfra),
			"duration_ms", time.Since(infraStart).Milliseconds())

		// Health check only runs when at least one dep was actually
		// dispatched. With every dep deferred to siblings, the check
		// would burn its 10s timeout looking for containers that live
		// in the sibling's namespace.
		if len(dispatchedInfra) > 0 {
			output.PrintProgress(i18n.T("up.waiting_infra_healthy"))
			if err := checkInfraHealth(ctx, dispatchedInfra, deps.Project.Name, deps.Infra); err != nil {
				return nil, errors.New(errors.ErrCodeDockerNotRunning, err.Error()).
					WithSuggestion("Fix the issue above and run 'raioz up' again")
			}
			output.PrintProgressDone(i18n.T("up.infra_healthy"))
		}

		// Record this project's reference to each shared dep it dispatched
		// so down can tear them down only when the last consumer leaves.
		registerSharedDepRefs(ctx, deps, dispatchedInfra)
	}

	// Build endpoints map for service discovery
	endpoints := buildEndpoints(ctx, docker.NewLookup(), deps, detections, portAllocs)

	// Step 2.5 — preUp hook (ADR-024): runs post-infra,
	// pre-services. Failure aborts.
	if err := uc.preUpHookExec(ctx, deps, projectDir); err != nil {
		return nil, err
	}

	// Step 3: Start services in dependency order
	serviceNames := orderedServiceNames(deps)

	if err := uc.startServices(ctx, startServicesParams{
		deps:         deps,
		detections:   detections,
		serviceNames: serviceNames,
		endpoints:    endpoints,
		portAllocs:   portAllocs,
		dispatcher:   dispatcher,
		networkName:  networkName,
		projectDir:   projectDir,
		deferredDeps: deferredDeps,
	}); err != nil {
		return nil, err
	}

	// Persist host PIDs (and project/workspace/network provenance) so
	// `raioz down` / next `raioz up` / `raioz status` can find them.
	saveHostPIDs(projectDir, deps.Project.Name, deps.Workspace, networkName,
		dispatcher, serviceNames, detections, deferredDeps)

	// Step 4 — proxy (see orchestration_proxy.go + router_env.go).
	if err := uc.maybeStartProxy(
		ctx, deps, detections, serviceNames, networkName, routerOff,
	); err != nil {
		return nil, err
	}

	return &orchestrationResult{
		serviceNames: serviceNames,
		infraNames:   infraNames,
		dispatcher:   dispatcher,
		detections:   detections,
		networkName:  networkName,
	}, nil
}

// buildEndpoints creates the endpoints map for service discovery.
// Published deps get Port (container-side, DNS-resolvable) AND HostPort
// (host-side via localhost) so each caller can pick the right one.
func buildEndpoints(
	ctx context.Context,
	lookup naming.ContainerLookup,
	deps *models.Deps,
	detections DetectionMap,
	portAllocs *PortAllocResult,
) map[string]interfaces.ServiceEndpoint {
	endpoints := make(map[string]interfaces.ServiceEndpoint)

	for name, detection := range detections {
		ep := interfaces.ServiceEndpoint{
			Name:    name,
			Runtime: detection.Runtime,
			Port:    detection.Port,
		}

		if detection.IsDocker() {
			// Docker services use their container name as host within the network.
			// Dependencies may be workspace-shared, services are always per-project.
			if entry, ok := deps.Infra[name]; ok {
				var nameOverride string
				if entry.Inline != nil {
					nameOverride = entry.Inline.Name
				}
				ep.Host = naming.ContainerTarget(ctx, lookup,
					deps.Project.Name, name, nameOverride)
			} else {
				ep.Host = naming.Container(deps.Project.Name, name)
			}
		} else {
			ep.Host = "localhost"
		}

		// For published dependencies, split container port (for in-network
		// DNS access) from host port (for host-side tools). The allocator
		// already decided both; we just copy them onto the endpoint.
		if portAllocs != nil {
			if alloc, ok := portAllocs.Deps[name]; ok && len(alloc.Mappings) > 0 {
				first := alloc.Mappings[0]
				ep.Port = first.ContainerPort
				ep.HostPort = first.HostPort
			}
		}

		// Legacy service docker ports (raioz.json-style config). For new
		// raioz.yaml services, this path never fires — the allocator is
		// authoritative for host services.
		if svc, ok := deps.Services[name]; ok && svc.Docker != nil && len(svc.Docker.Ports) > 0 {
			ep.Port = parseFirstPort(svc.Docker.Ports[0])
		}
		// URL scheme + legacy `ports:` fallback for inline infra deps. The
		// allocator (above) is authoritative for ports it could map.
		applyInlineDepEndpoint(&ep, name, deps, portAllocs)
		applyProxyURL(&ep, deps, name)

		endpoints[name] = ep
	}

	return endpoints
}

// serviceContainerIPFn resolves the address of a service's container.
// Declared here (this file already imports internal/docker) and as a
// package var so tests can answer without a docker daemon.
var serviceContainerIPFn = docker.ServiceContainerIP

// serviceEnvFor returns the per-service env recompute the file watcher
// uses on every reload.
func (uc *UseCase) serviceEnvFor(
	ctx context.Context, deps *models.Deps, projectDir string,
) func(string) map[string]string {
	return func(name string) map[string]string {
		return ComputedServiceEnv(ctx, uc.deps.DiscoveryManager, docker.NewLookup(), deps, projectDir, name)
	}
}

// orderedServiceNames is defined in orchestration_order.go (topological sort
// over service-to-service dependsOn edges).

// infraPorts extracts port mappings from an InfraEntry.
func infraPorts(entry models.InfraEntry) []string {
	if entry.Inline != nil {
		return entry.Inline.Ports
	}
	return nil
}

// servicePorts extracts port mappings from a Service.
func servicePorts(svc models.Service) []string {
	if svc.Docker != nil {
		return svc.Docker.Ports
	}
	return nil
}
