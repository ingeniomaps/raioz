package upcase

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/config"
	"raioz/internal/detect"
	"raioz/internal/docker"
)

// --- resolveDepPublishPorts --------------------------------------------------

func TestResolveDepPublishPorts_AllocatorResultTakesPrecedence(t *testing.T) {
	entry := config.InfraEntry{
		Inline: &config.Infra{
			Image: "postgres",
			Ports: []string{"9999:5432"}, // legacy
		},
	}
	allocs := &PortAllocResult{
		Deps: map[string]DepPortAllocation{
			"pg": {
				Name: "pg",
				Mappings: []DepPortMapping{
					{HostPort: 5433, ContainerPort: 5432},
				},
			},
		},
	}
	got := resolveDepPublishPorts("pg", entry, allocs)
	if len(got) != 1 || got[0] != "5433:5432" {
		t.Errorf("got %v, want [5433:5432]", got)
	}
}

func TestResolveDepPublishPorts_FallsBackToLegacyPorts(t *testing.T) {
	entry := config.InfraEntry{
		Inline: &config.Infra{
			Image: "postgres",
			Ports: []string{"5432:5432"},
		},
	}
	allocs := &PortAllocResult{
		Deps: map[string]DepPortAllocation{}, // empty
	}
	got := resolveDepPublishPorts("pg", entry, allocs)
	if len(got) != 1 || got[0] != "5432:5432" {
		t.Errorf("got %v, want [5432:5432]", got)
	}
}

func TestResolveDepPublishPorts_NilAllocsUsesLegacy(t *testing.T) {
	entry := config.InfraEntry{
		Inline: &config.Infra{
			Image: "redis",
			Ports: []string{"6379:6379"},
		},
	}
	got := resolveDepPublishPorts("redis", entry, nil)
	if len(got) != 1 || got[0] != "6379:6379" {
		t.Errorf("got %v, want [6379:6379]", got)
	}
}

func TestResolveDepPublishPorts_NoAllocNoLegacyReturnsNil(t *testing.T) {
	entry := config.InfraEntry{
		Inline: &config.Infra{Image: "redis"},
	}
	allocs := &PortAllocResult{Deps: map[string]DepPortAllocation{}}
	got := resolveDepPublishPorts("redis", entry, allocs)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestResolveDepPublishPorts_ExternalEntryReturnsNil(t *testing.T) {
	entry := config.InfraEntry{Inline: nil}
	allocs := &PortAllocResult{Deps: map[string]DepPortAllocation{}}
	got := resolveDepPublishPorts("ext", entry, allocs)
	if got != nil {
		t.Errorf("expected nil for external entry, got %v", got)
	}
}

func TestResolveDepPublishPorts_MultipleMappings(t *testing.T) {
	allocs := &PortAllocResult{
		Deps: map[string]DepPortAllocation{
			"multi": {
				Name: "multi",
				Mappings: []DepPortMapping{
					{HostPort: 5432, ContainerPort: 5432},
					{HostPort: 9091, ContainerPort: 9090},
				},
			},
		},
	}
	entry := config.InfraEntry{Inline: &config.Infra{Image: "multi"}}
	got := resolveDepPublishPorts("multi", entry, allocs)
	if len(got) != 2 {
		t.Fatalf("want 2 entries, got %d", len(got))
	}
	if got[0] != "5432:5432" || got[1] != "9091:9090" {
		t.Errorf("got %v", got)
	}
}

// --- serviceBindError / depBindError -----------------------------------------

func TestServiceBindError_Explicit(t *testing.T) {
	alloc := PortAllocation{Name: "web", Port: 3000, Explicit: true}
	err := serviceBindError(alloc)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "web") || !strings.Contains(msg, "3000") {
		t.Errorf("error should mention service and port, got: %s", msg)
	}
}

func TestServiceBindError_Implicit(t *testing.T) {
	alloc := PortAllocation{Name: "api", Port: 8080, Explicit: false}
	err := serviceBindError(alloc)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "api") || !strings.Contains(msg, "8080") {
		t.Errorf("error should mention service and port, got: %s", msg)
	}
}

