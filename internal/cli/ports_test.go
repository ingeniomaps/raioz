package cli

import (
	"testing"
)

func TestPortsCmd(t *testing.T) {
	if portsCmd == nil {
		t.Fatal("portsCmd should be initialized")
	}
	if portsCmd.Use != "ports" {
		t.Errorf("Use = %s, want ports", portsCmd.Use)
	}
	if portsCmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestPortsCmdFlags(t *testing.T) {
	f := portsCmd.Flags().Lookup("project")
	if f == nil {
		t.Fatal("flag 'project' not registered")
	}
	if f.Shorthand != "p" {
		t.Errorf("shorthand = %s, want p", f.Shorthand)
	}
}

func TestPortsCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "ports" {
			found = true
			break
		}
	}
	if !found {
		t.Error("portsCmd not registered on rootCmd")
	}
}
