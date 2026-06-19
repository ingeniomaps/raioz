package upcase

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/domain/models"
)

// TestBuildProxyRoute_HostnameAliases: aliases declared on a
// service must survive buildProxyRoute so downstream code (Caddyfile,
// network-alias registration) sees every hostname the user wants routed
// to the upstream.
func TestBuildProxyRoute_HostnameAliases(t *testing.T) {
	deps := &models.Deps{
		Project: models.Project{Name: "demo"},
		Services: map[string]models.Service{
			"keycloak": {
				Hostname:        "sso",
				HostnameAliases: []string{"accounts", "login"},
				ProxyOverride: &models.ServiceProxyOverride{
					Target: "demo-keycloak",
					Port:   8080,
				},
			},
		},
	}
	det := &models.DetectResult{Runtime: models.RuntimeMake, Port: 0}
	route := buildProxyRoute(context.Background(), nil, deps, "keycloak", det)
	if route.Hostname != "sso" {
		t.Errorf("Hostname = %q, want sso", route.Hostname)
	}
	if len(route.Aliases) != 2 || route.Aliases[0] != "accounts" || route.Aliases[1] != "login" {
		t.Errorf("Aliases = %v, want [accounts login]", route.Aliases)
	}
}

func TestBuildProxyRoute_OverrideBeatsDetection(t *testing.T) {
	// Service classified as host (no Docker detection) — without override,
	// target would be host.docker.internal with no port. The yaml-declared
	// override must be honored verbatim.
	deps := &models.Deps{
		Project: models.Project{Name: "hypixo"},
		Services: map[string]models.Service{
			"keycloak": {
				ProxyOverride: &models.ServiceProxyOverride{
					Target: "hypixo-keycloak",
					Port:   8080,
				},
			},
		},
	}
	det := &models.DetectResult{Runtime: models.RuntimeMake, Port: 0}

	route := buildProxyRoute(context.Background(), nil, deps, "keycloak", det)
	if route.Target != "hypixo-keycloak" {
		t.Errorf("Target = %q, want hypixo-keycloak", route.Target)
	}
	if route.Port != 8080 {
		t.Errorf("Port = %d, want 8080", route.Port)
	}
}

func TestBuildProxyRoute_NoOverrideFallsBackToDetection(t *testing.T) {
	deps := &models.Deps{
		Project: models.Project{Name: "proj"},
		Services: map[string]models.Service{
			"api": {},
		},
	}
	det := &models.DetectResult{Runtime: models.RuntimeCompose, Port: 3000}

	route := buildProxyRoute(context.Background(), nil, deps, "api", det)
	if route.Target == "" || route.Target == "host.docker.internal" {
		t.Errorf("expected Docker container target, got %q", route.Target)
	}
	if route.Port != 3000 {
		t.Errorf("Port = %d, want 3000", route.Port)
	}
}

func TestProxyTargetOverride_Unset(t *testing.T) {
	deps := &models.Deps{
		Services: map[string]models.Service{"api": {}},
	}
	if target, port := proxyTargetOverride(deps, "api"); target != "" || port != 0 {
		t.Errorf("expected empty override, got %q:%d", target, port)
	}
}

// TestBuildProxyRoute_DepOverrideBeatsDetection: same escape hatch, but for
// a dependency. Regression guard for v0.1.0 where
// `dependencies.<n>.proxy:` was silently dropped.
func TestBuildProxyRoute_DepOverrideBeatsDetection(t *testing.T) {
	deps := &models.Deps{
		Project: models.Project{Name: "hypixo"},
		Infra: map[string]models.InfraEntry{
			"redisinsight": {Inline: &models.Infra{
				Image: "redis/redisinsight",
				ProxyOverride: &models.ServiceProxyOverride{
					Target: "hypixo-redisinsight",
					Port:   5540,
				},
			}},
		},
	}
	det := &models.DetectResult{Runtime: models.RuntimeCompose, Port: 0}

	route := buildProxyRoute(context.Background(), nil, deps, "redisinsight", det)
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
	deps := &models.Deps{
		Project: models.Project{Name: "proj"},
		Infra: map[string]models.InfraEntry{
			"pgadmin": {Inline: &models.Infra{
				Image:  "dpage/pgadmin4",
				Expose: []int{80, 443},
			}},
		},
	}
	det := &models.DetectResult{Runtime: models.RuntimeCompose, Port: 0}

	route := buildProxyRoute(context.Background(), nil, deps, "pgadmin", det)
	if route.Port != 80 {
		t.Errorf("Port = %d, want 80 (from Expose[0])", route.Port)
	}
}

