package app

import (
	"testing"

	"raioz/internal/proxy"
)

// TestDefaultProxyIPLocal_MatchesProxyPackage pins app.defaultProxyIPLocal
// to agree with proxy.DefaultProxyIP for every input the convention is
// declared on. The duplicate exists to avoid an app→proxy production
// import (the comment on defaultProxyIPLocal explains why); this test
// is the drift guard the inline comment references. If the convention
// changes, both copies move together or this test catches it.
//
// Test files are exempt from the ADR-029 import ratchet
// (scripts/lint-app-infra-imports.sh), so importing internal/proxy here
// does NOT grow the baseline.
func TestDefaultProxyIPLocal_MatchesProxyPackage(t *testing.T) {
	cases := []string{
		"",
		"172.28.0.0/16",
		"10.3.0.0/16",
		"192.168.0.0/16",
		"172.28.0.0/24",
		"not a subnet",
		"172.28.0.0/8",
		"::1/128",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			got := defaultProxyIPLocal(in)
			want := proxy.DefaultProxyIP(in)
			if got != want {
				t.Fatalf("defaultProxyIPLocal(%q) = %q, proxy.DefaultProxyIP = %q — duplicate drifted",
					in, got, want)
			}
		})
	}
}
