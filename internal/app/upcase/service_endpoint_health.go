package upcase

import (
	"context"
	"sort"
	"strings"
	"time"

	"raioz/internal/config"
	"raioz/internal/domain/models"
	"raioz/internal/host"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
)

const (
	// endpointProbeDeadline caps the wait for every declared `health:`
	// endpoint together, not per service. A service that never answers
	// costs the run this much once, not once per service.
	endpointProbeDeadline = 30 * time.Second
	// endpointProbeInterval is the gap between rounds.
	endpointProbeInterval = time.Second
)

// endpointProbe is the HTTP probe waitForServiceEndpoints consults.
// Package var so tests decide the answer without binding a socket —
// same seam as hostStatusPortProbe in the status path.
var endpointProbe = host.ProbeHTTP

// waitForServiceEndpoints polls the `health:` endpoint each service
// declares until it answers or the deadline passes.
//
// The field was parsed into HealthEndpoint and read by nobody: `raioz up`
// reported an environment ready as soon as the processes existed, which
// is the question `health:` was added to answer better. A timeout warns
// rather than aborts — the same posture checkInfraHealth takes when its
// own wait runs out, because a slow boot and a broken service look
// identical from here and the user may well want the rest of the stack.
//
// A service needs an address to be probed: `port:` gives the loopback
// one, and a container — the service's own, or the one `proxy.target:`
// names — gives its network address. With neither there is nothing to
// hit: raioz allocates the host port at run time.
//
// That case is logged at debug, not warned. Declaring `health:` without
// `port:` is the common shape — a service behind the proxy is reached at
// its hostname, not on loopback — so warning about it would put a line on
// every `up` of most projects to say raioz is doing nothing, which is
// noise, not news. `raioz health` still names it on the service's row,
// where the user is asking about health and the reason for a weaker
// signal is on topic.
func waitForServiceEndpoints(ctx context.Context, deps *models.Deps, serviceNames []string) {
	type target struct {
		name string
		url  string
		// alt is the same endpoint on the container's own address, for a
		// compose / Dockerfile service that publishes no host port.
		alt func() string
	}

	var targets []target
	var unprobeable []string

	for _, name := range sortedNames(serviceNames) {
		svc, ok := deps.Services[name]
		if !ok || svc.HealthEndpoint == "" {
			continue
		}
		svc, project := svc, deps.Project.Name
		canAskContainer := ContainerHealthPort(svc) > 0
		if svc.Port <= 0 && !canAskContainer {
			unprobeable = append(unprobeable, name)
			continue
		}
		t := target{name: name}
		if svc.Port > 0 {
			t.url = host.HealthURL(svc.Port, svc.HealthEndpoint)
		}
		if canAskContainer {
			t.alt = func() string {
				return ContainerHealthURL(ctx, project, name, svc, serviceContainerIPFn, containerIPFn)
			}
		}
		targets = append(targets, t)
	}

	if len(unprobeable) > 0 {
		logging.DebugWithContext(ctx, "health: declared without port:, not probing",
			"services", strings.Join(unprobeable, ","))
	}
	if len(targets) == 0 {
		return
	}

	output.PrintProgress(i18n.T("up.waiting_health_endpoints", len(targets)))

	pending := make(map[string]string, len(targets))
	alts := make(map[string]func() string, len(targets))
	for _, t := range targets {
		pending[t.name] = t.url
		alts[t.name] = t.alt
	}

	deadline := time.Now().Add(endpointProbeDeadline)
	for {
		for name, url := range pending {
			ok := url != "" && endpointProbe(ctx, url)
			if !ok && alts[name] != nil {
				if alt := alts[name](); alt != "" {
					if ok = endpointProbe(ctx, alt); !ok && url == "" {
						// Nothing on loopback to name in the warning.
						pending[name] = alt
					}
				}
			}
			if ok {
				delete(pending, name)
				output.PrintSuccess(i18n.T("up.health_endpoint_ok", name))
			}
		}
		if len(pending) == 0 {
			return
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(endpointProbeInterval):
		}
	}

	for _, name := range sortedNames(keysOf(pending)) {
		output.PrintWarning(i18n.T("up.health_endpoint_timeout", name, pending[name]))
	}
}

// ContainerHealthPort is the port a service's health endpoint answers on
// inside its container: the one `proxy.port:` declares when the service
// names its container with `proxy.target:`, `port:` otherwise. Zero when
// the service runs in no container raioz can address.
func ContainerHealthPort(svc models.Service) int {
	if o := svc.ProxyOverride; o != nil && o.Target != "" {
		if o.Port > 0 {
			return o.Port
		}
		return svc.Port
	}
	if det := config.ResolveServiceDetection(svc, svc.Source.Path); det.IsDocker() {
		return svc.Port
	}
	return 0
}

// ContainerHealthURL is the service's `health:` endpoint on its container's
// own address, "" when no container answers for it. A container that
// publishes no host port is only reachable there: loopback says nothing
// about it. `proxy.target:` wins over the container raioz would look for
// by label, because it is the user saying which container serves.
func ContainerHealthURL(
	ctx context.Context, project, name string, svc models.Service,
	byLabel func(ctx context.Context, project, service string) string,
	byName func(ctx context.Context, container string) string,
) string {
	port := ContainerHealthPort(svc)
	if port <= 0 {
		return ""
	}
	ip := ""
	if o := svc.ProxyOverride; o != nil && o.Target != "" {
		ip = byName(ctx, o.Target)
	}
	if ip == "" {
		ip = byLabel(ctx, project, name)
	}
	if ip == "" {
		return ""
	}
	return host.HealthURLAt(ip, port, svc.HealthEndpoint)
}

func sortedNames(names []string) []string {
	out := append([]string(nil), names...)
	sort.Strings(out)
	return out
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
