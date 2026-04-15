package proxy

import (
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/naming"
)

func boolPtr(b bool) *bool { return &b }

func TestSetPublish_Default(t *testing.T) {
	m := NewManager("")
	if !m.IsPublished() {
		t.Error("default Manager must publish (legacy behavior)")
	}
}

func TestSetPublish_NilLeavesDefault(t *testing.T) {
	m := NewManager("")
	m.SetPublish(nil)
	if !m.IsPublished() {
		t.Error("SetPublish(nil) must keep the current value (default true)")
	}
}

func TestSetPublish_Explicit(t *testing.T) {
	m := NewManager("")
	m.SetPublish(boolPtr(false))
	if m.IsPublished() {
		t.Error("SetPublish(false) must turn publishing off")
	}
	m.SetPublish(boolPtr(true))
	if !m.IsPublished() {
		t.Error("SetPublish(true) must turn publishing back on")
	}
}

func TestHostsLine_BuildsFromRoutesAndIP(t *testing.T) {
	naming.SetPrefix("acme")
	defer naming.SetPrefix("")

	m := NewManager("")
	m.SetWorkspace("acme")
	m.SetProjectName("api")
	m.SetDomain("acme.dev")
	m.SetNetworkSubnet("172.28.0.0/16")
	m.AddRoute(t.Context(), interfaces.ProxyRoute{ServiceName: "api", Hostname: "api"})
	m.AddRoute(t.Context(), interfaces.ProxyRoute{ServiceName: "admin", Hostname: "admin"})

	got := m.HostsLine()
	want := "172.28.1.1  admin.acme.dev api.acme.dev"
	if got != want {
		t.Errorf("HostsLine = %q, want %q", got, want)
	}
}

func TestHostsLine_EmptyWhenNoIP(t *testing.T) {
	m := NewManager("")
	m.AddRoute(t.Context(), interfaces.ProxyRoute{ServiceName: "api", Hostname: "api"})
	if got := m.HostsLine(); got != "" {
		t.Errorf("expected empty when no IP resolvable, got %q", got)
	}
}

func TestHostsLine_EmptyWhenNoRoutes(t *testing.T) {
	naming.SetPrefix("acme")
	defer naming.SetPrefix("")

	m := NewManager("")
	m.SetNetworkSubnet("172.28.0.0/16")
	if got := m.HostsLine(); got != "" {
		t.Errorf("expected empty when no routes, got %q", got)
	}
}

func TestIsNonHTTPImage_BareName(t *testing.T) {
	cases := []struct {
		image string
		want  bool
	}{
		{"postgres:16", true},
		{"redis:7-alpine", true},
		{"bitnami/postgresql:15", true},

		// HTTP UIs that share a substring with the binary image
		{"redis/redisinsight:latest", false},
		{"dpage/pgadmin4:latest", false},
		{"clickhouse/clickhouse-server", false}, // bare = "clickhouse-server", not "clickhouse"

		{"nginx:alpine", false},
		{"my-registry.local:5000/my-app:1.0", false}, // colon-in-host shouldn't fool the tag stripper
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.image, func(t *testing.T) {
			if got := IsNonHTTPImage(tc.image); got != tc.want {
				t.Errorf("IsNonHTTPImage(%q) = %v, want %v", tc.image, got, tc.want)
			}
		})
	}
}
