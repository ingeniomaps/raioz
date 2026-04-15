package upcase

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"raioz/internal/config"
	"raioz/internal/detect"
	"raioz/internal/domain/interfaces"
	"raioz/internal/logging"
	"raioz/internal/naming"
	"raioz/internal/output"
	"raioz/internal/proxy"
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
//
// Returns an error when the proxy cannot start. The caller treats this as a
// hard `up` failure: once `proxy: true` is in raioz.yaml the user has
// explicitly asked for unified HTTPS, and silently continuing with a broken
// proxy produces a half-working dev environment that's worse than none.
func (uc *UseCase) startProxy(
	ctx context.Context,
	deps *config.Deps,
	detections DetectionMap,
	serviceNames []string,
	networkName string,
) error {
	uc.deps.ProxyManager.SetProjectName(deps.Project.Name)
	uc.deps.ProxyManager.SetWorkspace(deps.Workspace)
	uc.deps.ProxyManager.SetNetworkSubnet(deps.Network.GetSubnet())
	if deps.ProxyConfig != nil {
		uc.deps.ProxyManager.SetDomain(deps.ProxyConfig.Domain)
		uc.deps.ProxyManager.SetTLSMode(deps.ProxyConfig.TLS)
		uc.deps.ProxyManager.SetContainerIP(deps.ProxyConfig.IP)
		uc.deps.ProxyManager.SetPublish(deps.ProxyConfig.Publish)
	}

	output.PrintProgress("Starting proxy...")

	for name := range detections {
		if !shouldProxy(deps, name) {
			logging.DebugWithContext(ctx, "Skipping non-HTTP dependency from proxy",
				"dep", name)
			continue
		}
		detection := detections[name]
		route := buildProxyRoute(deps, name, &detection)
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

	output.PrintProgressDone("Proxy started")
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
	output.PrintInfo("Proxy is not bound to host ports (proxy.publish: false).")
	output.PrintInfo("Add this line to /etc/hosts to reach the URLs above:")
	output.PrintInfo("")
	output.PrintInfo("    " + line)
	output.PrintInfo("")
	output.PrintInfo("Tip: `raioz hosts` prints this line on demand.")
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		output.PrintWarning(
			"On " + runtime.GOOS + " the Docker bridge IP is NOT reachable from the host. " +
				"Re-enable proxy.publish: true (or omit it) to bind 80/443 instead.")
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
	output.PrintError("Proxy failed to start: " + firstErr.Error())

	if !stdinIsInteractiveFn() {
		return fmt.Errorf("proxy start failed: %w", firstErr)
	}

	switch proxyFailurePrompt() {
	case proxyActionRetry:
		output.PrintProgress("Retrying proxy start...")
		if err := uc.deps.ProxyManager.Start(ctx, networkName); err != nil {
			output.PrintError("Retry failed: " + err.Error())
			return fmt.Errorf("proxy start failed on retry: %w", err)
		}
		output.PrintProgressDone("Proxy started")
		printProxyURLs(uc.deps.ProxyManager, serviceNames)
		return nil
	case proxyActionSkip:
		output.PrintInfo("Continuing without proxy. Services remain reachable by port/container name.")
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
	output.PrintInfo("How would you like to proceed?")
	output.PrintInfo("  [1] Retry (I've freed the ports)")
	output.PrintInfo("  [2] Skip the proxy for this run (services keep working without https://*.localhost)")
	output.PrintInfo("  [3] Cancel up")
	output.PrintPrompt("Your choice [1-3]: ")

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
func shouldProxy(deps *config.Deps, name string) bool {
	if _, isService := deps.Services[name]; isService {
		return true
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
	return proxy.IsNonHTTPImage(image)
}

// proxyTargetOverride returns the user's explicit (target, port) for a
// service or dependency if declared in raioz.yaml (`<kind>.<name>.proxy:`).
// Empty strings / zero port signal "not set" and caller must fall back to
// detection. This is the escape hatch for entries whose runtime raioz can't
// fully introspect — services with `command:` that launches a hidden compose
// stack (BUG-13), or dependencies using `compose:` / a non-default port.
func proxyTargetOverride(deps *config.Deps, name string) (string, int) {
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
	deps *config.Deps,
	name string,
	detection *detect.DetectResult,
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
			target = naming.DepContainer(deps.Project.Name, name, nameOverride)
		} else {
			target = naming.Container(deps.Project.Name, name)
		}
		port = detection.Port
	} else {
		target = "host.docker.internal"
		port = detection.Port
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
