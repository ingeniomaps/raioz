package upcase

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
)

func TestApplyInternalDepPort(t *testing.T) {
	prev := imageExposedPortFn
	imageExposedPortFn = func(_ context.Context, image string) (int, error) {
		if image == "redis:7" {
			return 6379, nil
		}
		return 0, errors.New("unknown image")
	}
	t.Cleanup(func() { imageExposedPortFn = prev })

	tests := []struct {
		name     string
		infra    *models.Infra
		ep       interfaces.ServiceEndpoint
		wantPort int
		wantOnly bool
	}{
		{"declared expose", &models.Infra{Image: "nginx", Expose: []int{8080}}, interfaces.ServiceEndpoint{}, 8080, true},
		{"port the image exposes", &models.Infra{Image: "redis", Tag: "7"}, interfaces.ServiceEndpoint{}, 6379, true},
		{"image that exposes nothing known", &models.Infra{Image: "scratch"}, interfaces.ServiceEndpoint{}, 0, false},
		{
			"published dependency keeps its mapping",
			&models.Infra{Image: "redis", Tag: "7"},
			interfaces.ServiceEndpoint{Port: 6379, HostPort: 36379}, 6379, false,
		},
		{"compose dependency", &models.Infra{Compose: []string{"./x.yml"}}, interfaces.ServiceEndpoint{}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := &models.Deps{Infra: map[string]models.InfraEntry{"dep": {Inline: tt.infra}}}
			ep := tt.ep
			applyInternalDepPort(context.Background(), &ep, "dep", deps)
			if ep.Port != tt.wantPort || ep.ContainerOnly != tt.wantOnly {
				t.Errorf("Port=%d ContainerOnly=%v, want %d/%v", ep.Port, ep.ContainerOnly, tt.wantPort, tt.wantOnly)
			}
		})
	}
}

func TestComposeDepPort(t *testing.T) {
	prev := imageExposedPortFn
	imageExposedPortFn = func(_ context.Context, image string) (int, error) {
		if image == "redis:7.4-alpine" {
			return 6379, nil
		}
		return 0, errors.New("unknown image")
	}
	t.Cleanup(func() { imageExposedPortFn = prev })

	write := func(body string) []string {
		path := filepath.Join(t.TempDir(), "compose.yml")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return []string{path}
	}

	tests := []struct {
		name      string
		compose   string
		wantPort  int
		wantImage string
	}{
		{"only service, port from the image", "services:\n  cache:\n    image: redis:7.4-alpine\n", 6379, "redis:7.4-alpine"},
		{"expose wins", "services:\n  kv:\n    image: redis:7.4-alpine\n    expose: [7000]\n", 7000, "redis:7.4-alpine"},
		{"container side of ports", "services:\n  kv:\n    image: x\n    ports: [\"127.0.0.1:5433:5432/tcp\"]\n", 5432, "x"},
		{"long form ports", "services:\n  kv:\n    image: x\n    ports:\n      - target: 9000\n        published: 9001\n", 9000, "x"},
		{"service named like the dependency", "services:\n  other:\n    image: x\n    expose: [1]\n  kv:\n    image: y\n    expose: [2]\n", 2, "y"},
		{"several services, none matches", "services:\n  a:\n    image: x\n    expose: [1]\n  b:\n    image: y\n    expose: [2]\n", 0, ""},
		{"interpolated port is not guessed", "services:\n  kv:\n    image: x\n    ports: [\"${PORT}:${PORT}\"]\n", 0, "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port, image := composeDepPort(context.Background(), write(tt.compose), "kv")
			if port != tt.wantPort || image != tt.wantImage {
				t.Errorf("got %d %q, want %d %q", port, image, tt.wantPort, tt.wantImage)
			}
		})
	}
}

func TestCheckProxyAddress(t *testing.T) {
	off := false
	tests := []struct {
		name    string
		subnet  string
		cfg     *models.ProxyConfig
		wantErr bool
	}{
		{"published without a subnet is fine", "", &models.ProxyConfig{}, false},
		{"unpublished with a subnet", "10.9.0.0/16", &models.ProxyConfig{Publish: &off}, false},
		{"unpublished without any address source", "", &models.ProxyConfig{Publish: &off}, true},
		{"explicit ip inside the subnet", "10.9.0.0/16", &models.ProxyConfig{Publish: &off, IP: "10.9.1.1"}, false},
		{"explicit ip outside the subnet", "10.9.0.0/16", &models.ProxyConfig{IP: "10.99.0.1"}, true},
		{"explicit ip without a subnet", "", &models.ProxyConfig{IP: "10.9.1.1"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := &models.Deps{Proxy: true, ProxyConfig: tt.cfg}
			if tt.subnet != "" {
				deps.Network = models.NetworkConfig{Name: "n", Subnet: tt.subnet, IsObject: true}
			}
			if err := checkProxyAddress(deps); (err != nil) != tt.wantErr {
				t.Errorf("checkProxyAddress = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
