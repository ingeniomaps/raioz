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
