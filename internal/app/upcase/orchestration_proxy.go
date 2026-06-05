package upcase

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"raioz/internal/docker"
	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/netutil"
	"raioz/internal/output"
)

// startProxy wires Caddy routes for every service/dependency and starts the
// proxy container. Routing rules: docker services resolve via container DNS
// on the shared network; host services via host.docker.internal; ports come
// from detections (already populated by the allocator).
//
// Returns an error when the proxy cannot start — caller treats this as hard
// `up` failure since `proxy: true` is an explicit HTTPS request.
func (uc *UseCase) startProxy(
	ctx context.Context,
	deps *models.Deps,
	detections DetectionMap,
	serviceNames []string,
	networkName string,
) error {
	// ADR-013 / ADR-032: single Configure call; TLS string normalized
	// through ParseTLSMode (legacy mkcert/letsencrypt aliases accepted).
	cfg := interfaces.ProxyConfig{
		ProjectName:   deps.Project.Name,
		Workspace:     deps.Workspace,
		NetworkSubnet: deps.Network.GetSubnet(),
	}
	if deps.ProxyConfig != nil {
		cfg.Domain = deps.ProxyConfig.Domain
		if mode, ok := interfaces.ParseTLSMode(deps.ProxyConfig.TLS); ok {
			cfg.TLSMode = mode
		}
		cfg.ContainerIP = deps.ProxyConfig.IP
		cfg.Publish = deps.ProxyConfig.Publish
	}
	uc.deps.ProxyManager.Configure(cfg)

	output.PrintProgress(i18n.T("up.proxy.starting"))

	enrichDetectionsWithExposedPorts(ctx, deps, detections)

	for name := range detections {
		if !shouldProxy(deps, name) {
			logging.DebugWithContext(ctx, "Skipping non-HTTP dependency from proxy",
				"dep", name)
			continue
		}
		detection := detections[name]
		route := buildProxyRoute(ctx, docker.NewLookup(), deps, name, &detection)
		if err := uc.deps.ProxyManager.AddRoute(ctx, route); err != nil {
			logging.WarnWithContext(ctx, "Failed to add proxy route",
				"service", name, "error", err.Error())
		}
	}

	// Persist this project's routes BEFORE Start so that when Start (or the
	// internal Reload it triggers when the proxy is already up) regenerates
	// the Caddyfile, it sees this project's contribution alongside any
	// sibling projects already running in the workspace. No-op outside of
	// workspace-shared mode.
	if err := uc.deps.ProxyManager.SaveProjectRoutes(); err != nil {
		logging.WarnWithContext(ctx, "Failed to persist project routes for shared proxy",
			"error", err.Error())
	}

	if err := uc.deps.ProxyManager.Start(ctx, networkName); err != nil {
		return uc.handleProxyStartFailure(ctx, networkName, serviceNames, err)
	}

	output.PrintProgressDone(i18n.T("up.proxy.started"))
	printProxyURLs(uc.deps.ProxyManager, serviceNames)
	printHostsHintIfUnpublished(uc.deps.ProxyManager)
	return nil
}

// printHostsHintIfUnpublished surfaces the /etc/hosts entry the user needs
// to map proxy hostnames to its container IP — only relevant when the
// proxy is NOT bound to host 80/443 (publish:false). On macOS/Windows the
// Docker bridge IP isn't reachable from the host, so we additionally warn
// that publish:false won't work without a workaround.
func printHostsHintIfUnpublished(pm interfaces.ProxyManager) {
	if pm.IsPublished() {
		return
	}
	line := pm.HostsLine()
	if line == "" {
		return
	}
	output.PrintInfo("")
	output.PrintInfo(i18n.T("up.proxy.unpublished_intro"))
	output.PrintInfo(i18n.T("up.proxy.hosts_line_hint"))
	output.PrintInfo("")
	output.PrintInfo("    " + line)
	output.PrintInfo("")
	output.PrintInfo(i18n.T("up.proxy.hosts_tip"))
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		output.PrintWarning(i18n.T("up.proxy.unpublished_warning_os", runtime.GOOS))
	}
}

// Proxy-failure recovery outcomes.
const (
	proxyActionCancel = iota
	proxyActionRetry
	proxyActionSkip
)

// proxyFailurePrompt is the interactive decision point surfaced when the
// proxy fails to start. Declared as a var so tests can stub it without
// wiring a fake tty. Defaults to the real prompt driven by stdin.
var proxyFailurePrompt = promptProxyFailureAction

// stdinIsInteractiveFn is the tty check used before prompting. Same rationale
// as proxyFailurePrompt — tests can force the interactive branch without
// attaching a PTY.
var stdinIsInteractiveFn = stdinIsInteractive

