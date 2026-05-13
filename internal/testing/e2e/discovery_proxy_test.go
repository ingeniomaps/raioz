package e2e

import (
	"context"
	"strings"
	"testing"

	"raioz/internal/discovery"
	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/proxy"
)

// TestFullFlow_ServiceDiscovery verifies that discovery generates correct
// env vars for all cross-runtime communication scenarios.
func TestFullFlow_ServiceDiscovery(t *testing.T) {
	mgr := discovery.NewManager()

	endpoints := map[string]interfaces.ServiceEndpoint{
		"api":      {Name: "api", Runtime: models.RuntimeDockerfile, Host: "raioz-app-api", Port: 3000},
		"frontend": {Name: "frontend", Runtime: models.RuntimeNPM, Host: "localhost", Port: 8080},
		"postgres": {Name: "postgres", Runtime: models.RuntimeImage, Host: "raioz-app-postgres", Port: 5432},
		"redis":    {Name: "redis", Runtime: models.RuntimeImage, Host: "raioz-app-redis", Port: 6379},
	}

	// Scenario 1: Docker service (api) calling other Docker services
	apiVars := mgr.GenerateEnvVars("api", models.RuntimeDockerfile, endpoints, false)

	if apiVars["POSTGRES_HOST"] != "raioz-app-postgres" {
		t.Errorf("api→postgres: expected container name, got %q", apiVars["POSTGRES_HOST"])
	}
	if apiVars["REDIS_HOST"] != "raioz-app-redis" {
		t.Errorf("api→redis: expected container name, got %q", apiVars["REDIS_HOST"])
	}
	if apiVars["FRONTEND_HOST"] != "host.docker.internal" {
		t.Errorf("api→frontend(host): expected host.docker.internal, got %q", apiVars["FRONTEND_HOST"])
	}

	// Scenario 2: Host service (frontend) calling Docker services
	frontVars := mgr.GenerateEnvVars("frontend", models.RuntimeNPM, endpoints, false)

	if frontVars["API_HOST"] != "localhost" {
		t.Errorf("frontend(host)→api(docker): expected localhost, got %q", frontVars["API_HOST"])
	}
	if frontVars["POSTGRES_HOST"] != "localhost" {
		t.Errorf("frontend(host)→postgres(docker): expected localhost, got %q", frontVars["POSTGRES_HOST"])
	}

	// Scenario 3: With proxy enabled
	apiVarsProxy := mgr.GenerateEnvVars("api", models.RuntimeDockerfile, endpoints, true)

	if apiVarsProxy["POSTGRES_HTTPS_URL"] != "https://postgres.localhost" {
		t.Errorf("expected HTTPS URL with proxy, got %q", apiVarsProxy["POSTGRES_HTTPS_URL"])
	}
	if apiVarsProxy["FRONTEND_HTTPS_URL"] != "https://frontend.localhost" {
		t.Errorf("expected frontend HTTPS URL, got %q", apiVarsProxy["FRONTEND_HTTPS_URL"])
	}

	// Scenario 4: Metadata always present
	if apiVars["RAIOZ_SERVICE"] != "api" {
		t.Errorf("expected RAIOZ_SERVICE=api, got %q", apiVars["RAIOZ_SERVICE"])
	}
	if apiVars["RAIOZ_RUNTIME"] != "dockerfile" {
		t.Errorf("expected RAIOZ_RUNTIME=dockerfile, got %q", apiVars["RAIOZ_RUNTIME"])
	}
}

