package upcase

import (
	"context"
	"strconv"
	"time"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
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
	deps *config.Deps,
	ws *interfaces.Workspace,
	projectDir string,
	configPath string,
) (*orchestrationResult, error) {
	// Step 0: Kill stale host processes from previous run
	cleanStaleHostProcesses(ctx, projectDir, deps.Project.Name)

	// Step 1: Detect runtimes
	output.PrintProgress(i18n.T("up.detecting_runtimes"))
	detections := detectRuntimes(ctx, deps)
	output.PrintProgressDone(i18n.T("up.runtimes_detected"))

	// Step 1b: Allocate host ports deterministically. This validates explicit
	// conflicts before we start anything, bumps implicit defaults when they
	// would collide (two host frontends both wanting :3000), and resolves
	// host-side bindings for dependencies the user asked to publish. The
	// resolved port is written back into detections so downstream consumers
	// (proxy routes, discovery env vars) see a single source of truth.
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

	if len(infraNames) > 0 {
		output.PrintProgress(i18n.T("up.starting_infra", len(infraNames)))
		infraStart := time.Now()

		for _, name := range infraNames {
			detection := detections[name]
			entry := deps.Infra[name]

			// Build image reference for the runner
			envVars := map[string]string{}
			if entry.Inline != nil {
				imageRef := entry.Inline.Image
				if entry.Inline.Tag != "" {
					imageRef += ":" + entry.Inline.Tag
				}
				envVars["RAIOZ_IMAGE"] = imageRef

				// Add env vars from inline config
				if entry.Inline.Env != nil {
					// Inline variables
					for k, v := range entry.Inline.Env.GetVariables() {
						envVars[k] = v
					}
					// File-based env (e.g., .env.postgres)
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
			"count", len(infraNames),
			"duration_ms", time.Since(infraStart).Milliseconds())

		// Wait for infra health with diagnostics
		output.PrintProgress(i18n.T("up.waiting_infra_healthy"))
		if err := checkInfraHealth(ctx, infraNames, deps.Project.Name, deps.Infra); err != nil {
			return nil, errors.New(errors.ErrCodeDockerNotRunning, err.Error()).
				WithSuggestion("Fix the issue above and run 'raioz up' again")
		}
		output.PrintProgressDone(i18n.T("up.infra_healthy"))
	}

	// Build endpoints map for service discovery
	endpoints := buildEndpoints(deps, detections, portAllocs)

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

			// Inject PORT for host services. Most modern frameworks
			// (Next.js, Vite, Express, Nuxt, Astro, Django via runserver, etc.)
			// honor $PORT, which lets raioz move two conflicting frontends
			// onto distinct ports without the dev touching any config. Docker
			// services get their port via published/exposed config, not PORT.
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
		dispatcher, serviceNames, detections)

	// Step 4: Start proxy if enabled. The proxy block is extracted into its
	// own file to keep this function under the 400-line cap; see
	// orchestration_proxy.go. A proxy failure aborts `up` — the user opted
	// into `proxy: true` in raioz.yaml, so pretending everything is fine
	// when HTTPS routing is broken just hides the problem.
	if deps.Proxy && uc.deps.ProxyManager != nil {
		if err := uc.startProxy(ctx, deps, detections, serviceNames, networkName); err != nil {
			return nil, err
		}
	}

	return &orchestrationResult{
		serviceNames: serviceNames,
		infraNames:   infraNames,
		dispatcher:   dispatcher,
		detections:   detections,
		networkName:  networkName,
	}, nil
}

// buildEndpoints creates the endpoints map from config and detections for
// service discovery. For published dependencies we populate *both* Port
// (container-side) and HostPort (host-side) so discovery can hand each caller
// the right one: container→container goes via Port on the DNS name,
// host→container goes via HostPort on localhost.
func buildEndpoints(
	deps *config.Deps,
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
				ep.Host = naming.DepContainer(deps.Project.Name, name, nameOverride)
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
func infraPorts(entry config.InfraEntry) []string {
	if entry.Inline != nil {
		return entry.Inline.Ports
	}
	return nil
}

// servicePorts extracts port mappings from a Service.
func servicePorts(svc config.Service) []string {
	if svc.Docker != nil {
		return svc.Docker.Ports
	}
	return nil
}
