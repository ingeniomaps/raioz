package upcase

import (
	"context"
	"os"
	"path/filepath"
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

// A service that names its container with `proxy.target:` is probed on
// that container's address and `proxy.port:` — the container raioz would
// look for by label is not the one that serves, and it may publish no
// host port at all.
func TestContainerHealthURL(t *testing.T) {
	byLabel := func(context.Context, string, string) string { return "10.0.0.2" }
	byName := func(_ context.Context, name string) string {
		if name == "acme-edge" {
			return "10.0.0.9"
		}
		return ""
	}
	dockerfileDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dockerfileDir, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	local := func(path string) models.SourceConfig { return models.SourceConfig{Kind: "local", Path: path} }

	tests := []struct {
		name string
		svc  models.Service
		want string
	}{
		{
			"proxy target and port win",
			models.Service{Source: local(dockerfileDir), Port: 8080, HealthEndpoint: "/healthz",
				ProxyOverride: &models.ServiceProxyOverride{Target: "acme-edge", Port: 9000}},
			"http://10.0.0.9:9000/healthz",
		},
		{
			"proxy target without port: probes port:",
			models.Service{Source: local(dockerfileDir), Port: 8080, HealthEndpoint: "healthz",
				ProxyOverride: &models.ServiceProxyOverride{Target: "acme-edge"}},
			"http://10.0.0.9:8080/healthz",
		},
		{
			"no host port declared, the container still answers",
			models.Service{Source: local(dockerfileDir), HealthEndpoint: "/ready",
				ProxyOverride: &models.ServiceProxyOverride{Target: "acme-edge", Port: 3001}},
			"http://10.0.0.9:3001/ready",
		},
		{
			"host-shaped target falls back to the labelled container",
			models.Service{Source: local(dockerfileDir), Port: 8080, HealthEndpoint: "/h",
				ProxyOverride: &models.ServiceProxyOverride{Target: "host.docker.internal", Port: 8080}},
			"http://10.0.0.2:8080/h",
		},
		{
			"container service without override",
			models.Service{Source: local(dockerfileDir), Port: 8080, HealthEndpoint: "/h"},
			"http://10.0.0.2:8080/h",
		},
		{
			"host service: no container to ask",
			models.Service{Source: models.SourceConfig{Kind: "local", Path: t.TempDir(), Command: "./run"},
				Port: 8080, HealthEndpoint: "/h"},
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainerHealthURL(context.Background(), "acme", "api", tt.svc, byLabel, byName); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
