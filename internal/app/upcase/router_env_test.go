package upcase

import (
	"testing"

	"raioz/internal/protocol"
)

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
