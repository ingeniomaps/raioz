package upcase

import (
	"context"
	"strconv"
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
	// Step 0 — kill stale host processes from a previous run, scoped
	// to the services this `up` touches (full or `--only` subset).
	scope := make(map[string]struct{}, len(deps.Services))
	for name := range deps.Services {
		scope[name] = struct{}{}
	}
	cleanStaleHostProcesses(ctx, projectDir, deps.Project.Name, scope)

	// Step 1: Detect runtimes
	output.PrintProgress(i18n.T("up.detecting_runtimes"))
	detections := detectRuntimes(ctx, deps)
	output.PrintProgressDone(i18n.T("up.runtimes_detected"))

	// Step 1b: Allocate host ports deterministically + run under a
	// global flock (acquirePortsLock) so concurrent
	// `raioz up` in different workspaces can't probe-and-claim the
	// same host port then race on `docker run -p`. Lock released
	// when processOrchestration returns.
	portsLockRelease, err := acquirePortsLock()
	if err != nil {
		return nil, err
	}
	defer portsLockRelease()
	portAllocs, err := AllocateHostPorts(deps, detections)
	if err != nil {
		return nil, err
	}

	// Step 1c: Check for host-port bind conflicts. When an external process
	// or a container from another project occupies a port raioz needs, we
	// prompt the user instead of failing or killing the occupant.
	if conflicts := checkPortBindConflicts(portAllocs); len(conflicts) > 0 {
		if err := resolvePortBindConflicts(
			ctx, conflicts, portAllocs, configPath, deps.Project.Name,
		); err != nil {
			return nil, err
		}
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

	// Create dispatcher
	dispatcher := orchestrate.NewDispatcher(uc.deps.DockerRunner)
	networkName := deps.Network.GetName()

	// Step 2: Start dependencies (infra) first
	var infraNames []string
	for name := range deps.Infra {
		infraNames = append(infraNames, name)
	}

	// deferredDeps: sibling-owned deps skipped at dispatch (issue #26
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

		for _, name := range infraNames {
			detection := detections[name]
			entry := deps.Infra[name]

			// Sibling-deps gate (issue #26). applySiblingVerdict deletes
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

			// Build image reference + env for the runner.
			envVars := map[string]string{}
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
			}

			// Build container name. Deps may be workspace-shared or have an
			// explicit `name:` override — both cases resolved by DepContainer.
			var nameOverride string
			if entry.Inline != nil {
				nameOverride = entry.Inline.Name
			}
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

			// When the dep declares `compose:`, hand its files + env files
			// straight to ImageRunner. ImageRunner branches on these: if
			// set it uses the user's compose with a network/labels overlay
			// layered on top; if not it generates a minimal compose from
			// the `image:` field (legacy behavior).
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

			if err := dispatcher.Start(ctx, svcCtx); err != nil {
				imageRef := ""
				if envVars["RAIOZ_IMAGE"] != "" {
					imageRef = envVars["RAIOZ_IMAGE"]
				}
				return nil, errors.DependencyStartFailed(name, imageRef, err)
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

	if len(serviceNames) > 0 {
		output.PrintProgress(i18n.T("up.starting_services", len(serviceNames)))
		svcStart := time.Now()

		for _, name := range serviceNames {
			svc := deps.Services[name]
			detection := detections[name]

			containerName := naming.Container(deps.Project.Name, name)

			// Generate discovery env vars for this service
			envVars := make(map[string]string)
			if uc.deps.DiscoveryManager != nil {
				envVars = uc.deps.DiscoveryManager.GenerateEnvVars(
					name, detection.Runtime, endpoints, deps.Proxy,
				)
			}

			// Inject PORT for host services so frameworks honoring $PORT
			// (Next.js, Vite, Django, etc.) rebind to the allocator's
			// pick. Docker services get their port via published config.
			if alloc, ok := portAllocs.Services[name]; ok && alloc.IsHost() && alloc.Port > 0 {
				envVars["PORT"] = strconv.Itoa(alloc.Port)
			}

			svcCtx := buildServiceContext(
				name, detection, networkName,
				envVars,
				servicePorts(svc),
				svc.GetDependsOn(),
				containerName,
				svc.Source.Path,
				deps.Project.Name,
			)

			// Propagate custom stop command (from `stop:` in raioz.yaml) so the
			// runner can use it instead of SIGTERMing the PID.
			if svc.Commands != nil && svc.Commands.Down != "" {
				svcCtx.StopCommand = svc.Commands.Down
			}
			// ADR-025: needed for HostRunner's launcher-container wait.
			if svc.ProxyOverride != nil {
				svcCtx.ProxyTarget = svc.ProxyOverride.Target
			}

			if err := dispatcher.Start(ctx, svcCtx); err != nil {
				return nil, errors.ServiceStartFailed(name, string(detection.Runtime), err)
			}

			output.PrintSuccess(name + " (" + string(detection.Runtime) + ")")
		}

		logging.InfoWithContext(ctx, "Services started",
			"count", len(serviceNames),
			"duration_ms", time.Since(svcStart).Milliseconds())
		output.PrintProgressDone(i18n.T("up.services_started", len(serviceNames)))
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
		// Legacy `ports:` on inline infra, when the user hasn't migrated to
		// the new publish/expose fields. Keep honoring it verbatim.
		if entry, ok := deps.Infra[name]; ok && entry.Inline != nil &&
			len(entry.Inline.Ports) > 0 && entry.Inline.Publish == nil {
			ep.Port = parseFirstPort(entry.Inline.Ports[0])
			ep.HostPort = ep.Port
		}

		endpoints[name] = ep
	}

	return endpoints
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