func TestDepBindError_Explicit(t *testing.T) {
	alloc := DepPortAllocation{Name: "pg", Explicit: true}
	m := DepPortMapping{HostPort: 5432, ContainerPort: 5432}
	err := depBindError(alloc, m)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "pg") || !strings.Contains(msg, "5432") {
		t.Errorf("error should mention dep and port, got: %s", msg)
	}
}

func TestDepBindError_Auto(t *testing.T) {
	alloc := DepPortAllocation{Name: "redis", Explicit: false}
	m := DepPortMapping{HostPort: 6379, ContainerPort: 6379}
	err := depBindError(alloc, m)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "redis") || !strings.Contains(msg, "6379") {
		t.Errorf("error should mention dep and port, got: %s", msg)
	}
}

// --- applyPortChange ---------------------------------------------------------

func TestApplyPortChange_Service(t *testing.T) {
	result := &PortAllocResult{
		Services: map[string]PortAllocation{
			"web": {Name: "web", Port: 3000},
		},
		Deps: map[string]DepPortAllocation{},
	}
	c := PortBindConflict{Kind: "service", Name: "web", Port: 3000}
	applyPortChange(c, 3001, result, "")
	if result.Services["web"].Port != 3001 {
		t.Errorf("port not updated: got %d, want 3001", result.Services["web"].Port)
	}
}

func TestApplyPortChange_Dep(t *testing.T) {
	result := &PortAllocResult{
		Services: map[string]PortAllocation{},
		Deps: map[string]DepPortAllocation{
			"pg": {
				Name: "pg",
				Mappings: []DepPortMapping{
					{HostPort: 5432, ContainerPort: 5432},
				},
			},
		},
	}
	c := PortBindConflict{Kind: "dep", Name: "pg", Port: 5432}
	applyPortChange(c, 5433, result, "")
	if result.Deps["pg"].Mappings[0].HostPort != 5433 {
		t.Errorf("host port not updated: got %d, want 5433",
			result.Deps["pg"].Mappings[0].HostPort)
	}
	// container port stays the same
	if result.Deps["pg"].Mappings[0].ContainerPort != 5432 {
		t.Errorf("container port should not change")
	}
}

func TestApplyPortChange_UnknownServiceNoop(t *testing.T) {
	result := &PortAllocResult{
		Services: map[string]PortAllocation{},
		Deps:     map[string]DepPortAllocation{},
	}
	c := PortBindConflict{Kind: "service", Name: "ghost", Port: 9999}
	// Should not panic
	applyPortChange(c, 9998, result, "")
}

// --- buildProxyRoute ---------------------------------------------------------

func TestBuildProxyRoute_DockerService(t *testing.T) {
	deps := &config.Deps{
		Project:  config.Project{Name: "myproj"},
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
	}
	det := detect.DetectResult{
		Runtime: detect.RuntimeCompose,
		Port:    8080,
	}
	route := buildProxyRoute(deps, "api", &det)
	if route.ServiceName != "api" {
		t.Errorf("ServiceName = %s, want api", route.ServiceName)
	}
	if route.Hostname != "api" {
		t.Errorf("Hostname = %s, want api", route.Hostname)
	}
	if !strings.Contains(route.Target, "myproj") {
		t.Errorf("Target should contain project name, got: %s", route.Target)
	}
	if route.Port != 8080 {
		t.Errorf("Port = %d, want 8080", route.Port)
	}
}

func TestBuildProxyRoute_HostService(t *testing.T) {
	deps := &config.Deps{
		Project:  config.Project{Name: "proj"},
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
	}
	det := detect.DetectResult{
		Runtime: detect.RuntimeNPM,
		Port:    3000,
	}
	route := buildProxyRoute(deps, "web", &det)
	if route.Target != "host.docker.internal" {
		t.Errorf("Target = %s, want host.docker.internal", route.Target)
	}
	if route.Port != 3000 {
		t.Errorf("Port = %d, want 3000", route.Port)
	}
}

