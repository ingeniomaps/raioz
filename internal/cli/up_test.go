package cli

import (
	"testing"
)

func TestUpCmd(t *testing.T) {
	if upCmd == nil {
		t.Fatal("upCmd should be initialized")
	}
	if upCmd.Use != "up" {
		t.Errorf("Use = %s, want up", upCmd.Use)
	}
	if upCmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if upCmd.Long == "" {
		t.Error("Long should not be empty")
	}
	if !upCmd.SilenceUsage {
		t.Error("SilenceUsage should be true")
	}
}

func TestUpCmdFlags(t *testing.T) {
	flags := []struct {
		name      string
		shorthand string
		defValue  string
	}{
		{"file", "f", ".raioz.json"},
		{"profile", "p", ""},
		{"force-reclone", "", "false"},
		{"dry-run", "", "false"},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			f := upCmd.Flags().Lookup(tt.name)
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

func TestUpCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "up" {
			found = true
			break
		}
	}
	if !found {
		t.Error("upCmd not registered on rootCmd")
	}
}
