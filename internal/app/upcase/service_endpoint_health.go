package upcase

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	"raioz/internal/domain/models"
	"raioz/internal/i18n"
	"raioz/internal/output"
)

const (
	// endpointProbeDeadline caps the wait for every declared `health:`
	// endpoint together, not per service. A service that never answers
	// costs the run this much once, not once per service.
	endpointProbeDeadline = 30 * time.Second
	// endpointProbeInterval is the gap between rounds.
	endpointProbeInterval = time.Second
	// endpointProbeTimeout bounds a single request so a service that
	// accepts the connection and then hangs cannot stall the round.
	endpointProbeTimeout = 2 * time.Second
)

// endpointProbe is the HTTP probe waitForServiceEndpoints consults.
// Package var so tests decide the answer without binding a socket —
// same seam as hostStatusPortProbe in the status path.
var endpointProbe = probeHealthEndpoint

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
// allocates the host port at run time and there is no address to hit. That
// case warns once, with the fix in the message, instead of staying silent.
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
			url:  healthURL(svc.Port, svc.HealthEndpoint),
		})
	}

	for _, name := range unprobeable {
		output.PrintWarning(i18n.T("up.health_endpoint_needs_port", name))
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

// healthURL builds the probe address. The endpoint is taken as declared;
// a missing leading slash is the user's typo to see, not ours to guess.
func healthURL(port int, endpoint string) string {
	if len(endpoint) > 0 && endpoint[0] != '/' {
		endpoint = "/" + endpoint
	}
	return fmt.Sprintf("http://127.0.0.1:%d%s", port, endpoint)
}

// probeHealthEndpoint reports whether the endpoint answered without a
// server-side error. Any 2xx/3xx counts: a service that redirects its
// health path to a dashboard is still up, and raioz is not the judge of
// what the body should say.
func probeHealthEndpoint(ctx context.Context, url string) bool {
	reqCtx, cancel := context.WithTimeout(ctx, endpointProbeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode >= 200 && resp.StatusCode < 400
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
