package upcase

import (
	"context"
	"sort"
	"strings"
	"time"

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
// Only services that also declare `port:` can be probed: without it raioz
// allocates the host port at run time and there is no address to hit.
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
	}

	var targets []target
	var unprobeable []string

	for _, name := range sortedNames(serviceNames) {
		svc, ok := deps.Services[name]
		if !ok || svc.HealthEndpoint == "" {
			continue
		}
		if svc.Port <= 0 {
			unprobeable = append(unprobeable, name)
			continue
		}
		targets = append(targets, target{
			name: name,
			url:  host.HealthURL(svc.Port, svc.HealthEndpoint),
		})
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
	for _, t := range targets {
		pending[t.name] = t.url
	}

	deadline := time.Now().Add(endpointProbeDeadline)
	for {
		for name, url := range pending {
			if endpointProbe(ctx, url) {
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
