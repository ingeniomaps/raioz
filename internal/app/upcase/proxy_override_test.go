package upcase

import (
	"testing"

	"raioz/internal/config"
	"raioz/internal/detect"
)

func TestBuildProxyRoute_OverrideBeatsDetection(t *testing.T) {
	// Service classified as host (no Docker detection) — without override,
	// target would be host.docker.internal with no port. The yaml-declared
	// override must be honored verbatim.
	deps := &config.Deps{
		Project: config.Project{Name: "hypixo"},
		Services: map[string]config.Service{
			"keycloak": {
				ProxyOverride: &config.ServiceProxyOverride{
					Target: "hypixo-keycloak",
					Port:   8080,
				},
			},
		},
	}
	det := &detect.DetectResult{Runtime: detect.RuntimeMake, Port: 0}

	route := buildProxyRoute(deps, "keycloak", det)
	if route.Target != "hypixo-keycloak" {
		t.Errorf("Target = %q, want hypixo-keycloak", route.Target)
	}
	if route.Port != 8080 {
		t.Errorf("Port = %d, want 8080", route.Port)
	}
}

func TestBuildProxyRoute_NoOverrideFallsBackToDetection(t *testing.T) {
	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
		Services: map[string]config.Service{
			"api": {},
		},
	}
	det := &detect.DetectResult{Runtime: detect.RuntimeCompose, Port: 3000}

	route := buildProxyRoute(deps, "api", det)
	if route.Target == "" || route.Target == "host.docker.internal" {
		t.Errorf("expected Docker container target, got %q", route.Target)
	}
	if route.Port != 3000 {
		t.Errorf("Port = %d, want 3000", route.Port)
	}
}

func TestProxyTargetOverride_Unset(t *testing.T) {
	deps := &config.Deps{
		Services: map[string]config.Service{"api": {}},
	}
	if target, port := proxyTargetOverride(deps, "api"); target != "" || port != 0 {
		t.Errorf("expected empty override, got %q:%d", target, port)
	}
}
