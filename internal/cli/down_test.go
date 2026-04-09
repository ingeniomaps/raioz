package cli

import (
	"testing"
)

func TestDownCmd(t *testing.T) {
	if downCmd == nil {
		t.Fatal("downCmd should be initialized")
	}
	if downCmd.Use != "down" {
		t.Errorf("Use = %s, want down", downCmd.Use)
	}
	if downCmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if downCmd.Long == "" {
		t.Error("Long should not be empty")
	}
	if !downCmd.SilenceUsage {
		t.Error("SilenceUsage should be true")
	}
}

func TestDownCmdFlags(t *testing.T) {
	flags := []struct {
		name      string
		shorthand string
		defValue  string
	}{
		{"file", "f", ""},
		{"project", "p", ""},
		{"all", "", "false"},
		{"prune-shared", "", "false"},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			f := downCmd.Flags().Lookup(tt.name)
			if f == nil {
				t.Fatalf("flag %q not registered", tt.name)
			}
			if tt.shorthand != "" && f.Shorthand != tt.shorthand {
				t.Errorf("flag %q shorthand = %s, want %s",
					tt.name, f.Shorthand, tt.shorthand)
			}
			if f.DefValue != tt.defValue {
				t.Errorf("flag %q default = %s, want %s",
					tt.name, f.DefValue, tt.defValue)
			}
		})
	}
}

func TestDownCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "down" {
			found = true
			break
		}
	}
	if !found {
		t.Error("downCmd not registered on rootCmd")
	}
}