// handleProxyStartFailure turns a proxy start error into a recoverable user
// decision when stdin is a tty. Offers retry (user freed the ports), skip
// (continue without HTTPS routing), or cancel. In non-interactive contexts
// (CI, scripts, piped stdin) it falls back to the hard-fail behavior so
// automation doesn't hang forever waiting for a human.
func (uc *UseCase) handleProxyStartFailure(
	ctx context.Context, networkName string, serviceNames []string, firstErr error,
) error {
	logging.ErrorWithContext(ctx, "Failed to start proxy", "error", firstErr.Error())
	output.PrintError(i18n.T("up.proxy.start_failed", firstErr.Error()))

	if !stdinIsInteractiveFn() {
		return fmt.Errorf("proxy start failed: %w", firstErr)
	}

	switch proxyFailurePrompt() {
	case proxyActionRetry:
		output.PrintProgress(i18n.T("up.proxy.retrying"))
		if err := uc.deps.ProxyManager.Start(ctx, networkName); err != nil {
			output.PrintError(i18n.T("up.proxy.retry_failed", err.Error()))
			return fmt.Errorf("proxy start failed on retry: %w", err)
		}
		output.PrintProgressDone(i18n.T("up.proxy.started"))
		printProxyURLs(uc.deps.ProxyManager, serviceNames)
		return nil
	case proxyActionSkip:
		output.PrintInfo(i18n.T("up.proxy.skip_continue"))
		return nil
	default:
		return fmt.Errorf("proxy start failed: %w", firstErr)
	}
}

// promptProxyFailureAction asks the user how to recover from a proxy start
// failure. Returns one of proxyAction* constants. Any unrecognized input
// (including EOF from a closed stdin) maps to cancel, which bubbles up as
// the original error — safe default when something unexpected happens.
func promptProxyFailureAction() int {
	output.PrintInfo("")
	output.PrintInfo(i18n.T("up.proxy.prompt_header"))
	output.PrintInfo(i18n.T("up.proxy.prompt_opt_retry"))
	output.PrintInfo(i18n.T("up.proxy.prompt_opt_skip"))
	output.PrintInfo(i18n.T("up.proxy.prompt_opt_cancel"))
	output.PrintPrompt(i18n.T("up.proxy.prompt_input"))

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return proxyActionCancel
	}
	switch strings.TrimSpace(response) {
	case "1":
		return proxyActionRetry
	case "2":
		return proxyActionSkip
	default:
		return proxyActionCancel
	}
}

// stdinIsInteractive reports whether os.Stdin is attached to a terminal.
// Used to decide between an interactive prompt and a silent hard-fail.
// Avoids pulling in golang.org/x/term — the character-device check is
// portable across Linux/macOS and sufficient for our needs.
func stdinIsInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// printProxyURLs lists the HTTPS URLs the proxy exposes for each service.
// Extracted so retry-success and first-try-success share the same output.
func printProxyURLs(pm interfaces.ProxyManager, serviceNames []string) {
	for _, name := range serviceNames {
		url := pm.GetURL(name)
		if url != "" {
			output.PrintInfo(name + " -> " + url)
		}
	}
}

// Image classification (which deps speak binary protocols and shouldn't get
// HTTPS routes) lives in internal/proxy/filter.go so the same heuristic is
// shared by orchestration AND the standalone `raioz hosts` command.

// shouldProxy decides whether to create a proxy route for a given service or
// dependency name. Services always get routed (they're the user's app). Deps
// get routed unless the image is on the known non-HTTP list AND the user
// didn't explicitly declare routing on it. Explicit `routing:` in raioz.yaml
// is the opt-in escape hatch for deps where the user knows they do speak
// HTTP (e.g. a custom image that happens to reuse a DB name).
func shouldProxy(deps *models.Deps, name string) bool {
	if svc, isService := deps.Services[name]; isService {
		// `proxy: false` opts a service out of routing — used for host-net
		// services with no UI (Prometheus, exporters) where a route would be
		// dead and misleading. Absence of the override keeps the default.
		return svc.ProxyOverride == nil || !svc.ProxyOverride.Disabled
	}
	entry, isDep := deps.Infra[name]
	if !isDep || entry.Inline == nil {
		return true
	}
	if entry.Inline.Routing != nil {
		return true
	}
	return !isNonHTTPImage(entry.Inline.Image)
}

// isNonHTTPImage delegates to the shared classifier in proxy/filter.go.
// Local alias kept for readability of nearby call sites.
func isNonHTTPImage(image string) bool {
	return netutil.IsNonHTTPImage(image)
}

