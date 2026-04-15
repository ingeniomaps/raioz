package upcase

import (
	"net"
	"os"
	"strings"
	"testing"

	"raioz/internal/config"
	"raioz/internal/detect"
)

// TestMain stubs the host-port busy probe for the entire upcase test suite.
// Without this, tests asserting concrete port numbers (e.g. "npm default is
// 3000") fail whenever the CI or developer host happens to already have that
// port bound — turning what should be a deterministic unit test into a
// host-specific flake. The production flow is unaffected: portInUseProbe is
// restored to docker.CheckPortInUse after the tests run.
func TestMain(m *testing.M) {
	prev := portInUseProbe
	portInUseProbe = func(string) (bool, error) { return false, nil }
	code := m.Run()
	portInUseProbe = prev
	os.Exit(code)
}

// newDeps is a tiny helper to build a config.Deps with a set of services for
// allocator tests. Each entry is (name, runtime-unused-here, explicitPort).
// The Runtime is set via the detections map in each test because the allocator
// reads it from there, not from the config.
func newDeps(services map[string]int) *config.Deps {
	deps := &config.Deps{
		Project:  config.Project{Name: "test"},
		Services: map[string]config.Service{},
	}
	for name, port := range services {
		deps.Services[name] = config.Service{
			Source: config.SourceConfig{Kind: "local", Path: "/tmp/" + name},
			Port:   port,
		}
	}
	return deps
}

func hostDet(rt detect.Runtime) detect.DetectResult {
	return detect.DetectResult{Runtime: rt}
}

func dockerDet() detect.DetectResult {
	return detect.DetectResult{Runtime: detect.RuntimeCompose}
}

