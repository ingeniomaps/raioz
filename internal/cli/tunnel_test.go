package cli

import (
	"testing"
)

func TestTunnelCmd(t *testing.T) {
	if tunnelCmd == nil {
		t.Fatal("tunnelCmd should be initialized")
	}
	if tunnelCmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if !tunnelCmd.SilenceUsage {
		t.Error("SilenceUsage should be true")
	}
}

func TestTunnelCmdFlags(t *testing.T) {
	f := tunnelCmd.Flags().Lookup("port")
	if f == nil {
		t.Fatal("flag 'port' not registered")
	}
	if f.DefValue != "0" {
		t.Errorf("port default = %s, want 0", f.DefValue)
	}
}

func TestTunnelCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "tunnel" {
			found = true
			break
		}
	}
	if !found {
		t.Error("tunnelCmd not registered on rootCmd")
	}
}

func TestTunnelSubcommandsRegistered(t *testing.T) {
	expected := []string{"list", "stop", "stop-all"}
	registered := make(map[string]bool)
	for _, sub := range tunnelCmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("tunnel subcommand %q not registered", name)
		}
	}
}

func TestTunnelStopUnknown(t *testing.T) {
	// Stopping a tunnel that doesn't exist should return an error.
	err := tunnelStopCmd.RunE(tunnelStopCmd, []string{"nonexistent-svc-for-test-xyz-abc-123"})
	if err == nil {
		t.Error("expected error when stopping unknown tunnel, got nil")
	}
}
