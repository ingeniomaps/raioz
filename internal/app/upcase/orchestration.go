package upcase

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/logging"
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
) (string, []string, []string, error) {
	// Step 0: Kill stale host processes from previous run
	cleanStaleHostProcesses(ctx, projectDir, deps.Project.Name)

	// Step 1: Detect runtimes
	output.PrintProgress(i18n.T("up.detecting_runtimes"))
	detections := detectRuntimes(ctx, deps)
	output.PrintProgressDone(i18n.T("up.runtimes_detected"))

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

			// Build container name
			containerName := fmt.Sprintf("raioz-%s-%s", deps.Project.Name, name)

			svcCtx := buildServiceContext(
				name, detection, networkName,
				envVars,
				infraPorts(entry),
				nil, // infra has no dependsOn
				containerName,
				"", // no path for images
			)

			if err := dispatcher.Start(ctx, svcCtx); err != nil {
				imageRef := ""
				if envVars["RAIOZ_IMAGE"] != "" {
					imageRef = envVars["RAIOZ_IMAGE"]
				}
				return "", nil, nil, errors.DependencyStartFailed(name, imageRef, err)
			}
			output.PrintInfraStarted(name)
		}

		logging.InfoWithContext(ctx, "Dependencies started",
			"count", len(infraNames),
			"duration_ms", time.Since(infraStart).Milliseconds())

		// Wait for infra health
		output.PrintProgress(i18n.T("up.waiting_infra_healthy"))
		// TODO: Implement health waiting for orchestrated mode
		output.PrintProgressDone(i18n.T("up.infra_healthy"))
	}

	// Build endpoints map for service discovery
	endpoints := buildEndpoints(deps, detections)

	// Step 3: Start services in dependency order
	serviceNames := orderedServiceNames(deps)

	if len(serviceNames) > 0 {
		output.PrintProgress(i18n.T("up.starting_services", len(serviceNames)))
		svcStart := time.Now()

		for _, name := range serviceNames {
			svc := deps.Services[name]
			detection := detections[name]

			containerName := fmt.Sprintf("raioz-%s-%s", deps.Project.Name, name)

			// Generate discovery env vars for this service
			envVars := make(map[string]string)
			if uc.deps.DiscoveryManager != nil {
				envVars = uc.deps.DiscoveryManager.GenerateEnvVars(
					name, detection.Runtime, endpoints, deps.Proxy,
				)
			}

			svcCtx := buildServiceContext(
				name, detection, networkName,
				envVars,
				servicePorts(svc),
				svc.GetDependsOn(),
				containerName,
				svc.Source.Path,
			)

			if err := dispatcher.Start(ctx, svcCtx); err != nil {
				return "", nil, nil, errors.ServiceStartFailed(name, string(detection.Runtime), err)
			}

			output.PrintSuccess(name + " (" + string(detection.Runtime) + ")")
		}

		logging.InfoWithContext(ctx, "Services started",
			"count", len(serviceNames),
			"duration_ms", time.Since(svcStart).Milliseconds())
		output.PrintProgressDone(i18n.T("up.services_started", len(serviceNames)))
	}

	// Persist host PIDs so raioz down / next raioz up can find them
	saveHostPIDs(projectDir, deps.Project.Name, dispatcher, serviceNames, detections)

	// Step 4: Start proxy if enabled
	if deps.Proxy && uc.deps.ProxyManager != nil {
		// Apply proxy configuration from raioz.yaml
		if deps.ProxyConfig != nil {
			uc.deps.ProxyManager.SetDomain(deps.ProxyConfig.Domain)
			uc.deps.ProxyManager.SetTLSMode(deps.ProxyConfig.TLS)
		}

		output.PrintProgress("Starting proxy...")

		// Add routes for all services and dependencies
		for name, detection := range detections {
			hostname := name
			var target string
			var port int

			if detection.IsDocker() {
				target = fmt.Sprintf("raioz-%s-%s", deps.Project.Name, name)
				port = detection.Port
			} else {
				target = "host.docker.internal"
				port = detection.Port
			}

			// If port not detected, try config ports or env
			if port == 0 {
				if svc, ok := deps.Services[name]; ok {
					port = inferServicePort(svc, detection)
				}
				if entry, ok := deps.Infra[name]; ok && entry.Inline != nil && len(entry.Inline.Ports) > 0 {
					port = parseFirstPort(entry.Inline.Ports[0])
				}
			}

			// Check for custom hostname
			if svc, ok := deps.Services[name]; ok && svc.Hostname != "" {
				hostname = svc.Hostname
			}

			route := interfaces.ProxyRoute{
				ServiceName: name,
				Hostname:    hostname,
				Target:      target,
				Port:        port,
			}

			// Apply routing config
			if svc, ok := deps.Services[name]; ok && svc.Routing != nil {
				route.WebSocket = svc.Routing.WS
				route.Stream = svc.Routing.Stream
				route.GRPC = svc.Routing.GRPC
			}

			uc.deps.ProxyManager.AddRoute(ctx, route)
		}

		if err := uc.deps.ProxyManager.Start(ctx, networkName); err != nil {
			logging.WarnWithContext(ctx, "Failed to start proxy", "error", err.Error())
			output.PrintWarning("Proxy failed to start: " + err.Error())
		} else {
			output.PrintProgressDone("Proxy started")
			// Show URLs
			for _, name := range serviceNames {
				url := uc.deps.ProxyManager.GetURL(name)
				if url != "" {
					output.PrintInfo(name + " -> " + url)
				}
			}
		}
	}

	return "", serviceNames, infraNames, nil
}

