package runtime

import "testing"

func TestSupports_DockerDefault(t *testing.T) {
	SetBinary("docker")
	t.Cleanup(func() { SetBinary("docker") })
	for _, c := range []Capability{HostGatewayAlias, ComposeProfiles, LabelFilterOnDown} {
		if !Supports(c) {
			t.Errorf("docker should support capability %d (optimistic default)", c)
		}
	}
}

func TestSupports_NerdctlHostGatewayFalse(t *testing.T) {
	prev := Binary()
	SetBinary("nerdctl")
	t.Cleanup(func() { SetBinary(prev) })
	if Supports(HostGatewayAlias) {
		t.Error("nerdctl should NOT support host-gateway alias without v2 detection (conservative default)")
	}
	// Other capabilities still default to true.
	if !Supports(ComposeProfiles) {
		t.Error("nerdctl should support compose profiles (optimistic default)")
	}
}

func TestSupports_PodmanOptimistic(t *testing.T) {
	prev := Binary()
	SetBinary("podman")
	t.Cleanup(func() { SetBinary(prev) })
	for _, c := range []Capability{HostGatewayAlias, ComposeProfiles, LabelFilterOnDown} {
		if !Supports(c) {
			t.Errorf("podman should support %d under v1 optimistic table", c)
		}
	}
}

// Operator override: nerdctl users on v2.x can opt back into
// HostGatewayAlias via the env var before version detection lands.
func TestSupports_OverrideForcesTrueForNerdctl(t *testing.T) {
	prev := Binary()
	SetBinary("nerdctl")
	t.Setenv("RAIOZ_RUNTIME_CAPABILITY", "HostGatewayAlias=true")
	ResetCapabilityOverridesForTest()
	t.Cleanup(func() {
		SetBinary(prev)
		ResetCapabilityOverridesForTest()
	})

	if !Supports(HostGatewayAlias) {
		t.Error("override must flip HostGatewayAlias to true for nerdctl")
	}
	if !Supports(ComposeProfiles) {
		t.Error("non-overridden capability should still default true")
	}
}

func TestSupports_OverrideForcesFalse(t *testing.T) {
	prev := Binary()
	SetBinary("docker")
	t.Setenv("RAIOZ_RUNTIME_CAPABILITY", "ComposeProfiles=false")
	ResetCapabilityOverridesForTest()
	t.Cleanup(func() {
		SetBinary(prev)
		ResetCapabilityOverridesForTest()
	})

	if Supports(ComposeProfiles) {
		t.Error("override must flip ComposeProfiles to false on docker")
	}
	if !Supports(HostGatewayAlias) {
		t.Error("unrelated capability should still default true")
	}
}

func TestParseCapabilityOverrides_FormatMatrix(t *testing.T) {
	cases := []struct {
		raw  string
		want map[string]bool
	}{
		{"", map[string]bool{}},
		{"HostGatewayAlias=true", map[string]bool{"HostGatewayAlias": true}},
		{"HostGatewayAlias=true,ComposeProfiles=false", map[string]bool{
			"HostGatewayAlias": true, "ComposeProfiles": false,
		}},
		{"HostGatewayAlias=YES, ComposeProfiles = no", map[string]bool{
			"HostGatewayAlias": true, "ComposeProfiles": false,
		}},
		{"NoEqualsSign", map[string]bool{}},
		{"HostGatewayAlias=maybe", map[string]bool{}}, // unknown truthy → ignored
	}
	for _, tc := range cases {
		got := parseCapabilityOverrides(tc.raw)
		if len(got) != len(tc.want) {
			t.Errorf("parse(%q) length=%d, want %d (got=%v)",
				tc.raw, len(got), len(tc.want), got)
			continue
		}
		for k, v := range tc.want {
			if got[k] != v {
				t.Errorf("parse(%q)[%q]=%v, want %v", tc.raw, k, got[k], v)
			}
		}
	}
}
