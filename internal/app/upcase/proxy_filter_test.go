package upcase

import (
	"testing"

	"raioz/internal/config"
)

func TestIsNonHTTPImage(t *testing.T) {
	cases := []struct {
		image string
		want  bool
	}{
		{"postgres:16", true},
		{"bitnami/postgresql:15-debian-12", true},
		{"redis:7-alpine", true},
		{"mariadb", true},
		{"mysql:8", true},
		{"mongo:latest", true},
		{"rabbitmq:management", true},

		// Bare-name match avoids substring false-positives:
		// redisinsight/pgadmin/mongo-express/etc. are HTTP UIs that share
		// a substring with the binary-protocol image they front.
		{"redis/redisinsight:latest", false},
		{"dpage/pgadmin4:latest", false},
		{"mongo-express:latest", false},
		{"clickhouse/clickhouse-server", false}, // bare = "clickhouse-server", not "clickhouse"

		{"nginx:alpine", false},
		{"caddy:latest", false},
		{"httpd", false},
		{"custom/my-api:1.0", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.image, func(t *testing.T) {
			if got := isNonHTTPImage(tc.image); got != tc.want {
				t.Errorf("isNonHTTPImage(%q) = %v, want %v", tc.image, got, tc.want)
			}
		})
	}
}

func TestShouldProxy_ServiceAlwaysProxied(t *testing.T) {
	deps := &config.Deps{
		Services: map[string]config.Service{"api": {}},
		Infra:    map[string]config.InfraEntry{},
	}
	if !shouldProxy(deps, "api") {
		t.Error("services must always get proxy routes")
	}
}

func TestShouldProxy_PostgresSkippedByDefault(t *testing.T) {
	deps := &config.Deps{
		Services: map[string]config.Service{},
		Infra: map[string]config.InfraEntry{
			"postgres": {Inline: &config.Infra{Image: "postgres:16"}},
		},
	}
	if shouldProxy(deps, "postgres") {
		t.Error("postgres must be skipped (binary protocol, no HTTP)")
	}
}

func TestShouldProxy_RedisSkippedByDefault(t *testing.T) {
	deps := &config.Deps{
		Services: map[string]config.Service{},
		Infra: map[string]config.InfraEntry{
			"cache": {Inline: &config.Infra{Image: "redis:7"}},
		},
	}
	if shouldProxy(deps, "cache") {
		t.Error("redis must be skipped by heuristic")
	}
}

func TestShouldProxy_HTTPImageProxied(t *testing.T) {
	deps := &config.Deps{
		Services: map[string]config.Service{},
		Infra: map[string]config.InfraEntry{
			"admin": {Inline: &config.Infra{Image: "nginx:alpine"}},
		},
	}
	if !shouldProxy(deps, "admin") {
		t.Error("non-DB images default to proxy-yes")
	}
}

func TestShouldProxy_RoutingOptIn(t *testing.T) {
	// User forces routing on a postgres container (e.g. a bespoke image that
	// actually speaks HTTP on top of pg). Must override the heuristic.
	deps := &config.Deps{
		Services: map[string]config.Service{},
		Infra: map[string]config.InfraEntry{
			"pgweb": {Inline: &config.Infra{
				Image:   "custom/postgres-admin-ui",
				Routing: &config.RoutingConfig{},
			}},
		},
	}
	if !shouldProxy(deps, "pgweb") {
		t.Error("explicit routing: must override the DB heuristic (BUG-14 opt-in)")
	}
}

func TestShouldProxy_UnknownName(t *testing.T) {
	// A name not in Services or Infra (legacy path) defaults to true so we
	// don't silently drop legitimate routes.
	deps := &config.Deps{
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
	}
	if !shouldProxy(deps, "mystery") {
		t.Error("unknown names default to proxy-yes")
	}
}
