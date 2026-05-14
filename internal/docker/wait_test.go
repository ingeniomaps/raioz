package docker

import "testing"

// TestIsHostGatewayTarget documents the predicate used by HostRunner
// to decide when to skip the post-launcher container wait. Dotted
// names are treated as host-shaped on the conservative-default
// principle described in the docstring.
func TestIsHostGatewayTarget(t *testing.T) {
	cases := []struct {
		name   string
		target string
		want   bool
	}{
		{"empty", "", true},
		{"host docker internal", "host.docker.internal", true},
		{"localhost", "localhost", true},
		{"ipv4 loopback", "127.0.0.1", true},
		{"ipv6 loopback", "::1", true},
		{"dotted FQDN", "service.internal.example.com", true},
		{"dotted IP", "10.0.0.5", true},
		{"bare container name", "hypixo-postgres", false},
		{"underscore container name", "my_app_db", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsHostGatewayTarget(tc.target); got != tc.want {
				t.Errorf("IsHostGatewayTarget(%q) = %v, want %v",
					tc.target, got, tc.want)
			}
		})
	}
}
