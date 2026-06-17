package upcase

import (
	"context"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/naming"
)

// stubPublishedHostPort swaps the live-port lookup for a deterministic map
// keyed by container name, restoring the real one after the test.
func stubPublishedHostPort(t *testing.T, ports map[string]int) {
	t.Helper()
	prev := publishedHostPortFn
	publishedHostPortFn = func(_ context.Context, container string, _ int) (int, error) {
		return ports[container], nil
	}
	t.Cleanup(func() { publishedHostPortFn = prev })
}

// sharedDepDeps builds a Deps where dep "redis" is workspace-shared via a
// container-name override (so IsSharedDep is true without touching the global
// prefix). explicit controls whether the allocation is a user-pinned port.
func sharedDepResult(explicit bool, hostPort int) (*models.Deps, *PortAllocResult) {
	deps := &models.Deps{
		Project: models.Project{Name: "api"},
		Infra: map[string]models.InfraEntry{
			"redis": {Inline: &models.Infra{
				Name:    "shared-redis",
				Expose:  []int{6379},
				Publish: &models.PublishSpec{Auto: !explicit},
			}},
		},
	}
	result := &PortAllocResult{
		Deps: map[string]DepPortAllocation{
			"redis": {
				Name:     "redis",
				Explicit: explicit,
				Mappings: []DepPortMapping{{HostPort: hostPort, ContainerPort: 6379}},
			},
		},
	}
	return deps, result
}

func TestReuseSharedDepHostPorts(t *testing.T) {
	t.Run("pins to live port when shared dep already published", func(t *testing.T) {
		stubPublishedHostPort(t, map[string]int{"shared-redis": 6379})
		deps, result := sharedDepResult(false, 6380) // allocator bumped to 6380
		reuseSharedDepHostPorts(context.Background(), deps, result)
		if got := result.Deps["redis"].Mappings[0].HostPort; got != 6379 {
			t.Errorf("HostPort = %d, want 6379 (reused live port)", got)
		}
	})

	t.Run("leaves explicit pin untouched", func(t *testing.T) {
		stubPublishedHostPort(t, map[string]int{"shared-redis": 6379})
		deps, result := sharedDepResult(true, 6380)
		reuseSharedDepHostPorts(context.Background(), deps, result)
		if got := result.Deps["redis"].Mappings[0].HostPort; got != 6380 {
			t.Errorf("HostPort = %d, want 6380 (explicit pin preserved)", got)
		}
	})

	t.Run("no live container leaves allocation untouched", func(t *testing.T) {
		stubPublishedHostPort(t, map[string]int{}) // returns 0 → not running
		deps, result := sharedDepResult(false, 6380)
		reuseSharedDepHostPorts(context.Background(), deps, result)
		if got := result.Deps["redis"].Mappings[0].HostPort; got != 6380 {
			t.Errorf("HostPort = %d, want 6380 (no live port to reuse)", got)
		}
	})

	t.Run("per-project dep is never rewritten", func(t *testing.T) {
		prev := naming.GetPrefix()
		naming.SetPrefix(naming.DefaultPrefix) // no workspace → not shared
		t.Cleanup(func() { naming.SetPrefix(prev) })

		stubPublishedHostPort(t, map[string]int{"raioz-api-redis": 6379})
		deps := &models.Deps{
			Project: models.Project{Name: "api"},
			Infra: map[string]models.InfraEntry{
				"redis": {Inline: &models.Infra{
					Expose:  []int{6379},
					Publish: &models.PublishSpec{Auto: true},
				}},
			},
		}
		result := &PortAllocResult{Deps: map[string]DepPortAllocation{
			"redis": {Name: "redis", Mappings: []DepPortMapping{{HostPort: 6380, ContainerPort: 6379}}},
		}}
		reuseSharedDepHostPorts(context.Background(), deps, result)
		if got := result.Deps["redis"].Mappings[0].HostPort; got != 6380 {
			t.Errorf("HostPort = %d, want 6380 (per-project dep not shared)", got)
		}
	})
}
