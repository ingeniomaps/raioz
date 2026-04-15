package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGraphCmd(t *testing.T) {
	if graphCmd == nil {
		t.Fatal("graphCmd should be initialized")
	}
	if graphCmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if !graphCmd.SilenceUsage {
		t.Error("SilenceUsage should be true")
	}
}

func TestGraphCmdFlags(t *testing.T) {
	tests := []struct {
		name     string
		defValue string
	}{
		{"format", "ascii"},
		{"file", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := graphCmd.Flags().Lookup(tt.name)
			if f == nil {
				t.Fatalf("flag %q not registered", tt.name)
			}
			if f.DefValue != tt.defValue {
				t.Errorf("default = %s, want %s", f.DefValue, tt.defValue)
			}
		})
	}
}

func TestGraphCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "graph" {
			found = true
			break
		}
	}
	if !found {
		t.Error("graphCmd not registered on rootCmd")
	}
}

func TestGraphCmdRunInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	origPath := graphConfigPath
	graphConfigPath = filepath.Join(dir, "missing.yaml")
	defer func() { graphConfigPath = origPath }()

	err := graphCmd.RunE(graphCmd, []string{})
	if err == nil {
		t.Error("expected error loading missing config, got nil")
	}
}