// buildEndpoints creates the endpoints map from config and detections for service discovery.
func buildEndpoints(deps *config.Deps, detections DetectionMap) map[string]interfaces.ServiceEndpoint {
	endpoints := make(map[string]interfaces.ServiceEndpoint)

	for name, detection := range detections {
		ep := interfaces.ServiceEndpoint{
			Name:    name,
			Runtime: detection.Runtime,
			Port:    detection.Port,
		}

		if detection.IsDocker() {
			// Docker services use their container name as host within the network
			ep.Host = fmt.Sprintf("raioz-%s-%s", deps.Project.Name, name)
		} else {
			ep.Host = "localhost"
		}

		// Override port from config if specified
		if svc, ok := deps.Services[name]; ok && svc.Docker != nil && len(svc.Docker.Ports) > 0 {
			ep.Port = parseFirstPort(svc.Docker.Ports[0])
		}
		if entry, ok := deps.Infra[name]; ok && entry.Inline != nil && len(entry.Inline.Ports) > 0 {
			ep.Port = parseFirstPort(entry.Inline.Ports[0])
		}

		endpoints[name] = ep
	}

	return endpoints
}

// parseFirstPort extracts the host port from a port mapping like "8080:3000" or "5432".
func parseFirstPort(portSpec string) int {
	// Format: "hostPort:containerPort" or just "port"
	parts := strings.SplitN(portSpec, ":", 2)
	portStr := parts[0]

	port := 0
	for _, ch := range portStr {
		if ch >= '0' && ch <= '9' {
			port = port*10 + int(ch-'0')
		} else {
			break
		}
	}
	return port
}

// orderedServiceNames returns service names sorted by dependency order (topological sort).
func orderedServiceNames(deps *config.Deps) []string {
	// Build adjacency list
	graph := make(map[string][]string)
	inDegree := make(map[string]int)
	allServices := make(map[string]bool)

	for name, svc := range deps.Services {
		allServices[name] = true
		for _, dep := range svc.GetDependsOn() {
			// Only count dependencies that are services (not infra)
			if _, isService := deps.Services[dep]; isService {
				graph[dep] = append(graph[dep], name)
				inDegree[name]++
			}
		}
		if _, exists := inDegree[name]; !exists {
			inDegree[name] = 0
		}
	}

	// Kahn's algorithm
	var queue []string
	for name := range allServices {
		if inDegree[name] == 0 {
			queue = append(queue, name)
		}
	}

	var ordered []string
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		ordered = append(ordered, name)

		for _, dependent := range graph[name] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	return ordered
}

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

// preHookExec runs pre-hooks before starting services.
func (uc *UseCase) preHookExec(ctx context.Context, deps *config.Deps, projectDir string) error {
	if deps.PreHook == "" {
		return nil
	}

	output.PrintProgress("Running pre-hook...")
	logging.InfoWithContext(ctx, "Executing pre-hook", "command", deps.PreHook)

	commands := strings.Split(deps.PreHook, " && ")
	for _, cmdStr := range commands {
		cmdStr = strings.TrimSpace(cmdStr)
		if cmdStr == "" {
			continue
		}
		cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
		cmd.Dir = projectDir
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("pre-hook failed: %s: %w\n%s", cmdStr, err, string(output))
		}
	}

	output.PrintProgressDone("Pre-hook completed")
	return nil
}

// postHookExec runs post-hooks after starting services.
func (uc *UseCase) postHookExec(ctx context.Context, deps *config.Deps, projectDir string) {
	if deps.PostHook == "" {
		return
	}

	logging.InfoWithContext(ctx, "Executing post-hook", "command", deps.PostHook)

	commands := strings.Split(deps.PostHook, " && ")
	for _, cmdStr := range commands {
		cmdStr = strings.TrimSpace(cmdStr)
		if cmdStr == "" {
			continue
		}
		cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
		cmd.Dir = projectDir
		if out, err := cmd.CombinedOutput(); err != nil {
			logging.WarnWithContext(ctx, "Post-hook failed", "command", cmdStr, "error", err.Error(), "output", string(out))
		}
	}
}