func TestBuildProxyRoute_CustomHostname(t *testing.T) {
	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
		Services: map[string]config.Service{
			"api": {Hostname: "backend"},
		},
		Infra: map[string]config.InfraEntry{},
	}
	det := detect.DetectResult{Runtime: detect.RuntimeGo, Port: 8080}
	route := buildProxyRoute(deps, "api", &det)
	if route.Hostname != "backend" {
		t.Errorf("Hostname = %s, want backend", route.Hostname)
	}
}

func TestBuildProxyRoute_RoutingFlags(t *testing.T) {
	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
		Services: map[string]config.Service{
			"ws-svc": {
				Routing: &config.RoutingConfig{
					WS:     true,
					Stream: true,
					GRPC:   false,
				},
			},
		},
		Infra: map[string]config.InfraEntry{},
	}
	det := detect.DetectResult{Runtime: detect.RuntimeNPM, Port: 3000}
	route := buildProxyRoute(deps, "ws-svc", &det)
	if !route.WebSocket {
		t.Error("WebSocket should be true")
	}
	if !route.Stream {
		t.Error("Stream should be true")
	}
	if route.GRPC {
		t.Error("GRPC should be false")
	}
}

func TestBuildProxyRoute_FallbackPortFromInfra(t *testing.T) {
	deps := &config.Deps{
		Project:  config.Project{Name: "proj"},
		Services: map[string]config.Service{},
		Infra: map[string]config.InfraEntry{
			"redis": {
				Inline: &config.Infra{
					Image: "redis",
					Ports: []string{"6379:6379"},
				},
			},
		},
	}
	det := detect.DetectResult{Runtime: detect.RuntimeImage, Port: 0}
	route := buildProxyRoute(deps, "redis", &det)
	if route.Port != 6379 {
		t.Errorf("Port = %d, want 6379 (from infra ports)", route.Port)
	}
}

// --- printConflictBanner -----------------------------------------------------

func TestPrintConflictBanner_DockerRaioz(t *testing.T) {
	c := PortBindConflict{Kind: "service", Name: "web", Port: 3000}
	occ := docker.PortOccupant{
		IsDocker:    true,
		IsRaioz:     true,
		ProjectName: "other-proj",
	}
	// Should not panic
	printConflictBanner(c, occ)
}

func TestPrintConflictBanner_DockerNonRaioz(t *testing.T) {
	c := PortBindConflict{Kind: "dep", Name: "pg", Port: 5432}
	occ := docker.PortOccupant{
		IsDocker:      true,
		IsRaioz:       false,
		ContainerName: "some-postgres-1",
	}
	printConflictBanner(c, occ)
}

func TestPrintConflictBanner_External(t *testing.T) {
	c := PortBindConflict{Kind: "service", Name: "api", Port: 8080}
	occ := docker.PortOccupant{IsDocker: false}
	printConflictBanner(c, occ)
}

// --- checkHostServiceHealth --------------------------------------------------

func TestCheckHostServiceHealth_AddressInUse(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "svc.log")
	os.WriteFile(logPath, []byte("listen tcp :3000: bind: address already in use\n"), 0644)

	// Should not panic, prints warning
	checkHostServiceHealth(t.Context(), "web", logPath)
}

func TestCheckHostServiceHealth_ErrorInLogs(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "svc.log")
	os.WriteFile(logPath, []byte("Error: cannot connect to database\n"), 0644)

	checkHostServiceHealth(t.Context(), "api", logPath)
}

func TestCheckHostServiceHealth_CleanLogs(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "svc.log")
	os.WriteFile(logPath, []byte("Server started successfully\nListening on :8080\n"), 0644)

	checkHostServiceHealth(t.Context(), "api", logPath)
}

func TestCheckHostServiceHealth_MissingFile(t *testing.T) {
	// Non-existent log file should not panic
	checkHostServiceHealth(t.Context(), "ghost", "/tmp/nonexistent-log-12345.log")
}