// TestBuildProxyRoute_DepHostnameOverride: `hostname:` on a dep replaces the
// default subdomain (= entry name) so the route lands at
// https://<hostname>.<domain> instead of https://<entry-name>.<domain>.
func TestBuildProxyRoute_DepHostnameOverride(t *testing.T) {
	deps := &models.Deps{
		Project: models.Project{Name: "demo"},
		Infra: map[string]models.InfraEntry{
			"mailpit": {Inline: &models.Infra{
				Image:    "axllent/mailpit",
				Hostname: "mail",
				Expose:   []int{8025},
			}},
		},
	}
	det := &models.DetectResult{Runtime: models.RuntimeCompose, Port: 8025}

	route := buildProxyRoute(context.Background(), nil, deps, "mailpit", det)
	if route.Hostname != "mail" {
		t.Errorf("Hostname = %q, want mail", route.Hostname)
	}
}

// TestBuildProxyRoute_DepProxyPortOnlyOverridesPort: when the user passes
// `proxy.port` without `proxy.target`, detection still picks the container
// name but the port must be the user-declared one. Without this, a multi-port
// image like mailpit (1025 SMTP / 8025 UI) gets routed to the wrong upstream
// port.
func TestBuildProxyRoute_DepProxyPortOnlyOverridesPort(t *testing.T) {
	deps := &models.Deps{
		Project: models.Project{Name: "demo"},
		Infra: map[string]models.InfraEntry{
			"mailpit": {Inline: &models.Infra{
				Image: "axllent/mailpit",
				ProxyOverride: &models.ServiceProxyOverride{
					Port: 8025,
				},
			}},
		},
	}
	det := &models.DetectResult{Runtime: models.RuntimeCompose, Port: 1025}

	route := buildProxyRoute(context.Background(), nil, deps, "mailpit", det)
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
	deps := &models.Deps{
		Project: models.Project{Name: "proj"},
		Infra: map[string]models.InfraEntry{
			"dep": {Inline: &models.Infra{
				Image:  "example/dep",
				Ports:  []string{"9000:9000"},
				Expose: []int{80},
			}},
		},
	}
	det := &models.DetectResult{Runtime: models.RuntimeCompose, Port: 0}

	route := buildProxyRoute(context.Background(), nil, deps, "dep", det)
	if route.Port != 9000 {
		t.Errorf("Port = %d, want 9000 (Ports wins over Expose)", route.Port)
	}
}

// TestEndToEnd_DepHostnameAndProxyPortFromYAML exercises the full chain
// disk → LoadDepsFromYAML → buildProxyRoute. Reproduces the
// gouduet/keycloak case verbatim: a mailpit dep with `hostname: mail`
// and `proxy.port: 8025` while detection picked the SMTP port 1025.
// Without both fixes the route emits hostname="mailpit" and port=1025
// — the bugs the user filed.
func TestEndToEnd_DepHostnameAndProxyPortFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raioz.yaml")
	yamlText := `project: demo
proxy:
  domain: demo.dev
dependencies:
  mailpit:
    image: axllent/mailpit:latest
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

	mh := deps.Infra["mailpit"].Inline
	if mh == nil {
		t.Fatal("mailpit Infra missing")
	}
	if mh.Hostname != "mail" {
		t.Errorf("mailpit.Hostname = %q, want mail", mh.Hostname)
	}
	if mh.ProxyOverride == nil || mh.ProxyOverride.Port != 8025 {
		t.Errorf("mailpit.ProxyOverride.Port = %+v, want 8025",
			mh.ProxyOverride)
	}

	// Detection picked the SMTP port (1025) — this is the failure mode
	// from the issue. The override must win.
	det := &models.DetectResult{Runtime: models.RuntimeCompose, Port: 1025}
	route := buildProxyRoute(context.Background(), nil, deps, "mailpit", det)
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
	det2 := &models.DetectResult{Runtime: models.RuntimeCompose, Port: 5540}
	route2 := buildProxyRoute(context.Background(), nil, deps, "redisinsight", det2)
	if route2.Hostname != "insight" {
		t.Errorf("route2.Hostname = %q, want insight", route2.Hostname)
	}
	if route2.Port != 5540 {
		t.Errorf("route2.Port = %d, want 5540", route2.Port)
	}
}
