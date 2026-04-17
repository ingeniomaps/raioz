package cli

import (
	"testing"
)

func TestDownCmd(t *testing.T) {
	if downCmd == nil {
		t.Fatal("downCmd should be initialized")
	}
	if downCmd.Use != "down" {
		t.Errorf("Use = %s, want down", downCmd.Use)
	}
	if downCmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if downCmd.Long == "" {
		t.Error("Long should not be empty")
	}
	if !downCmd.SilenceUsage {
		t.Error("SilenceUsage should be true")
	}
}

func TestDownCmdFlags(t *testing.T) {
	flags := []struct {
		name      string
		shorthand string
		defValue  string
	}{
		{"file", "f", ""},
		{"project", "p", ""},
		{"all", "", "false"},
		{"prune-shared", "", "false"},
		{"conflicting", "", "false"},
		{"all-projects", "", "false"},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			f := downCmd.Flags().Lookup(tt.name)
			if f == nil {
				t.Fatalf("flag %q not registered", tt.name)
			}
			if tt.shorthand != "" && f.Shorthand != tt.shorthand {
				t.Errorf("flag %q shorthand = %s, want %s",
					tt.name, f.Shorthand, tt.shorthand)
			}
			if f.DefValue != tt.defValue {
				t.Errorf("flag %q default = %s, want %s",
					tt.name, f.DefValue, tt.defValue)
			}
		})
	}
}

// TestDownCmd_ConflictingAndAllProjectsMutex: --conflicting and
// --all-projects target overlapping concerns (free host ports vs nuke every
// other project). They must reject combined use to avoid silently picking
// one over the other.
func TestDownCmd_ConflictingAndAllProjectsMutex(t *testing.T) {
	prevConflicting, prevAll := downConflicting, downAllProjects
	defer func() {
		downConflicting, downAllProjects = prevConflicting, prevAll
	}()
	downConflicting = true
	downAllProjects = true

	err := downCmd.RunE(downCmd, nil)
	if err == nil {
		t.Fatal("expected error when --conflicting and --all-projects are both set")
	}
	if !contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusivity, got: %v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestDownCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "down" {
			found = true
			break
		}
	}
	if !found {
		t.Error("downCmd not registered on rootCmd")
	}
}
