package cli

import (
	"testing"
)

func TestRestartCmd(t *testing.T) {
	if restartCmd == nil {
		t.Fatal("restartCmd should be initialized")
	}
	if restartCmd.Use != "restart [service...]" {
		t.Errorf("Use = %s, want 'restart [service...]'", restartCmd.Use)
	}
	if restartCmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if !restartCmd.SilenceUsage {
		t.Error("SilenceUsage should be true")
	}
}

func TestRestartCmdFlags(t *testing.T) {
	flags := []struct {
		name     string
		defValue string
	}{
		{"file", ""},
		{"project", ""},
		{"all", "false"},
		{"include-infra", "false"},
		{"force-recreate", "false"},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			f := restartCmd.Flags().Lookup(tt.name)
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

func TestRestartCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "restart" {
			found = true
			break
		}
	}
	if !found {
		t.Error("restartCmd not registered on rootCmd")
	}
}
