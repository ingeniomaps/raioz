package cli

import (
	"testing"
)

func TestExecCmd(t *testing.T) {
	if execCmd == nil {
		t.Fatal("execCmd should be initialized")
	}
	if execCmd.Use != "exec <service> [command...]" {
		t.Errorf("Use = %s, want 'exec <service> [command...]'", execCmd.Use)
	}
	if execCmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if !execCmd.SilenceUsage {
		t.Error("SilenceUsage should be true")
	}
}

func TestExecCmdArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no args rejected", []string{}, true},
		{"one arg accepted (service only)", []string{"api"}, false},
		{"two args accepted (service + command)", []string{"api", "sh"}, false},
		{"many args accepted", []string{"api", "ls", "-la", "/app"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := execCmd.Args(execCmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Args(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestExecCmdFlags(t *testing.T) {
	flags := []struct {
		name     string
		defValue string
	}{
		{"file", ""},
		{"project", ""},
		{"interactive", "true"},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			f := execCmd.Flags().Lookup(tt.name)
			if f == nil {
				t.Fatalf("flag %q not registered", tt.name)
			}
			if f.DefValue != tt.defValue {
				t.Errorf("flag %q default = %s, want %s",
					tt.name, f.DefValue, tt.defValue)
			}
		})
	}
}

func TestExecCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "exec" {
			found = true
			break
		}
	}
	if !found {
		t.Error("execCmd not registered on rootCmd")
	}
}
