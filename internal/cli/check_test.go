package cli

import (
	"testing"
)

func TestCheckCmd(t *testing.T) {
	if checkCmd == nil {
		t.Fatal("checkCmd should be initialized")
	}

	if checkCmd.Use != "check" {
		t.Errorf("checkCmd.Use = %s, want check", checkCmd.Use)
	}

	if checkCmd.Short == "" {
		t.Error("checkCmd.Short should not be empty")
	}

	if checkCmd.Long == "" {
		t.Error("checkCmd.Long should not be empty")
	}

	if !checkCmd.SilenceUsage {
		t.Error("checkCmd.SilenceUsage should be true")
	}
}

func TestCheckCmdFlags(t *testing.T) {
	flags := []struct {
		name      string
		shorthand string
		defValue  string
	}{
		{"file", "f", ".raioz.json"},
		{"project", "p", ""},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			f := checkCmd.Flags().Lookup(tt.name)
			if f == nil {
				t.Fatalf("flag %q not registered", tt.name)
			}
			if f.Shorthand != tt.shorthand {
				t.Errorf("flag %q shorthand = %s, want %s", tt.name, f.Shorthand, tt.shorthand)
			}
			if f.DefValue != tt.defValue {
				t.Errorf("flag %q default = %s, want %s", tt.name, f.DefValue, tt.defValue)
			}
		})
	}
}

func TestCheckCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "check" {
			found = true
			break
		}
	}
	if !found {
		t.Error("checkCmd not registered on rootCmd")
	}
}