// enrichDetectionsWithExposedPorts backfills detection.Port for image-based
// dependencies whose port is still unknown by this stage. Most official
// images declare EXPOSE in their Dockerfile (postgres:5432, pgadmin4:80,
// redisinsight:5540 …) — reading that manifest lets raioz route the proxy
// without forcing the user to copy the port into `ports:` or `expose:`.
//
// Dependencies are already pulled at this point (deps start before proxy
// routing), so a pure inspect is enough — no pull here. Lookup failures
// are silent: if Docker is offline, the image is unusual, or no TCP port
// is declared, Port stays 0 and the existing fallback chain in
// buildProxyRoute handles it.
func enrichDetectionsWithExposedPorts(
	ctx context.Context,
	deps *models.Deps,
	detections DetectionMap,
) {
	for name, entry := range deps.Infra {
		if entry.Inline == nil {
			continue
		}
		det, ok := detections[name]
		if !ok || det.Port != 0 {
			continue
		}
		image := docker.BuildImageName(entry.Inline.Image, entry.Inline.Tag)
		if image == "" {
			continue
		}
		port, err := docker.GetImageExposedPort(ctx, image)
		if err != nil {
			logging.DebugWithContext(ctx, "ExposedPorts lookup skipped",
				"dep", name, "image", image, "error", err.Error())
			continue
		}
		det.Port = port
		detections[name] = det
	}
}

// proxyTargetOverride returns the user's explicit (target, port) for a
// service or dependency if declared in raioz.yaml (`<kind>.<name>.proxy:`).
// Empty strings / zero port signal "not set" and caller must fall back to
// detection. This is the escape hatch for entries whose runtime raioz can't
// fully introspect — services with `command:` that launches a hidden compose
// stack (BUG-13), or dependencies using `compose:` / a non-default port.
func proxyTargetOverride(deps *models.Deps, name string) (string, int) {
	if svc, ok := deps.Services[name]; ok && svc.ProxyOverride != nil {
		return svc.ProxyOverride.Target, svc.ProxyOverride.Port
	}
	if entry, ok := deps.Infra[name]; ok && entry.Inline != nil && entry.Inline.ProxyOverride != nil {
		return entry.Inline.ProxyOverride.Target, entry.Inline.ProxyOverride.Port
	}
	return "", 0
}

// buildProxyRoute turns (service name, detection) into a ProxyRoute, honoring
// custom hostnames and routing config (ws/sse/grpc) from raioz.yaml.
func buildProxyRoute(
	ctx context.Context,
	lookup naming.ContainerLookup,
	deps *models.Deps,
	name string,
	detection *models.DetectResult,
) interfaces.ProxyRoute {
	hostname := name
	var target string
	var port int

	// User-declared proxy target wins over detection. Covers the case where
	// `command:` hides a compose stack from raioz — without this, detection
	// labels the service as "host", target falls to host.docker.internal,
	// and port stays 0. With the override, we forward straight to the
	// container the user told us about.
	overrideTarget, overridePort := proxyTargetOverride(deps, name)
	if overrideTarget != "" {
		target = overrideTarget
		port = overridePort
	} else if detection.IsDocker() {
		if entry, ok := deps.Infra[name]; ok {
			var nameOverride string
			if entry.Inline != nil {
				nameOverride = entry.Inline.Name
			}
			// Resolve against live Docker so a user-supplied compose with
			// container_name: routes to the actual container, not the
			// canonical raioz name. Falls back to canonical when nothing
			// is found (test paths pass nil lookup → canonical).
			target = naming.ContainerTarget(ctx, lookup,
				deps.Project.Name, name, nameOverride)
		} else {
			target = naming.Container(deps.Project.Name, name)
		}
		port = detection.Port
	} else {
		target = "host.docker.internal"
		port = detection.Port
	}

	// proxy.port without proxy.target: detection picked the target
	// (container or host) but the user still wants to force a specific
	// upstream port. Common case: multi-port images like mailpit
	// (1025 SMTP + 8025 UI) where detection grabs the wrong one.
	if overrideTarget == "" && overridePort > 0 {
		port = overridePort
	}

	// Fallback port inference for edge cases where detection.Port is 0
	// (e.g. obscure runtimes the allocator also couldn't resolve). User
	// overrides are honored verbatim — if the user passed `proxy.port`, we
	// keep whatever they wrote including 0 (lets Caddy pick default).
	if port == 0 && overrideTarget == "" {
		if svc, ok := deps.Services[name]; ok {
			port = inferServicePort(svc, *detection)
		}
		if entry, ok := deps.Infra[name]; ok && entry.Inline != nil {
			switch {
			case len(entry.Inline.Ports) > 0:
				port = parseFirstPort(entry.Inline.Ports[0])
			case len(entry.Inline.Expose) > 0:
				// `expose:` is documented as "container port this dep
				// listens on" — the natural proxy target when the user
				// didn't also publish a host-side mapping (v0.1.1 fix).
				port = entry.Inline.Expose[0]
			}
		}
	}

	overrideName, aliases := resolveHostnameAndAliases(deps, name)
	if overrideName != "" {
		hostname = overrideName
	}

	route := interfaces.ProxyRoute{
		ServiceName: name,
		Hostname:    hostname,
		Aliases:     aliases,
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
