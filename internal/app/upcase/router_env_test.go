package upcase

import (
	"testing"

	"raioz/internal/protocol"
)

// TestShouldSuppressBundledProxy_RouterOffOverridesInheritedEnv asserts
// issue 030 — the bundled Caddy gate must respect --router-off even
// when RAIOZ_ROUTER_ACTIVE=1 was inherited from the shell. Otherwise
// the flag is silently ignored and the operator loses their debug
// path.
func TestShouldSuppressBundledProxy_Matrix(t *testing.T) {
	cases := []struct {
		name      string
		envValue  string
		routerOff bool
		want      bool
	}{
		{"clean shell, no flag", "", false, false},
		{"clean shell, --router-off (no-op)", "", true, false},
		{"env active, no flag → suppress", "1", false, true},
		{"env active, --router-off → bypass", "1", true, false},
		{"env active alternate truthy, --router-off → bypass", "true", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(protocol.RouterActive, tc.envValue)
			if got := shouldSuppressBundledProxy(tc.routerOff); got != tc.want {
				t.Errorf("shouldSuppressBundledProxy(%v) with env=%q = %v, want %v",
					tc.routerOff, tc.envValue, got, tc.want)
			}
		})
	}
}

func TestRouterActiveFromEnv(t *testing.T) {
	cases := []struct {
		val  string
		want bool
	}{
		{"", false},
		{"0", false},
		{"false", false},
		{"no", false},
		{"banana", false},
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"yes", true},
		{"YES", true},
	}
	for _, tc := range cases {
		t.Run(tc.val, func(t *testing.T) {
			t.Setenv(protocol.RouterActive, tc.val)
			if got := routerActiveFromEnv(); got != tc.want {
				t.Errorf("routerActiveFromEnv()=%v for %q, want %v",
					got, tc.val, tc.want)
			}
		})
	}
}
