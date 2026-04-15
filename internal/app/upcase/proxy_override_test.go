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

// TestBuildProxyRoute_DepOverrideBeatsDetection: same escape hatch, but for
// a dependency. Regression guard for v0.1.0 where
// `dependencies.<n>.proxy:` was silently dropped.
func TestBuildProxyRoute_DepOverrideBeatsDetection(t *testing.T) {
	deps := &config.Deps{
		Project: config.Project{Name: "hypixo"},
		Infra: map[string]config.InfraEntry{
			"redisinsight": {Inline: &config.Infra{
				Image: "redis/redisinsight",
				ProxyOverride: &config.ServiceProxyOverride{
					Target: "hypixo-redisinsight",
					Port:   5540,
				},
			}},
		},
	}
	det := &detect.DetectResult{Runtime: detect.RuntimeCompose, Port: 0}

	route := buildProxyRoute(deps, "redisinsight", det)
	if route.Target != "hypixo-redisinsight" {
		t.Errorf("Target = %q, want hypixo-redisinsight", route.Target)
	}
	if route.Port != 5540 {
		t.Errorf("Port = %d, want 5540", route.Port)
	}
}

// TestBuildProxyRoute_ExposeFallback: when detection can't resolve a port
// and the user declared `expose:` on the dep, the proxy should pick the
// first exposed port instead of leaving the route at port 0 (v0.1.1 fix).
func TestBuildProxyRoute_ExposeFallback(t *testing.T) {
	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
		Infra: map[string]config.InfraEntry{
			"pgadmin": {Inline: &config.Infra{
				Image:  "dpage/pgadmin4",
				Expose: []int{80, 443},
			}},
		},
	}
	det := &detect.DetectResult{Runtime: detect.RuntimeCompose, Port: 0}

	route := buildProxyRoute(deps, "pgadmin", det)
	if route.Port != 80 {
		t.Errorf("Port = %d, want 80 (from Expose[0])", route.Port)
	}
}

// TestBuildProxyRoute_PortsBeatsExpose: legacy `ports:` wins when both are
// set, preserving backwards-compat behavior.
func TestBuildProxyRoute_PortsBeatsExpose(t *testing.T) {
	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
		Infra: map[string]config.InfraEntry{
			"dep": {Inline: &config.Infra{
				Image:  "example/dep",
				Ports:  []string{"9000:9000"},
				Expose: []int{80},
			}},
		},
	}
	det := &detect.DetectResult{Runtime: detect.RuntimeCompose, Port: 0}

	route := buildProxyRoute(deps, "dep", det)
	if route.Port != 9000 {
		t.Errorf("Port = %d, want 9000 (Ports wins over Expose)", route.Port)
	}
}
