package cli

import (
	"testing"
)

func TestCleanCmd(t *testing.T) {
	if cleanCmd == nil {
		t.Fatal("cleanCmd should be initialized")
	}
	if cleanCmd.Use != "clean" {
		t.Errorf("Use = %s, want clean", cleanCmd.Use)
	}
	if cleanCmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestCleanCmdFlags(t *testing.T) {
	flags := []struct {
		name     string
		defValue string
	}{
		{"file", ""},
		{"project", ""},
		{"all", "false"},
		{"images", "false"},
		{"volumes", "false"},
		{"networks", "false"},
		{"dry-run", "false"},
		{"force", "false"},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			f := cleanCmd.Flags().Lookup(tt.name)
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

func TestCleanCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "clean" {
			found = true
			break
		}
	}
	if !found {
		t.Error("cleanCmd not registered on rootCmd")
	}
}
