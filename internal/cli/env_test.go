package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvCmd(t *testing.T) {
	if envCmd == nil {
		t.Fatal("envCmd should be initialized")
	}
	if envCmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if envCmd.Long == "" {
		t.Error("Long should not be empty")
	}
	if !envCmd.SilenceUsage {
		t.Error("SilenceUsage should be true")
	}
}

func TestEnvCmdFlag(t *testing.T) {
	f := envCmd.Flags().Lookup("file")
	if f == nil {
		t.Fatal("flag 'file' not registered")
	}
	if f.Shorthand != "f" {
		t.Errorf("shorthand = %s, want f", f.Shorthand)
	}
}

func TestEnvCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "env" {
			found = true
			break
		}
	}
	if !found {
		t.Error("envCmd not registered on rootCmd")
	}
}

func TestEnvCmdRunInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	// Set the file flag to an invalid path
	if err := envCmd.Flags().Set("file", filepath.Join(dir, "missing.yaml")); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	defer func() { _ = envCmd.Flags().Set("file", "") }()

	err := envCmd.RunE(envCmd, []string{"some-service"})
	if err == nil {
		t.Error("expected error loading missing config, got nil")
	}
}
