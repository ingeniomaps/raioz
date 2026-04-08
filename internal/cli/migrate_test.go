package cli

import (
	"testing"
)

func TestMigrateCmd(t *testing.T) {
	if migrateCmd == nil {
		t.Fatal("migrateCmd should be initialized")
	}
	if migrateCmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestMigrateCmdFlags(t *testing.T) {
	flags := []struct {
		name      string
		shorthand string
	}{
		{"compose", "c"},
		{"output", "o"},
		{"project", "p"},
		{"network", ""},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			f := migrateCmd.Flags().Lookup(tt.name)
			if f == nil {
				t.Fatalf("flag %q not registered", tt.name)
			}
			if tt.shorthand != "" && f.Shorthand != tt.shorthand {
				t.Errorf("shorthand = %s, want %s", f.Shorthand, tt.shorthand)
			}
		})
	}
}

func TestMigrateCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "migrate" {
			found = true
			break
		}
	}
	if !found {
		t.Error("migrateCmd not registered on rootCmd")
	}
}
