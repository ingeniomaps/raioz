package cli

import (
	"testing"
)

func TestCompareCmd(t *testing.T) {
	if compareCmd == nil {
		t.Fatal("compareCmd should be initialized")
	}
	if compareCmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestCompareCmdFlags(t *testing.T) {
	flags := []struct {
		name string
	}{
		{"file"},
		{"production"},
		{"json"},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			f := compareCmd.Flags().Lookup(tt.name)
			if f == nil {
				t.Fatalf("flag %q not registered", tt.name)
			}
		})
	}
}

func TestCompareCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "compare" {
			found = true
			break
		}
	}
	if !found {
		t.Error("compareCmd not registered on rootCmd")
	}
}
