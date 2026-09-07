package upcase

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func TestHealthURL(t *testing.T) {
	cases := []struct {
		name     string
		port     int
		endpoint string
		want     string
	}{
		{"leading slash kept", 3000, "/api/health", "http://127.0.0.1:3000/api/health"},
		{"missing slash added", 8080, "health/ready", "http://127.0.0.1:8080/health/ready"},
		{"root endpoint", 9000, "/", "http://127.0.0.1:9000/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := healthURL(tc.port, tc.endpoint); got != tc.want {
				t.Errorf("healthURL(%d, %q) = %q, want %q", tc.port, tc.endpoint, got, tc.want)
			}
		})
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

// A service without `port:` has no address to probe. Silence would put the
// user back where the field started, so it says so.
func TestWaitForServiceEndpoints_WarnsWithoutPort(t *testing.T) {
	i18n.Init("en")
	seen := stubEndpointProbe(t, func(string) bool { return true })

	out := captureStdoutForLog(t, func() {
		waitForServiceEndpoints(t.Context(), depsWithService("api", 0, "/api/health"), []string{"api"})
	})

	if len(*seen) != 0 {
		t.Errorf("probed %v, want no probe without a port", *seen)
	}
	if !strings.Contains(out, "api") || !strings.Contains(out, "port") {
		t.Errorf("output = %q, want a warning naming the service and the missing port", out)
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

func TestProbeHealthEndpoint(t *testing.T) {
	cases := []struct {
		name string
		code int
		want bool
	}{
		{"200 is up", http.StatusOK, true},
		{"204 is up", http.StatusNoContent, true},
		{"302 is up", http.StatusFound, true},
		{"404 is not", http.StatusNotFound, false},
		{"500 is not", http.StatusInternalServerError, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.code)
			}))
			defer srv.Close()
			if got := probeHealthEndpoint(t.Context(), srv.URL); got != tc.want {
				t.Errorf("probeHealthEndpoint(%d) = %v, want %v", tc.code, got, tc.want)
			}
		})
	}
}

func TestProbeHealthEndpoint_NobodyListening(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // the port is now closed

	if probeHealthEndpoint(t.Context(), url) {
		t.Error("probeHealthEndpoint answered true with nobody listening")
	}
}
