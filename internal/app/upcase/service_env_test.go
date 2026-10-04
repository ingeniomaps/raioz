package upcase

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/discovery"
	"raioz/internal/domain/models"
)

// Restart and the watcher relaunch a service with ComputedServiceEnv; up
// starts it through buildStartContext. Both must hand the process the same
// computed vars, or a relaunch silently changes what the service sees.
func TestComputedServiceEnv_MatchesUp(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"),
		[]byte(`{"name":"web","scripts":{"dev":"node server.js"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	deps := &models.Deps{
		Project: models.Project{Name: "bencha"},
		Services: map[string]models.Service{
			"web": {Source: models.SourceConfig{Kind: "local", Path: dir}, Port: 38101},
			"api": {Source: models.SourceConfig{Kind: "local", Path: dir}, Port: 38102},
		},
		Infra: map[string]models.InfraEntry{
			"cache": {Inline: &models.Infra{Image: "redis", Tag: "7-alpine", Ports: []string{"36379:6379"}}},
		},
	}
	dm := discovery.NewManager()
	ctx := context.Background()

	detections := BuildDetectionMap(deps)
	portAllocs, err := AllocateHostPorts(deps, detections)
	if err != nil {
		t.Fatalf("AllocateHostPorts: %v", err)
	}
	applyPortAllocs(detections, portAllocs)
	uc := &UseCase{deps: &Dependencies{DiscoveryManager: dm}}
	atUp := uc.buildStartContext("web", deps.Services["web"], detections["web"], startServicesParams{
		deps:       deps,
		detections: detections,
		endpoints:  buildEndpoints(ctx, nil, deps, detections, portAllocs),
		portAllocs: portAllocs,
	}).EnvVars

	got := ComputedServiceEnv(ctx, dm, nil, deps, "", "web")

	if got["API_URL"] != "http://localhost:38102" {
		t.Errorf("API_URL = %q, want http://localhost:38102", got["API_URL"])
	}
	if got["PORT"] != "38101" {
		t.Errorf("PORT = %q, want 38101", got["PORT"])
	}
	if len(got) != len(atUp) {
		t.Errorf("relaunch env has %d vars, up injected %d", len(got), len(atUp))
	}
	for k, want := range atUp {
		if got[k] != want {
			t.Errorf("%s = %q, up injected %q", k, got[k], want)
		}
	}
}

func TestComputedServiceEnv_UnknownService(t *testing.T) {
	deps := &models.Deps{Project: models.Project{Name: "bencha"}}
	if got := ComputedServiceEnv(context.Background(), discovery.NewManager(), nil, deps, "", "ghost"); got != nil {
		t.Errorf("got %v, want nil for an unknown service", got)
	}
}
