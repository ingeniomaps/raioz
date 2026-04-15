package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotCmd(t *testing.T) {
	if snapshotCmd == nil {
		t.Fatal("snapshotCmd should be initialized")
	}
	if snapshotCmd.Use != "snapshot" {
		t.Errorf("Use = %s, want snapshot", snapshotCmd.Use)
	}
	if snapshotCmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if !snapshotCmd.SilenceUsage {
		t.Error("SilenceUsage should be true")
	}
}

func TestSnapshotCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "snapshot" {
			found = true
			break
		}
	}
	if !found {
		t.Error("snapshotCmd not registered on rootCmd")
	}
}

func TestSnapshotSubcommandsRegistered(t *testing.T) {
	expected := []string{"create", "restore", "list", "delete"}
	registered := make(map[string]bool)
	for _, sub := range snapshotCmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("snapshot subcommand %q not registered", name)
		}
	}
}

func TestSnapshotPersistentFlag(t *testing.T) {
	f := snapshotCmd.PersistentFlags().Lookup("file")
	if f == nil {
		t.Fatal("persistent flag 'file' not registered")
	}
	if f.Shorthand != "f" {
		t.Errorf("shorthand = %s, want f", f.Shorthand)
	}
}

func TestSnapshotCreateRunInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	orig := snapshotConfigPath
	snapshotConfigPath = filepath.Join(dir, "missing.yaml")
	defer func() { snapshotConfigPath = orig }()

	err := snapshotCreateCmd.RunE(snapshotCreateCmd, []string{"mysnap"})
	if err == nil {
		t.Error("expected error loading missing config, got nil")
	}
}

func TestSnapshotRestoreRunInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	orig := snapshotConfigPath
	snapshotConfigPath = filepath.Join(dir, "missing.yaml")
	defer func() { snapshotConfigPath = orig }()

	err := snapshotRestoreCmd.RunE(snapshotRestoreCmd, []string{"mysnap"})
	if err == nil {
		t.Error("expected error loading missing config, got nil")
	}
}

func TestSnapshotListRunInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	orig := snapshotConfigPath
	snapshotConfigPath = filepath.Join(dir, "missing.yaml")
	defer func() { snapshotConfigPath = orig }()

	err := snapshotListCmd.RunE(snapshotListCmd, []string{})
	if err == nil {
		t.Error("expected error loading missing config, got nil")
	}
}

func TestSnapshotDeleteRunInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	orig := snapshotConfigPath
	snapshotConfigPath = filepath.Join(dir, "missing.yaml")
	defer func() { snapshotConfigPath = orig }()

	err := snapshotDeleteCmd.RunE(snapshotDeleteCmd, []string{"mysnap"})
	if err == nil {
		t.Error("expected error loading missing config, got nil")
	}
}