// TestFullFlow_ProxyCaddyfile verifies that the proxy generates correct
// Caddyfile config for all routing types.
func TestFullFlow_ProxyCaddyfile(t *testing.T) {
	mgr := proxy.NewManager("")

	// Add routes for different service types
	mgr.AddRoute(context.Background(), interfaces.ProxyRoute{
		ServiceName: "api",
		Hostname:    "api",
		Target:      "raioz-app-api",
		Port:        3000,
	})
	mgr.AddRoute(context.Background(), interfaces.ProxyRoute{
		ServiceName: "frontend",
		Hostname:    "frontend",
		Target:      "host.docker.internal",
		Port:        8080,
	})
	mgr.AddRoute(context.Background(), interfaces.ProxyRoute{
		ServiceName: "chat",
		Hostname:    "chat",
		Target:      "raioz-app-chat",
		Port:        3001,
		WebSocket:   true,
	})
	mgr.AddRoute(context.Background(), interfaces.ProxyRoute{
		ServiceName: "events",
		Hostname:    "events",
		Target:      "raioz-app-events",
		Port:        3002,
		Stream:      true,
	})
	mgr.AddRoute(context.Background(), interfaces.ProxyRoute{
		ServiceName: "grpc-gw",
		Hostname:    "grpc-gw",
		Target:      "raioz-app-grpc",
		Port:        50051,
		GRPC:        true,
	})

	content := mgr.GenerateCaddyfileContent()

	// Verify all routes are present
	tests := []struct {
		name     string
		contains string
	}{
		{"api route", "api.localhost"},
		{"api target", "raioz-app-api:3000"},
		{"frontend route", "frontend.localhost"},
		{"frontend target to host", "host.docker.internal:8080"},
		{"chat websocket", "header_up X-Forwarded-Proto"},
		{"events streaming", "flush_interval -1"},
		{"grpc h2c", "h2c://"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(content, tt.contains) {
				t.Errorf("Caddyfile missing %q.\nGot:\n%s", tt.contains, content)
			}
		})
	}

	// Verify URLs
	if url := mgr.GetURL("api"); url != "https://api.localhost" {
		t.Errorf("GetURL(api) = %q, want 'https://api.localhost'", url)
	}
	if url := mgr.GetURL("frontend"); url != "https://frontend.localhost" {
		t.Errorf("GetURL(frontend) = %q, want 'https://frontend.localhost'", url)
	}
}

// TestFullFlow_DiscoveryWithAllRuntimes verifies discovery works for
// every combination of runtime pairs.
func TestFullFlow_DiscoveryWithAllRuntimes(t *testing.T) {
	mgr := discovery.NewManager()

	runtimes := []struct {
		name    string
		runtime models.Runtime
		host    string
	}{
		{"compose-svc", models.RuntimeCompose, "compose-container"},
		{"docker-svc", models.RuntimeDockerfile, "docker-container"},
		{"image-svc", models.RuntimeImage, "image-container"},
		{"npm-svc", models.RuntimeNPM, "localhost"},
		{"go-svc", models.RuntimeGo, "localhost"},
		{"make-svc", models.RuntimeMake, "localhost"},
		{"python-svc", models.RuntimePython, "localhost"},
		{"rust-svc", models.RuntimeRust, "localhost"},
	}

	endpoints := make(map[string]interfaces.ServiceEndpoint)
	for _, r := range runtimes {
		endpoints[r.name] = interfaces.ServiceEndpoint{
			Name: r.name, Runtime: r.runtime, Host: r.host, Port: 3000,
		}
	}

	// For each runtime, generate env vars and verify no panics and correct HOST resolution
	for _, caller := range runtimes {
		vars := mgr.GenerateEnvVars(caller.name, caller.runtime, endpoints, false)

		// Should not contain self
		selfKey := strings.ToUpper(strings.ReplaceAll(caller.name, "-", "_")) + "_HOST"
		if _, exists := vars[selfKey]; exists {
			t.Errorf("%s: should not contain self in env vars", caller.name)
		}

		// Docker callers should see host.docker.internal for host targets
		if caller.runtime == models.RuntimeDockerfile || caller.runtime == models.RuntimeCompose || caller.runtime == models.RuntimeImage {
			if vars["NPM_SVC_HOST"] != "host.docker.internal" {
				t.Errorf("%s(docker) → npm-svc(host): expected host.docker.internal, got %q",
					caller.name, vars["NPM_SVC_HOST"])
			}
		}

		// Host callers should see localhost for docker targets
		if caller.runtime == models.RuntimeNPM || caller.runtime == models.RuntimeGo {
			if vars["COMPOSE_SVC_HOST"] != "localhost" {
				t.Errorf("%s(host) → compose-svc(docker): expected localhost, got %q",
					caller.name, vars["COMPOSE_SVC_HOST"])
			}
		}
	}
}
