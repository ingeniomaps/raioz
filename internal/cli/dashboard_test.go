package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDashboardCmd(t *testing.T) {
	if dashboardCmd == nil {
		t.Fatal("dashboardCmd should be initialized")
	}
	if dashboardCmd.Use != "dashboard" {
		t.Errorf("Use = %s, want dashboard", dashboardCmd.Use)
	}
	if dashboardCmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if !dashboardCmd.SilenceUsage {
		t.Error("SilenceUsage should be true")
	}
}

func TestDashboardCmdAlias(t *testing.T) {
	found := false
	for _, alias := range dashboardCmd.Aliases {
		if alias == "tui" {
			found = true
			break
		}
	}
	if !found {
		t.Error("dashboardCmd should have 'tui' alias")
	}
}

func TestDashboardCmdFlags(t *testing.T) {
	f := dashboardCmd.Flags().Lookup("file")
	if f == nil {
		t.Fatal("flag 'file' not registered")
	}
	if f.Shorthand != "f" {
		t.Errorf("shorthand = %s, want f", f.Shorthand)
	}
}

func TestDashboardCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "dashboard" {
			found = true
			break
		}
	}
	if !found {
		t.Error("dashboardCmd not registered on rootCmd")
	}
}

func TestDashboardRunInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	// Point to a non-existent config; legacy path should fail to load.
	origPath := dashboardConfigPath
	dashboardConfigPath = filepath.Join(dir, "nope.yaml")
	defer func() { dashboardConfigPath = origPath }()

	err := dashboardCmd.RunE(dashboardCmd, []string{})
	if err == nil {
		t.Error("expected error loading missing config, got nil")
	}
}
