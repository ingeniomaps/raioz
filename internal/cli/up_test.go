package cli

import (
	"strings"
	"testing"
)

// mergeOnlyArgs is the symmetry shim that lets `raioz up api` behave
// identically to `raioz down api` / `raioz restart api`. The function
// itself is trivial; pinning the union+dedup behavior here protects the
// CLI contract against accidental regressions.
func TestMergeOnlyArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		flag []string
		want string
	}{
		{"both empty", nil, nil, ""},
		{"args only", []string{"api"}, nil, "api"},
		{"flag only", nil, []string{"web"}, "web"},
		{"args first, flag later", []string{"api"}, []string{"web"}, "api,web"},
		{"dedup across both", []string{"api", "web"}, []string{"web"}, "api,web"},
		{"dedup within args", []string{"api", "api"}, nil, "api"},
		{"empty strings dropped", []string{"", "api"}, []string{"", "web"}, "api,web"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strings.Join(mergeOnlyArgs(tt.args, tt.flag), ",")
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUpCmd(t *testing.T) {
	if upCmd == nil {
		t.Fatal("upCmd should be initialized")
	}
	if upCmd.Use != "up [service...]" {
		t.Errorf("Use = %s, want up [service...]", upCmd.Use)
	}
	if upCmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if upCmd.Long == "" {
		t.Error("Long should not be empty")
	}
	if !upCmd.SilenceUsage {
		t.Error("SilenceUsage should be true")
	}
}

func TestUpCmdFlags(t *testing.T) {
	flags := []struct {
		name      string
		shorthand string
		defValue  string
	}{
		{"file", "f", ""},
		{"profile", "p", ""},
		{"force-reclone", "", "false"},
		{"dry-run", "", "false"},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			f := upCmd.Flags().Lookup(tt.name)
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

func TestUpCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "up" {
			found = true
			break
		}
	}
	if !found {
		t.Error("upCmd not registered on rootCmd")
	}
}
