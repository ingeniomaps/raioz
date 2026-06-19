package discovery

import (
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
)

func TestGenerateEnvVars_ContainerToContainer(t *testing.T) {
	m := NewManager()
	endpoints := map[string]interfaces.ServiceEndpoint{
		"postgres": {Name: "postgres", Runtime: models.RuntimeImage, Host: "postgres", Port: 5432},
		"api":      {Name: "api", Runtime: models.RuntimeDockerfile, Host: "api-container", Port: 3000},
	}

	vars := m.GenerateEnvVars("api", models.RuntimeDockerfile, endpoints, false)

	// api is Docker, postgres is Docker → use container name
	if vars["POSTGRES_HOST"] != "postgres" {
		t.Errorf("expected POSTGRES_HOST=postgres, got %s", vars["POSTGRES_HOST"])
	}
	if vars["POSTGRES_PORT"] != "5432" {
		t.Errorf("expected POSTGRES_PORT=5432, got %s", vars["POSTGRES_PORT"])
	}
	if vars["POSTGRES_URL"] != "http://postgres:5432" {
		t.Errorf("expected POSTGRES_URL=http://postgres:5432, got %s", vars["POSTGRES_URL"])
	}
}

func TestGenerateEnvVars_HostToContainer(t *testing.T) {
	m := NewManager()
	endpoints := map[string]interfaces.ServiceEndpoint{
		"postgres": {Name: "postgres", Runtime: models.RuntimeImage, Host: "postgres", Port: 5432},
		"cart-api": {Name: "cart-api", Runtime: models.RuntimeNPM, Host: "localhost", Port: 3001},
	}

	vars := m.GenerateEnvVars("cart-api", models.RuntimeNPM, endpoints, false)

	// cart-api is host, postgres is Docker → use localhost
	if vars["POSTGRES_HOST"] != "localhost" {
		t.Errorf("expected POSTGRES_HOST=localhost, got %s", vars["POSTGRES_HOST"])
	}
}

func TestGenerateEnvVars_ContainerToHost(t *testing.T) {
	m := NewManager()
	endpoints := map[string]interfaces.ServiceEndpoint{
		"auth-api": {Name: "auth-api", Runtime: models.RuntimeDockerfile, Host: "auth-container", Port: 3000},
		"cart-api": {Name: "cart-api", Runtime: models.RuntimeNPM, Host: "localhost", Port: 3001},
	}

	vars := m.GenerateEnvVars("auth-api", models.RuntimeDockerfile, endpoints, false)

	// auth-api is Docker, cart-api is host → use host.docker.internal
	if vars["CART_API_HOST"] != "host.docker.internal" {
		t.Errorf("expected CART_API_HOST=host.docker.internal, got %s", vars["CART_API_HOST"])
	}
	if vars["CART_API_PORT"] != "3001" {
		t.Errorf("expected CART_API_PORT=3001, got %s", vars["CART_API_PORT"])
	}
}

func TestGenerateEnvVars_HostToContainerUsesHostPort(t *testing.T) {
	// When a dependency is published to a host port different from the
	// container port (e.g. publish: true bumped 5432→5433), host callers
	// must see the HOST port, not the container one. This exercises the
	// phase-2 HostPort/Port split on ServiceEndpoint.
	m := NewManager()
	endpoints := map[string]interfaces.ServiceEndpoint{
		"postgres": {
			Name:     "postgres",
			Runtime:  models.RuntimeImage,
			Host:     "raioz-app-postgres",
			Port:     5432, // container port
			HostPort: 5433, // published host port (bumped because 5432 was taken)
		},
		"api": {Name: "api", Runtime: models.RuntimeNPM, Host: "localhost", Port: 3000},
	}

	vars := m.GenerateEnvVars("api", models.RuntimeNPM, endpoints, false)

	if vars["POSTGRES_HOST"] != "localhost" {
		t.Errorf("POSTGRES_HOST = %q, want localhost", vars["POSTGRES_HOST"])
	}
	if vars["POSTGRES_PORT"] != "5433" {
		t.Errorf("POSTGRES_PORT = %q, want 5433 (the host port)", vars["POSTGRES_PORT"])
	}
	if vars["POSTGRES_URL"] != "http://localhost:5433" {
		t.Errorf("POSTGRES_URL = %q, want http://localhost:5433", vars["POSTGRES_URL"])
	}
}

func TestGenerateEnvVars_ContainerToContainerIgnoresHostPort(t *testing.T) {
	// Inside the Docker network, container→container traffic must use the
	// *container* port on the DNS name, even when HostPort is set. Using
	// HostPort here would break intra-network communication the moment the
	// dev pinned a non-matching publish port.
	m := NewManager()
	endpoints := map[string]interfaces.ServiceEndpoint{
		"postgres": {
			Name:     "postgres",
			Runtime:  models.RuntimeImage,
			Host:     "raioz-app-postgres",
			Port:     5432,
			HostPort: 5433,
		},
		"api": {
			Name:    "api",
			Runtime: models.RuntimeDockerfile,
			Host:    "raioz-app-api",
			Port:    3000,
		},
	}

	vars := m.GenerateEnvVars("api", models.RuntimeDockerfile, endpoints, false)

	if vars["POSTGRES_HOST"] != "raioz-app-postgres" {
		t.Errorf("POSTGRES_HOST = %q, want container DNS", vars["POSTGRES_HOST"])
	}
	if vars["POSTGRES_PORT"] != "5432" {
		t.Errorf("POSTGRES_PORT = %q, want 5432 (container port)", vars["POSTGRES_PORT"])
	}
}

