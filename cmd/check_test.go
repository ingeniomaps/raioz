package cmd

import (
	"testing"
)

func TestCheckCmd(t *testing.T) {
	// Test that checkCmd is registered
	if checkCmd == nil {
		t.Error("checkCmd should be initialized")
	}

	if checkCmd.Use != "check" {
		t.Errorf("checkCmd.Use = %s, want check", checkCmd.Use)
	}

	if checkCmd.Short == "" {
		t.Error("checkCmd.Short should not be empty")
	}

	if checkCmd.Long == "" {
		t.Error("checkCmd.Long should not be empty")
	}
}
