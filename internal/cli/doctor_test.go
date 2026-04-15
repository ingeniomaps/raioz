package cli

import (
	"testing"
)

func TestDoctorCmd(t *testing.T) {
	if doctorCmd == nil {
		t.Fatal("doctorCmd should be initialized")
	}
	if doctorCmd.Use != "doctor" {
		t.Errorf("Use = %s, want doctor", doctorCmd.Use)
	}
	if doctorCmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestDoctorCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "doctor" {
			found = true
			break
		}
	}
	if !found {
		t.Error("doctorCmd not registered on rootCmd")
	}
}