func TestGenerateEnvVars_HostToHost(t *testing.T) {
	m := NewManager()
	endpoints := map[string]interfaces.ServiceEndpoint{
		"admin":    {Name: "admin", Runtime: models.RuntimeGo, Host: "localhost", Port: 8080},
		"cart-api": {Name: "cart-api", Runtime: models.RuntimeNPM, Host: "localhost", Port: 3001},
	}

	vars := m.GenerateEnvVars("admin", models.RuntimeGo, endpoints, false)

	if vars["CART_API_HOST"] != "localhost" {
		t.Errorf("expected CART_API_HOST=localhost, got %s", vars["CART_API_HOST"])
	}
}

func TestGenerateEnvVars_WithProxy(t *testing.T) {
	m := NewManager()
	endpoints := map[string]interfaces.ServiceEndpoint{
		"api":      {Name: "api", Runtime: models.RuntimeDockerfile, Host: "api", Port: 3000},
		"postgres": {Name: "postgres", Runtime: models.RuntimeImage, Host: "postgres", Port: 5432},
	}

	vars := m.GenerateEnvVars("api", models.RuntimeDockerfile, endpoints, true)

	if vars["POSTGRES_HTTPS_URL"] != "https://postgres.localhost" {
		t.Errorf("expected POSTGRES_HTTPS_URL=https://postgres.localhost, got %s", vars["POSTGRES_HTTPS_URL"])
	}
}

// Regression: when RAIOZ_ROUTER_ACTIVE=1, the bundled
// Caddy is suppressed (ADR-037) and `*.localhost` resolves to
// nothing raioz controls. _HTTPS_URL must NOT be emitted in that
// case — emitting it would mislead consumers about who serves
// the URL.
func TestGenerateEnvVars_RouterActiveSuppressesHTTPSURL(t *testing.T) {
	t.Setenv("RAIOZ_ROUTER_ACTIVE", "1")
	m := NewManager()
	endpoints := map[string]interfaces.ServiceEndpoint{
		"api":      {Name: "api", Runtime: models.RuntimeDockerfile, Host: "api", Port: 3000},
		"postgres": {Name: "postgres", Runtime: models.RuntimeImage, Host: "postgres", Port: 5432},
	}

	vars := m.GenerateEnvVars("api", models.RuntimeDockerfile, endpoints, true)

	if _, present := vars["POSTGRES_HTTPS_URL"]; present {
		t.Errorf("POSTGRES_HTTPS_URL must NOT be set under RAIOZ_ROUTER_ACTIVE; got %q",
			vars["POSTGRES_HTTPS_URL"])
	}
	// Sanity: the URL/HOST/PORT trio that points at real endpoints
	// is still emitted.
	if vars["POSTGRES_HOST"] == "" {
		t.Errorf("POSTGRES_HOST should still be emitted; got empty")
	}
}

func TestGenerateEnvVars_SkipsSelf(t *testing.T) {
	m := NewManager()
	endpoints := map[string]interfaces.ServiceEndpoint{
		"api": {Name: "api", Runtime: models.RuntimeDockerfile, Host: "api", Port: 3000},
	}

	vars := m.GenerateEnvVars("api", models.RuntimeDockerfile, endpoints, false)

	if _, exists := vars["API_HOST"]; exists {
		t.Error("should not include self in env vars")
	}
}

func TestGenerateEnvVars_Metadata(t *testing.T) {
	m := NewManager()
	vars := m.GenerateEnvVars("api", models.RuntimeGo, nil, false)

	if vars["RAIOZ_SERVICE"] != "api" {
		t.Errorf("expected RAIOZ_SERVICE=api, got %s", vars["RAIOZ_SERVICE"])
	}
	if vars["RAIOZ_RUNTIME"] != "go" {
		t.Errorf("expected RAIOZ_RUNTIME=go, got %s", vars["RAIOZ_RUNTIME"])
	}
}

func TestToEnvPrefix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"postgres", "POSTGRES"},
		{"auth-api", "AUTH_API"},
		{"my.service", "MY_SERVICE"},
		{"cart-api-v2", "CART_API_V2"},
	}
	for _, tt := range tests {
		got := toEnvPrefix(tt.input)
		if got != tt.want {
			t.Errorf("toEnvPrefix(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGenerateEnvVars_SchemeFromEndpoint(t *testing.T) {
	// A non-HTTP dep (redis) carries its own URL scheme so the host caller
	// gets redis://… — an http://… URL is unparseable by redis clients.
	m := NewManager()
	endpoints := map[string]interfaces.ServiceEndpoint{
		"redis": {
			Name:     "redis",
			Runtime:  models.RuntimeImage,
			Host:     "raioz-app-redis",
			Port:     6379,
			HostPort: 6379,
			Scheme:   "redis",
		},
		"api": {Name: "api", Runtime: models.RuntimeNPM, Host: "localhost", Port: 3000},
	}

	vars := m.GenerateEnvVars("api", models.RuntimeNPM, endpoints, false)

	if vars["REDIS_URL"] != "redis://localhost:6379" {
		t.Errorf("REDIS_URL = %q, want redis://localhost:6379", vars["REDIS_URL"])
	}
}

func TestGenerateEnvVars_EmptySchemeDefaultsToHTTP(t *testing.T) {
	// An endpoint without an explicit scheme keeps the historical http://
	// default, so HTTP services are unaffected.
	m := NewManager()
	endpoints := map[string]interfaces.ServiceEndpoint{
		"web": {Name: "web", Runtime: models.RuntimeNPM, Host: "localhost", Port: 8080},
		"api": {Name: "api", Runtime: models.RuntimeNPM, Host: "localhost", Port: 3000},
	}

	vars := m.GenerateEnvVars("api", models.RuntimeNPM, endpoints, false)

	if vars["WEB_URL"] != "http://localhost:8080" {
		t.Errorf("WEB_URL = %q, want http://localhost:8080", vars["WEB_URL"])
	}
}
