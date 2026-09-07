package upcase

import (
	"context"
	"strings"
	"sync"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/i18n"
)

// stubEndpointProbe replaces the probe waitForServiceEndpoints consults
// and records every URL it was asked about.
func stubEndpointProbe(t *testing.T, answer func(url string) bool) *[]string {
	t.Helper()
	var mu sync.Mutex
	seen := []string{}
	prev := endpointProbe
	endpointProbe = func(_ context.Context, url string) bool {
		mu.Lock()
		seen = append(seen, url)
		mu.Unlock()
		return answer(url)
	}
	t.Cleanup(func() { endpointProbe = prev })
	return &seen
}

func depsWithService(name string, port int, health string) *models.Deps {
	return &models.Deps{
		Project: models.Project{Name: "test"},
		Services: map[string]models.Service{
			name: {
				Source:         models.SourceConfig{Path: "."},
				Port:           port,
				HealthEndpoint: health,
			},
		},
		Infra: map[string]models.InfraEntry{},
	}
}

// The regression this change exists for: `health:` was parsed and read by
// nobody, so a declared endpoint was never contacted.
func TestWaitForServiceEndpoints_ProbesDeclaredEndpoint(t *testing.T) {
	i18n.Init("en")
	seen := stubEndpointProbe(t, func(string) bool { return true })

	out := captureStdoutForLog(t, func() {
		waitForServiceEndpoints(t.Context(), depsWithService("api", 3000, "/api/health"), []string{"api"})
	})

	if len(*seen) != 1 || (*seen)[0] != "http://127.0.0.1:3000/api/health" {
		t.Fatalf("probed %v, want the declared endpoint once", *seen)
	}
	if !strings.Contains(out, "api") {
		t.Errorf("output = %q, want it to name the service that answered", out)
	}
}

// A service without `port:` cannot be probed, and saying so on every up
// would be noise: declaring health: without port: is the common shape for
// anything reached through the proxy. It goes to the debug log instead.
func TestWaitForServiceEndpoints_QuietWithoutPort(t *testing.T) {
	i18n.Init("en")
	seen := stubEndpointProbe(t, func(string) bool { return true })

	out := captureStdoutForLog(t, func() {
		waitForServiceEndpoints(t.Context(), depsWithService("api", 0, "/api/health"), []string{"api"})
	})

	if len(*seen) != 0 {
		t.Errorf("probed %v, want no probe without a port", *seen)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("output = %q, want silence on stdout", out)
	}
}

// No `health:` declared means nothing to probe and nothing to say.
func TestWaitForServiceEndpoints_SkipsUndeclared(t *testing.T) {
	i18n.Init("en")
	seen := stubEndpointProbe(t, func(string) bool { return true })

	out := captureStdoutForLog(t, func() {
		waitForServiceEndpoints(t.Context(), depsWithService("api", 3000, ""), []string{"api"})
	})

	if len(*seen) != 0 {
		t.Errorf("probed %v, want no probe for a service without health:", *seen)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("output = %q, want silence", out)
	}
}

// A dead endpoint must not abort the run: a slow boot and a broken service
// look the same from here.
func TestWaitForServiceEndpoints_TimeoutWarnsAndReturns(t *testing.T) {
	i18n.Init("en")
	stubEndpointProbe(t, func(string) bool { return false })

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // a cancelled context ends the wait on the first round

	out := captureStdoutForLog(t, func() {
		waitForServiceEndpoints(ctx, depsWithService("api", 3000, "/health"), []string{"api"})
	})

	if !strings.Contains(out, "api") || !strings.Contains(out, "/health") {
		t.Errorf("output = %q, want a warning naming the service and the endpoint", out)
	}
}
