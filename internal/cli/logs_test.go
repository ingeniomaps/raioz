package cli

import (
	"testing"
)

func TestLogsCmd(t *testing.T) {
	if logsCmd == nil {
		t.Fatal("logsCmd should be initialized")
	}
	if logsCmd.Use != "logs [service...]" {
		t.Errorf("Use = %s, want 'logs [service...]'", logsCmd.Use)
	}
	if logsCmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestLogsCmdFlags(t *testing.T) {
	flags := []struct {
		name     string
		defValue string
	}{
		{"file", ""},
		{"project", ""},
		{"follow", "false"},
		{"tail", "0"},
		{"all", "false"},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			f := logsCmd.Flags().Lookup(tt.name)
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

func TestLogsCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "logs" {
			found = true
			break
		}
	}
	if !found {
		t.Error("logsCmd not registered on rootCmd")
	}
}
