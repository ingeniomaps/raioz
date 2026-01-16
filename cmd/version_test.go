package cmd

import (
	"testing"
)

func TestVersionCmd(t *testing.T) {
	// Test that versionCmd is registered
	if versionCmd == nil {
		t.Error("versionCmd should be initialized")
	}

	if versionCmd.Use != "version" {
		t.Errorf("versionCmd.Use = %s, want version", versionCmd.Use)
	}

	if versionCmd.Short == "" {
		t.Error("versionCmd.Short should not be empty")
	}
}

func TestVersionInfo(t *testing.T) {
	// Test default values
	if Version == "" {
		t.Error("Version should have a default value")
	}

	if SchemaVersion == "" {
		t.Error("SchemaVersion should have a default value")
	}

	if SchemaVersion != "1.0" {
		t.Errorf("SchemaVersion = %s, want 1.0", SchemaVersion)
	}
}
