package discovery

import (
	"testing"

	"raioz/internal/detect"
	"raioz/internal/domain/interfaces"
)

func TestGenerateEnvVars_ContainerToContainer(t *testing.T) {
	m := NewManager()
	endpoints := map[string]interfaces.ServiceEndpoint{
		"postgres": {Name: "postgres", Runtime: detect.RuntimeImage, Host: "postgres", Port: 5432},
		"api":      {Name: "api", Runtime: detect.RuntimeDockerfile, Host: "api-container", Port: 3000},
	}

	vars := m.GenerateEnvVars("api", detect.RuntimeDockerfile, endpoints, false)

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
		"postgres":  {Name: "postgres", Runtime: detect.RuntimeImage, Host: "postgres", Port: 5432},
		"cart-api":  {Name: "cart-api", Runtime: detect.RuntimeNPM, Host: "localhost", Port: 3001},
	}

	vars := m.GenerateEnvVars("cart-api", detect.RuntimeNPM, endpoints, false)

	// cart-api is host, postgres is Docker → use localhost
	if vars["POSTGRES_HOST"] != "localhost" {
		t.Errorf("expected POSTGRES_HOST=localhost, got %s", vars["POSTGRES_HOST"])
	}
}

func TestGenerateEnvVars_ContainerToHost(t *testing.T) {
	m := NewManager()
	endpoints := map[string]interfaces.ServiceEndpoint{
		"auth-api": {Name: "auth-api", Runtime: detect.RuntimeDockerfile, Host: "auth-container", Port: 3000},
		"cart-api": {Name: "cart-api", Runtime: detect.RuntimeNPM, Host: "localhost", Port: 3001},
	}

	vars := m.GenerateEnvVars("auth-api", detect.RuntimeDockerfile, endpoints, false)

	// auth-api is Docker, cart-api is host → use host.docker.internal
	if vars["CART_API_HOST"] != "host.docker.internal" {
		t.Errorf("expected CART_API_HOST=host.docker.internal, got %s", vars["CART_API_HOST"])
	}
	if vars["CART_API_PORT"] != "3001" {
		t.Errorf("expected CART_API_PORT=3001, got %s", vars["CART_API_PORT"])
	}
}

func TestGenerateEnvVars_HostToHost(t *testing.T) {
	m := NewManager()
	endpoints := map[string]interfaces.ServiceEndpoint{
		"admin":    {Name: "admin", Runtime: detect.RuntimeGo, Host: "localhost", Port: 8080},
		"cart-api": {Name: "cart-api", Runtime: detect.RuntimeNPM, Host: "localhost", Port: 3001},
	}

	vars := m.GenerateEnvVars("admin", detect.RuntimeGo, endpoints, false)

	if vars["CART_API_HOST"] != "localhost" {
		t.Errorf("expected CART_API_HOST=localhost, got %s", vars["CART_API_HOST"])
	}
}

func TestGenerateEnvVars_WithProxy(t *testing.T) {
	m := NewManager()
	endpoints := map[string]interfaces.ServiceEndpoint{
		"api":      {Name: "api", Runtime: detect.RuntimeDockerfile, Host: "api", Port: 3000},
		"postgres": {Name: "postgres", Runtime: detect.RuntimeImage, Host: "postgres", Port: 5432},
	}

	vars := m.GenerateEnvVars("api", detect.RuntimeDockerfile, endpoints, true)

	if vars["POSTGRES_HTTPS_URL"] != "https://postgres.localhost" {
		t.Errorf("expected POSTGRES_HTTPS_URL=https://postgres.localhost, got %s", vars["POSTGRES_HTTPS_URL"])
	}
}

func TestGenerateEnvVars_SkipsSelf(t *testing.T) {
	m := NewManager()
	endpoints := map[string]interfaces.ServiceEndpoint{
		"api": {Name: "api", Runtime: detect.RuntimeDockerfile, Host: "api", Port: 3000},
	}

	vars := m.GenerateEnvVars("api", detect.RuntimeDockerfile, endpoints, false)

	if _, exists := vars["API_HOST"]; exists {
		t.Error("should not include self in env vars")
	}
}

func TestGenerateEnvVars_Metadata(t *testing.T) {
	m := NewManager()
	vars := m.GenerateEnvVars("api", detect.RuntimeGo, nil, false)

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
