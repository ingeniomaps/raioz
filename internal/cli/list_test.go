package cli

import (
	"testing"
)

func TestListCmd(t *testing.T) {
	if listCmd == nil {
		t.Fatal("listCmd should be initialized")
	}
	if listCmd.Use != "list" {
		t.Errorf("Use = %s, want list", listCmd.Use)
	}
	if listCmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestListCmdFlags(t *testing.T) {
	flags := []struct {
		name     string
		defValue string
	}{
		{"json", "false"},
		{"filter", ""},
		{"status", ""},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			f := listCmd.Flags().Lookup(tt.name)
			if f == nil {
				t.Fatalf("flag %q not registered", tt.name)
			}
			if f.DefValue != tt.defValue {
				t.Errorf("flag %q default = %s, want %s",
					tt.name, f.DefValue, tt.defValue)
			}
		})
	}
}

func TestListCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "list" {
			found = true
			break
		}
	}
	if !found {
		t.Error("listCmd not registered on rootCmd")
	}
}
