package cli

import "testing"

// TestNewDependencies_WiresEveryPort guards against silent gaps in
// the wiring layer. ADR-018 says production code always passes through
// newDependencies(); a forgotten port here means downstream use cases
// get a nil port and panic at runtime.
func TestNewDependencies_WiresEveryPort(t *testing.T) {
	deps := newDependencies()
	if deps == nil {
		t.Fatal("expected non-nil Dependencies")
	}
	cases := []struct {
		name string
		got  any
	}{
		{"ConfigLoader", deps.ConfigLoader},
		{"Validator", deps.Validator},
		{"DockerRunner", deps.DockerRunner},
		{"GitRepository", deps.GitRepository},
		{"Workspace", deps.Workspace},
		{"StateManager", deps.StateManager},
		{"LockManager", deps.LockManager},
		{"HostRunner", deps.HostRunner},
		{"EnvManager", deps.EnvManager},
		{"ProxyManager", deps.ProxyManager},
		{"DiscoveryManager", deps.DiscoveryManager},
		{"SnapshotManager", deps.SnapshotManager},
		{"TunnelManager", deps.TunnelManager},
	}
	for _, c := range cases {
		if c.got == nil {
			t.Errorf("expected %s to be set", c.name)
		}
	}
}
