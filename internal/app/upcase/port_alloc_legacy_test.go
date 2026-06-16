package upcase

import (
	"testing"

	"raioz/internal/domain/models"
)

// legacyDeps builds a Deps whose single infra dep uses the legacy `ports:`
// field (no publish/expose), the shape issue 020 is about.
func legacyDeps(name string, ports []string) *models.Deps {
	return newDepsWithInfra(nil, map[string]*models.Infra{
		name: {Image: "redis", Ports: ports},
	})
}

func TestAllocateHostPorts_LegacySingleValuePort(t *testing.T) {
	// `ports: ["6379"]` ⇒ Docker would publish on a random host port. The
	// allocator must instead pick a deterministic, free host port and map
	// it to container port 6379, so host callers get a reachable URL.
	deps := legacyDeps("redis", []string{"6379"})
	detections := DetectionMap{"redis": dockerDet()}

	allocs, err := AllocateHostPorts(deps, detections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	alloc, ok := allocs.Deps["redis"]
	if !ok {
		t.Fatal("legacy single-value port should be routed through the allocator")
	}
	if len(alloc.Mappings) != 1 {
		t.Fatalf("want 1 mapping, got %d", len(alloc.Mappings))
	}
	m := alloc.Mappings[0]
	if m.ContainerPort != 6379 {
		t.Errorf("container port = %d, want 6379", m.ContainerPort)
	}
	if m.HostPort <= 0 {
		t.Errorf("host port = %d, want a real allocated port", m.HostPort)
	}
}

func TestAllocateHostPorts_LegacyPinnedHostContainer(t *testing.T) {
	// `ports: ["8080:6379"]` pins host 8080 → container 6379.
	deps := legacyDeps("redis", []string{"8080:6379"})
	detections := DetectionMap{"redis": dockerDet()}

	allocs, err := AllocateHostPorts(deps, detections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := allocs.Deps["redis"].Mappings[0]
	if m.HostPort != 8080 || m.ContainerPort != 6379 {
		t.Errorf("mapping = %d:%d, want 8080:6379", m.HostPort, m.ContainerPort)
	}
}

func TestAllocateHostPorts_LegacyUnmodelableLeftVerbatim(t *testing.T) {
	// Shapes the allocator does not model (ranges, /proto, IP-prefixed)
	// must NOT be rewritten — they fall through to the verbatim legacy
	// path, so the dep gets no allocation entry.
	for _, spec := range []string{"6379-6381", "6379/tcp", "127.0.0.1:8080:6379", "notaport"} {
		deps := legacyDeps("redis", []string{spec})
		detections := DetectionMap{"redis": dockerDet()}

		allocs, err := AllocateHostPorts(deps, detections)
		if err != nil {
			t.Fatalf("spec %q: unexpected error: %v", spec, err)
		}
		if _, ok := allocs.Deps["redis"]; ok {
			t.Errorf("spec %q should be left to the verbatim legacy path", spec)
		}
	}
}

func TestAllocateHostPorts_LegacyPinnedConflict(t *testing.T) {
	// Two deps pinning the same host port is a hard error, matching what
	// Docker itself would reject.
	deps := newDepsWithInfra(nil, map[string]*models.Infra{
		"redis": {Image: "redis", Ports: []string{"8080:6379"}},
		"cache": {Image: "keydb", Ports: []string{"8080:6380"}},
	})
	detections := DetectionMap{"redis": dockerDet(), "cache": dockerDet()}

	if _, err := AllocateHostPorts(deps, detections); err == nil {
		t.Error("two legacy deps pinning host 8080 should conflict")
	}
}

func TestAllocateHostPorts_LegacyMixedListAllOrNothing(t *testing.T) {
	// If any entry in the list is unmodelable, the whole dep is left to the
	// verbatim path rather than partially rewritten.
	deps := legacyDeps("redis", []string{"6379", "6380-6381"})
	detections := DetectionMap{"redis": dockerDet()}

	allocs, err := AllocateHostPorts(deps, detections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := allocs.Deps["redis"]; ok {
		t.Error("a list with one unmodelable entry must not be partially allocated")
	}
}

func TestParseLegacyPortSpec(t *testing.T) {
	tests := []struct {
		spec            string
		host, container int
		pinned, ok      bool
	}{
		{"6379", 0, 6379, false, true},
		{"8080:6379", 8080, 6379, true, true},
		{" 8080 : 6379 ", 8080, 6379, true, true},
		{"6379-6381", 0, 0, false, false},
		{"6379/tcp", 0, 0, false, false},
		{"127.0.0.1:8080:6379", 0, 0, false, false},
		{"", 0, 0, false, false},
		{"0:6379", 0, 0, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			host, container, pinned, ok := parseLegacyPortSpec(tt.spec)
			if host != tt.host || container != tt.container ||
				pinned != tt.pinned || ok != tt.ok {
				t.Errorf("parseLegacyPortSpec(%q) = (%d,%d,%v,%v), want (%d,%d,%v,%v)",
					tt.spec, host, container, pinned, ok,
					tt.host, tt.container, tt.pinned, tt.ok)
			}
		})
	}
}