func TestAllocateHostPorts_SingleHostServiceImplicit(t *testing.T) {
	deps := newDeps(map[string]int{"web": 0})
	detections := DetectionMap{"web": hostDet(detect.RuntimeNPM)}

	allocs, err := AllocateHostPorts(deps, detections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allocs.Services["web"].Port != 3000 {
		t.Errorf("web port = %d, want 3000 (npm default)", allocs.Services["web"].Port)
	}
	if allocs.Services["web"].Explicit {
		t.Errorf("web should be implicit")
	}
}

func TestAllocateHostPorts_ExplicitHonored(t *testing.T) {
	deps := newDeps(map[string]int{"web": 4000})
	detections := DetectionMap{"web": hostDet(detect.RuntimeNPM)}

	allocs, err := AllocateHostPorts(deps, detections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allocs.Services["web"].Port != 4000 {
		t.Errorf("web port = %d, want 4000", allocs.Services["web"].Port)
	}
	if !allocs.Services["web"].Explicit {
		t.Errorf("web should be explicit")
	}
}

func TestAllocateHostPorts_TwoImplicitDefaultsBumpDeterministic(t *testing.T) {
	// Two npm services, both default to 3000 → the second one (sorted by
	// name) must move to 3001.
	deps := newDeps(map[string]int{"web-a": 0, "web-b": 0})
	detections := DetectionMap{
		"web-a": hostDet(detect.RuntimeNPM),
		"web-b": hostDet(detect.RuntimeNPM),
	}

	allocs, err := AllocateHostPorts(deps, detections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allocs.Services["web-a"].Port != 3000 {
		t.Errorf("web-a port = %d, want 3000", allocs.Services["web-a"].Port)
	}
	if allocs.Services["web-b"].Port != 3001 {
		t.Errorf("web-b port = %d, want 3001", allocs.Services["web-b"].Port)
	}
}

func TestAllocateHostPorts_ExplicitExplicitConflictFails(t *testing.T) {
	// Two services both explicitly declaring port 3000 → hard error.
	deps := newDeps(map[string]int{"web-a": 3000, "web-b": 3000})
	detections := DetectionMap{
		"web-a": hostDet(detect.RuntimeNPM),
		"web-b": hostDet(detect.RuntimeNPM),
	}

	_, err := AllocateHostPorts(deps, detections)
	if err == nil {
		t.Fatal("expected explicit-conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "3000") {
		t.Errorf("error should mention port 3000, got: %v", err)
	}
}

func TestAllocateHostPorts_ExplicitBeatsImplicit(t *testing.T) {
	// web-a is explicit on 3000, web-b is implicit npm → web-b must move
	// to 3001 even though it's alphabetically first in the explicit pass.
	// (explicit pass runs before implicit pass regardless of name order)
	deps := newDeps(map[string]int{"web-a": 0, "web-b": 3000})
	detections := DetectionMap{
		"web-a": hostDet(detect.RuntimeNPM),
		"web-b": hostDet(detect.RuntimeNPM),
	}

	allocs, err := AllocateHostPorts(deps, detections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allocs.Services["web-b"].Port != 3000 {
		t.Errorf("web-b (explicit) = %d, want 3000", allocs.Services["web-b"].Port)
	}
	if allocs.Services["web-a"].Port != 3001 {
		t.Errorf("web-a (implicit) = %d, want 3001", allocs.Services["web-a"].Port)
	}
	if allocs.Services["web-a"].Wanted != 3000 {
		t.Errorf("web-a wanted = %d, want 3000 (what it tried before bumping)", allocs.Services["web-a"].Wanted)
	}
}

func TestAllocateHostPorts_DockerServicesSkipped(t *testing.T) {
	// Docker services live in their own network namespace: the allocator
	// must leave them alone. Two docker services "both on 3000" is not a
	// host-side conflict.
	deps := newDeps(map[string]int{"api-a": 0, "api-b": 0})
	detections := DetectionMap{
		"api-a": dockerDet(),
		"api-b": dockerDet(),
	}

	allocs, err := AllocateHostPorts(deps, detections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := allocs.Services["api-a"]; ok {
		t.Errorf("docker service api-a should not be allocated")
	}
	if _, ok := allocs.Services["api-b"]; ok {
		t.Errorf("docker service api-b should not be allocated")
	}
}

func TestAllocateHostPorts_MixedHostAndDocker(t *testing.T) {
	// web (host, npm) + api (docker) both default 3000/8080. Only web is
	// allocated. No conflict because docker lives in its own namespace.
	deps := newDeps(map[string]int{"web": 0, "api": 0})
	detections := DetectionMap{
		"web": hostDet(detect.RuntimeNPM),
		"api": dockerDet(),
	}

	allocs, err := AllocateHostPorts(deps, detections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allocs.Services["web"].Port != 3000 {
		t.Errorf("web = %d, want 3000", allocs.Services["web"].Port)
	}
	if _, ok := allocs.Services["api"]; ok {
		t.Errorf("api (docker) should not be in allocations")
	}
}

func TestAllocateHostPorts_ThreeServicesChainBump(t *testing.T) {
	// web-a, web-b, web-c all default to 3000 → 3000/3001/3002.
	deps := newDeps(map[string]int{"web-a": 0, "web-b": 0, "web-c": 0})
	detections := DetectionMap{
		"web-a": hostDet(detect.RuntimeNPM),
		"web-b": hostDet(detect.RuntimeNPM),
		"web-c": hostDet(detect.RuntimeNPM),
	}

	allocs, err := AllocateHostPorts(deps, detections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := map[string]int{
		"web-a": allocs.Services["web-a"].Port,
		"web-b": allocs.Services["web-b"].Port,
		"web-c": allocs.Services["web-c"].Port,
	}
	want := map[string]int{"web-a": 3000, "web-b": 3001, "web-c": 3002}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %d, want %d", k, got[k], v)
		}
	}
}

func TestAllocateHostPorts_DifferentRuntimeDefaultsNoConflict(t *testing.T) {
	// web (npm→3000) + api-go (go→8080): no collision, both get their defaults.
	deps := newDeps(map[string]int{"web": 0, "api-go": 0})
	detections := DetectionMap{
		"web":    hostDet(detect.RuntimeNPM),
		"api-go": hostDet(detect.RuntimeGo),
	}

	allocs, err := AllocateHostPorts(deps, detections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allocs.Services["web"].Port != 3000 {
		t.Errorf("web = %d, want 3000", allocs.Services["web"].Port)
	}
	if allocs.Services["api-go"].Port != 8080 {
		t.Errorf("api-go = %d, want 8080", allocs.Services["api-go"].Port)
	}
}

func TestAllocateHostPorts_CrossProjectBindClash(t *testing.T) {
	// Occupy a real port with a listener, then ask the allocator to place an
	// explicit service on it. AllocateHostPorts itself does NOT check for bind
	// conflicts (by design — the caller handles them interactively via
	// checkPortBindConflicts). Verify that checkPortBindConflicts detects it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot bind test port: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	deps := newDeps(map[string]int{"web": port})
	detections := DetectionMap{"web": hostDet(detect.RuntimeNPM)}

	result, err := AllocateHostPorts(deps, detections)
	if err != nil {
		t.Fatalf("AllocateHostPorts should succeed (bind check is separate): %v", err)
	}

	conflicts := checkPortBindConflicts(result)
	if len(conflicts) == 0 {
		t.Fatal("expected bind conflict for occupied port, got none")
	}
	if conflicts[0].Port != port {
		t.Errorf("expected conflict on port %d, got %d", port, conflicts[0].Port)
	}
}

// --- Dependency publish allocation -------------------------------------------

// newDepsWithInfra builds a config.Deps that has both services and infra,
// for tests that exercise the joint allocation path.
func newDepsWithInfra(
	services map[string]int,
	infra map[string]*config.Infra,
) *config.Deps {
	deps := newDeps(services)
	deps.Infra = map[string]config.InfraEntry{}
	for name, inf := range infra {
		deps.Infra[name] = config.InfraEntry{Inline: inf}
	}
	return deps
}

func TestAllocateHostPorts_DepInternalOnly(t *testing.T) {
	// No publish → dep is not in the result at all. Containers still reach
	// it via DNS; the allocator just has nothing to say.
	deps := newDepsWithInfra(
		nil,
		map[string]*config.Infra{
			"postgres": {Image: "postgres", Tag: "16", Expose: []int{5432}},
		},
	)
	detections := DetectionMap{"postgres": dockerDet()}

	allocs, err := AllocateHostPorts(deps, detections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := allocs.Deps["postgres"]; ok {
		t.Errorf("postgres with no publish should not appear in allocations")
	}
}

func TestAllocateHostPorts_DepExplicitPublish(t *testing.T) {
	deps := newDepsWithInfra(
		nil,
		map[string]*config.Infra{
			"postgres": {
				Image:   "postgres",
				Expose:  []int{5432},
				Publish: &config.PublishSpec{Ports: []int{5432}},
			},
		},
	)
	detections := DetectionMap{"postgres": dockerDet()}

	allocs, err := AllocateHostPorts(deps, detections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	alloc, ok := allocs.Deps["postgres"]
	if !ok {
		t.Fatal("postgres should be allocated")
	}
	if !alloc.Explicit {
		t.Errorf("should be explicit")
	}
	if len(alloc.Mappings) != 1 {
		t.Fatalf("want 1 mapping, got %d", len(alloc.Mappings))
	}
	if alloc.Mappings[0].HostPort != 5432 || alloc.Mappings[0].ContainerPort != 5432 {
		t.Errorf("mapping = %+v, want 5432:5432", alloc.Mappings[0])
	}
}

func TestAllocateHostPorts_DepAutoPublish(t *testing.T) {
	// publish: true + expose: [5432] → raioz picks a host port starting at
	// 5432. With nothing else taken, it should land on 5432 itself.
	deps := newDepsWithInfra(
		nil,
		map[string]*config.Infra{
			"postgres": {
				Image:   "postgres",
				Expose:  []int{5432},
				Publish: &config.PublishSpec{Auto: true},
			},
		},
	)
	detections := DetectionMap{"postgres": dockerDet()}

	allocs, err := AllocateHostPorts(deps, detections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	alloc := allocs.Deps["postgres"]
	if alloc.Explicit {
		t.Errorf("should be implicit (auto)")
	}
	if len(alloc.Mappings) != 1 || alloc.Mappings[0].HostPort != 5432 {
		t.Errorf("mapping = %+v, want host 5432 container 5432", alloc.Mappings)
	}
}

func TestAllocateHostPorts_DepAutoBumpsOnConflict(t *testing.T) {
	// A service already holds 5432 → postgres auto must bump to 5433.
	deps := newDepsWithInfra(
		map[string]int{"web": 5432}, // explicit service on 5432
		map[string]*config.Infra{
			"postgres": {
				Image:   "postgres",
				Expose:  []int{5432},
				Publish: &config.PublishSpec{Auto: true},
			},
		},
	)
	detections := DetectionMap{
		"web":      hostDet(detect.RuntimeGo),
		"postgres": dockerDet(),
	}

	allocs, err := AllocateHostPorts(deps, detections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allocs.Services["web"].Port != 5432 {
		t.Errorf("web lost its explicit port")
	}
	if allocs.Deps["postgres"].Mappings[0].HostPort != 5433 {
		t.Errorf("postgres host = %d, want 5433 (bumped)",
			allocs.Deps["postgres"].Mappings[0].HostPort)
	}
	// container port stays 5432 — that's the whole point of the split
	if allocs.Deps["postgres"].Mappings[0].ContainerPort != 5432 {
		t.Errorf("postgres container = %d, want 5432",
			allocs.Deps["postgres"].Mappings[0].ContainerPort)
	}
}

func TestAllocateHostPorts_DepExplicitConflictWithService(t *testing.T) {
	// A service explicitly on 5432, a dep ALSO explicitly on 5432 → error.
	deps := newDepsWithInfra(
		map[string]int{"web": 5432},
		map[string]*config.Infra{
			"postgres": {
				Image:   "postgres",
				Publish: &config.PublishSpec{Ports: []int{5432}},
			},
		},
	)
	detections := DetectionMap{
		"web":      hostDet(detect.RuntimeGo),
		"postgres": dockerDet(),
	}

	_, err := AllocateHostPorts(deps, detections)
	if err == nil {
		t.Fatal("expected explicit-conflict error")
	}
	if !strings.Contains(err.Error(), "5432") {
		t.Errorf("error should mention port 5432, got: %v", err)
	}
}

func TestAllocateHostPorts_TwoDepsExplicitConflict(t *testing.T) {
	deps := newDepsWithInfra(
		nil,
		map[string]*config.Infra{
			"pg-a": {
				Image:   "postgres",
				Publish: &config.PublishSpec{Ports: []int{5432}},
			},
			"pg-b": {
				Image:   "postgres",
				Publish: &config.PublishSpec{Ports: []int{5432}},
			},
		},
	)
	detections := DetectionMap{
		"pg-a": dockerDet(),
		"pg-b": dockerDet(),
	}

	_, err := AllocateHostPorts(deps, detections)
	if err == nil {
		t.Fatal("expected explicit-conflict error between two deps")
	}
}

func TestAllocateHostPorts_DepAutoMultipleExpose(t *testing.T) {
	// publish: true with multiple exposed ports → a mapping per expose entry.
	deps := newDepsWithInfra(
		nil,
		map[string]*config.Infra{
			"multi": {
				Image:   "fake",
				Expose:  []int{5432, 9090},
				Publish: &config.PublishSpec{Auto: true},
			},
		},
	)
	detections := DetectionMap{"multi": dockerDet()}

	allocs, err := AllocateHostPorts(deps, detections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ms := allocs.Deps["multi"].Mappings
	if len(ms) != 2 {
		t.Fatalf("want 2 mappings, got %d", len(ms))
	}
	if ms[0].HostPort != 5432 || ms[0].ContainerPort != 5432 {
		t.Errorf("mapping[0] = %+v, want 5432:5432", ms[0])
	}
	if ms[1].HostPort != 9090 || ms[1].ContainerPort != 9090 {
		t.Errorf("mapping[1] = %+v, want 9090:9090", ms[1])
	}
}

func TestAllocateHostPorts_DepAutoNoExposeSilentlySkipped(t *testing.T) {
	// publish: true without expose → allocator logs warning and skips. Not
	// an error (the dep still starts internal-only).
	deps := newDepsWithInfra(
		nil,
		map[string]*config.Infra{
			"mystery": {
				Image:   "fake",
				Publish: &config.PublishSpec{Auto: true},
			},
		},
	)
	detections := DetectionMap{"mystery": dockerDet()}

	allocs, err := AllocateHostPorts(deps, detections)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := allocs.Deps["mystery"]; ok {
		t.Errorf("mystery with no expose should not be allocated")
	}
}

func TestAllocateHostPorts_IsHostHelper(t *testing.T) {
	if !(PortAllocation{Runtime: detect.RuntimeNPM}).IsHost() {
		t.Error("npm should be host")
	}
	if (PortAllocation{Runtime: detect.RuntimeCompose}).IsHost() {
		t.Error("compose should not be host")
	}
	if (PortAllocation{Runtime: detect.RuntimeDockerfile}).IsHost() {
		t.Error("dockerfile should not be host")
	}
	if (PortAllocation{Runtime: detect.RuntimeImage}).IsHost() {
		t.Error("image should not be host")
	}
}
