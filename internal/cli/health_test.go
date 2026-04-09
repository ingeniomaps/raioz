package cli

import (
	"testing"
)

func TestHealthCmd(t *testing.T) {
	if healthCmd == nil {
		t.Fatal("healthCmd should be initialized")
	}
	if healthCmd.Use != "health" {
		t.Errorf("Use = %s, want health", healthCmd.Use)
	}
	if healthCmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if !healthCmd.SilenceUsage {
		t.Error("SilenceUsage should be true")
	}
}

func TestHealthCmdFlags(t *testing.T) {
	f := healthCmd.Flags().Lookup("file")
	if f == nil {
		t.Fatal("flag 'file' not registered")
	}
	if f.DefValue != "" {
		t.Errorf("default = %s, want empty", f.DefValue)
	}
}

func TestHealthCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "health" {
			found = true
			break
		}
	}
	if !found {
		t.Error("healthCmd not registered on rootCmd")
	}
}
