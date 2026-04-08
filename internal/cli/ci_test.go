package cli

import (
	"testing"
)

func TestCiCmd(t *testing.T) {
	if ciCmd == nil {
		t.Fatal("ciCmd should be initialized")
	}
	if ciCmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if ciCmd.Long == "" {
		t.Error("Long should not be empty")
	}
}

func TestCiCmdFlags(t *testing.T) {
	flags := []string{
		"file", "keep", "ephemeral", "job-id",
		"skip-build", "skip-pull", "only-validate", "force-reclone",
	}

	for _, name := range flags {
		t.Run(name, func(t *testing.T) {
			f := ciCmd.Flags().Lookup(name)
			if f == nil {
				t.Fatalf("flag %q not registered", name)
			}
		})
	}
}

func TestCiCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "ci" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ciCmd not registered on rootCmd")
	}
}
