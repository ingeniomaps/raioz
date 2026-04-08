package cli

import (
	"testing"
)

func TestVolumesCmd(t *testing.T) {
	if volumesCmd == nil {
		t.Fatal("volumesCmd should be initialized")
	}
	if volumesCmd.Use != "volumes" {
		t.Errorf("Use = %s, want 'volumes'", volumesCmd.Use)
	}
	if volumesCmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestVolumesListCmd(t *testing.T) {
	if volumesListCmd == nil {
		t.Fatal("volumesListCmd should be initialized")
	}
	if volumesListCmd.Use != "list" {
		t.Errorf("Use = %s, want 'list'", volumesListCmd.Use)
	}
}

func TestVolumesRemoveCmd(t *testing.T) {
	if volumesRemoveCmd == nil {
		t.Fatal("volumesRemoveCmd should be initialized")
	}
	if volumesRemoveCmd.Use != "remove [volume...]" {
		t.Errorf("Use = %s, want 'remove [volume...]'", volumesRemoveCmd.Use)
	}

	// Check aliases
	if len(volumesRemoveCmd.Aliases) != 1 || volumesRemoveCmd.Aliases[0] != "rm" {
		t.Errorf("Aliases = %v, want [rm]", volumesRemoveCmd.Aliases)
	}
}

func TestVolumesRemoveCmdFlags(t *testing.T) {
	flags := []struct {
		name     string
		defValue string
	}{
		{"all", "false"},
		{"force", "false"},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			f := volumesRemoveCmd.Flags().Lookup(tt.name)
			if f == nil {
				t.Fatalf("flag %q not registered", tt.name)
			}
			if f.DefValue != tt.defValue {
				t.Errorf("flag %q default = %s, want %s", tt.name, f.DefValue, tt.defValue)
			}
		})
	}
}

func TestVolumesPersistentFlags(t *testing.T) {
	flags := []struct {
		name     string
		defValue string
	}{
		{"file", ".raioz.json"},
		{"project", ""},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			f := volumesCmd.PersistentFlags().Lookup(tt.name)
			if f == nil {
				t.Fatalf("persistent flag %q not registered", tt.name)
			}
			if f.DefValue != tt.defValue {
				t.Errorf("flag %q default = %s, want %s", tt.name, f.DefValue, tt.defValue)
			}
		})
	}
}

func TestVolumesSubcommandsRegistered(t *testing.T) {
	subcommands := map[string]bool{"list": false, "remove": false}

	for _, cmd := range volumesCmd.Commands() {
		if _, ok := subcommands[cmd.Name()]; ok {
			subcommands[cmd.Name()] = true
		}
	}

	for name, found := range subcommands {
		if !found {
			t.Errorf("subcommand %q not registered on volumesCmd", name)
		}
	}
}

func TestVolumesRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "volumes" {
			found = true
			break
		}
	}
	if !found {
		t.Error("volumesCmd not registered on rootCmd")
	}
}
