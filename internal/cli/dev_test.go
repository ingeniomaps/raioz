package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDevCmd(t *testing.T) {
	if devCmd == nil {
		t.Fatal("devCmd should be initialized")
	}
	if devCmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if devCmd.Long == "" {
		t.Error("Long should not be empty")
	}
	if !devCmd.SilenceUsage {
		t.Error("SilenceUsage should be true")
	}
}

func TestDevCmdFlags(t *testing.T) {
	flags := []struct {
		name      string
		shorthand string
		defValue  string
	}{
		{"file", "f", ""},
		{"reset", "", "false"},
		{"list", "", "false"},
	}
	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			f := devCmd.Flags().Lookup(tt.name)
			if f == nil {
				t.Fatalf("flag %q not registered", tt.name)
			}
			if tt.shorthand != "" && f.Shorthand != tt.shorthand {
				t.Errorf("shorthand = %s, want %s", f.Shorthand, tt.shorthand)
			}
			if f.DefValue != tt.defValue {
				t.Errorf("default = %s, want %s", f.DefValue, tt.defValue)
			}
		})
	}
}

func TestDevCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "dev" {
			found = true
			break
		}
	}
	if !found {
		t.Error("devCmd not registered on rootCmd")
	}
}

func TestDevCmdRunInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	origPath := devConfigPath
	devConfigPath = filepath.Join(dir, "missing.yaml")
	defer func() { devConfigPath = origPath }()

	origList := devList
	origReset := devReset
	devList = false
	devReset = false
	defer func() {
		devList = origList
		devReset = origReset
	}()

	// Supply a dependency name; loading the non-existent config should fail.
	err := devCmd.RunE(devCmd, []string{"redis", "./path"})
	if err == nil {
		t.Error("expected error with missing config, got nil")
	}
}

func TestDevCmdRunListMode(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	origPath := devConfigPath
	// List mode doesn't load config; point at the temp dir so the list
	// branch runs without side effects in the real CWD.
	devConfigPath = filepath.Join(dir, "raioz.yaml")
	defer func() { devConfigPath = origPath }()

	origList := devList
	devList = true
	defer func() { devList = origList }()

	if err := devCmd.RunE(devCmd, []string{}); err != nil {
		t.Errorf("list mode unexpected error: %v", err)
	}
}
