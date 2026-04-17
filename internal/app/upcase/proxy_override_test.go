package upcase

import (
	"os"
	"path/filepath"
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

// TestBuildProxyRoute_DepHostnameOverride: `hostname:` on a dep replaces the
// default subdomain (= entry name) so the route lands at
// https://<hostname>.<domain> instead of https://<entry-name>.<domain>
// (issue #001).
func TestBuildProxyRoute_DepHostnameOverride(t *testing.T) {
	deps := &config.Deps{
		Project: config.Project{Name: "demo"},
		Infra: map[string]config.InfraEntry{
			"mailhog": {Inline: &config.Infra{
				Image:    "mailhog/mailhog",
				Hostname: "mail",
				Expose:   []int{8025},
			}},
		},
	}
	det := &detect.DetectResult{Runtime: detect.RuntimeCompose, Port: 8025}

	route := buildProxyRoute(deps, "mailhog", det)
	if route.Hostname != "mail" {
		t.Errorf("Hostname = %q, want mail", route.Hostname)
	}
}

// TestBuildProxyRoute_DepProxyPortOnlyOverridesPort: when the user passes
// `proxy.port` without `proxy.target`, detection still picks the container
// name but the port must be the user-declared one. Without this, a multi-port
// image like mailhog (1025 SMTP / 8025 UI) gets routed to the wrong upstream
// port (issue #003).
func TestBuildProxyRoute_DepProxyPortOnlyOverridesPort(t *testing.T) {
	deps := &config.Deps{
		Project: config.Project{Name: "demo"},
		Infra: map[string]config.InfraEntry{
			"mailhog": {Inline: &config.Infra{
				Image: "mailhog/mailhog",
				ProxyOverride: &config.ServiceProxyOverride{
					Port: 8025,
				},
			}},
		},
	}
	det := &detect.DetectResult{Runtime: detect.RuntimeCompose, Port: 1025}

	route := buildProxyRoute(deps, "mailhog", det)
	if route.Port != 8025 {
		t.Errorf("Port = %d, want 8025 (user override)", route.Port)
	}
	if route.Target == "" || route.Target == "host.docker.internal" {
		t.Errorf("expected Docker container target, got %q", route.Target)
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

// TestEndToEnd_DepHostnameAndProxyPortFromYAML exercises the full chain
// disk → LoadDepsFromYAML → buildProxyRoute that issues #001 and #003
// describe. Reproduces the gouduet/keycloak case verbatim: a mailhog dep
// with `hostname: mail` (issue #001) and `proxy.port: 8025` (issue #003)
// while detection picked the SMTP port 1025. Without both fixes the route
// emits hostname="mailhog" and port=1025 — the bugs the user filed.
func TestEndToEnd_DepHostnameAndProxyPortFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raioz.yaml")
	yamlText := `project: demo
proxy:
  domain: demo.dev
dependencies:
  mailhog:
    image: mailhog/mailhog:latest
    hostname: mail
    ports: ["8026:8025"]
    proxy:
      port: 8025
  redisinsight:
    image: redis/redisinsight:latest
    hostname: insight
    ports: ["5541:5540"]
`
	if err := os.WriteFile(path, []byte(yamlText), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	deps, _, err := config.LoadDepsFromYAML(path)
	if err != nil {
		t.Fatalf("LoadDepsFromYAML: %v", err)
	}

	mh := deps.Infra["mailhog"].Inline
	if mh == nil {
		t.Fatal("mailhog Infra missing")
	}
	if mh.Hostname != "mail" {
		t.Errorf("mailhog.Hostname = %q, want mail (issue #001)", mh.Hostname)
	}
	if mh.ProxyOverride == nil || mh.ProxyOverride.Port != 8025 {
		t.Errorf("mailhog.ProxyOverride.Port = %+v, want 8025 (issue #003)",
			mh.ProxyOverride)
	}

	// Detection picked the SMTP port (1025) — this is the failure mode
	// from the issue. The override must win.
	det := &detect.DetectResult{Runtime: detect.RuntimeCompose, Port: 1025}
	route := buildProxyRoute(deps, "mailhog", det)
	if route.Hostname != "mail" {
		t.Errorf("route.Hostname = %q, want mail", route.Hostname)
	}
	if route.Port != 8025 {
		t.Errorf("route.Port = %d, want 8025", route.Port)
	}
	if route.Target == "" || route.Target == "host.docker.internal" {
		t.Errorf("route.Target = %q, want a Docker container name", route.Target)
	}

	// redisinsight: hostname only, port from detection.
	ri := deps.Infra["redisinsight"].Inline
	if ri.Hostname != "insight" {
		t.Errorf("redisinsight.Hostname = %q, want insight", ri.Hostname)
	}
	det2 := &detect.DetectResult{Runtime: detect.RuntimeCompose, Port: 5540}
	route2 := buildProxyRoute(deps, "redisinsight", det2)
	if route2.Hostname != "insight" {
		t.Errorf("route2.Hostname = %q, want insight", route2.Hostname)
	}
	if route2.Port != 5540 {
		t.Errorf("route2.Port = %d, want 5540", route2.Port)
	}
}
