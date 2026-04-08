package cli

import (
	"testing"
)

func TestOverrideCmd(t *testing.T) {
	if overrideCmd == nil {
		t.Fatal("overrideCmd should be initialized")
	}
	if overrideCmd.Use != "override <service>" {
		t.Errorf("Use = %s, want 'override <service>'", overrideCmd.Use)
	}
	if overrideCmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestOverrideSubcommands(t *testing.T) {
	expected := []string{"list", "remove"}
	registered := make(map[string]bool)
	for _, cmd := range overrideCmd.Commands() {
		registered[cmd.Name()] = true
	}

	for _, name := range expected {
		t.Run(name, func(t *testing.T) {
			if !registered[name] {
				t.Errorf("subcommand %q not registered", name)
			}
		})
	}
}

func TestOverrideCmdArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no args rejected", []string{}, true},
		{"one arg accepted", []string{"my-svc"}, false},
		{"two args rejected", []string{"a", "b"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := overrideCmd.Args(overrideCmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Args(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestOverrideRemoveCmdAlias(t *testing.T) {
	if len(overrideRemoveCmd.Aliases) == 0 {
		t.Fatal("remove should have alias 'rm'")
	}
	if overrideRemoveCmd.Aliases[0] != "rm" {
		t.Errorf("alias = %s, want rm", overrideRemoveCmd.Aliases[0])
	}
}

func TestOverridePathFlag(t *testing.T) {
	f := overrideCmd.Flags().Lookup("path")
	if f == nil {
		t.Fatal("flag 'path' not registered")
	}
	if f.DefValue != "" {
		t.Errorf("path default = %s, want empty", f.DefValue)
	}
}

func TestOverrideCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "override" {
			found = true
			break
		}
	}
	if !found {
		t.Error("overrideCmd not registered on rootCmd")
	}
}
