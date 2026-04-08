package cli

import (
	"testing"
)

func TestStatusCmd(t *testing.T) {
	if statusCmd == nil {
		t.Fatal("statusCmd should be initialized")
	}
	if statusCmd.Use != "status" {
		t.Errorf("Use = %s, want status", statusCmd.Use)
	}
	if statusCmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if statusCmd.Long == "" {
		t.Error("Long should not be empty")
	}
	if !statusCmd.SilenceUsage {
		t.Error("SilenceUsage should be true")
	}
}

func TestStatusCmdFlags(t *testing.T) {
	flags := []struct {
		name      string
		shorthand string
		defValue  string
	}{
		{"file", "f", ".raioz.json"},
		{"project", "p", ""},
		{"json", "", "false"},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			f := statusCmd.Flags().Lookup(tt.name)
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

func TestStatusCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "status" {
			found = true
			break
		}
	}
	if !found {
		t.Error("statusCmd not registered on rootCmd")
	}
}
