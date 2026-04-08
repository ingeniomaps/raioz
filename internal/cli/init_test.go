package cli

import (
	"testing"
)

func TestInitCmd(t *testing.T) {
	if initCmd == nil {
		t.Fatal("initCmd should be initialized")
	}

	if initCmd.Use != "init" {
		t.Errorf("initCmd.Use = %s, want init", initCmd.Use)
	}

	if initCmd.Short == "" {
		t.Error("initCmd.Short should not be empty")
	}

	if initCmd.Long == "" {
		t.Error("initCmd.Long should not be empty")
	}

	if !initCmd.SilenceUsage {
		t.Error("initCmd.SilenceUsage should be true")
	}
}

func TestInitCmdFlags(t *testing.T) {
	t.Run("output flag registered", func(t *testing.T) {
		f := initCmd.Flags().Lookup("output")
		if f == nil {
			t.Fatal("flag 'output' not registered")
		}
		if f.Shorthand != "o" {
			t.Errorf("output shorthand = %s, want o", f.Shorthand)
		}
		if f.DefValue != ".raioz.json" {
			t.Errorf("output default = %s, want .raioz.json", f.DefValue)
		}
	})
}

func TestInitCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "init" {
			found = true
			break
		}
	}
	if !found {
		t.Error("initCmd not registered on rootCmd")
	}
}
