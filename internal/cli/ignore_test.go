package cli

import (
	"testing"
)

func TestIgnoreCmd(t *testing.T) {
	if ignoreCmd == nil {
		t.Fatal("ignoreCmd should be initialized")
	}
	if ignoreCmd.Use != "ignore" {
		t.Errorf("Use = %s, want ignore", ignoreCmd.Use)
	}
	if ignoreCmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestIgnoreSubcommands(t *testing.T) {
	expected := []string{"add", "remove", "list"}
	registered := make(map[string]bool)
	for _, cmd := range ignoreCmd.Commands() {
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

func TestIgnoreAddCmdArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no args rejected", []string{}, true},
		{"one arg accepted", []string{"my-svc"}, false},
		{"multiple args accepted", []string{"a", "b", "c"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ignoreAddCmd.Args(ignoreAddCmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Args(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestIgnoreRemoveCmdArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no args rejected", []string{}, true},
		{"one arg accepted", []string{"my-svc"}, false},
		{"multiple args accepted", []string{"a", "b"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ignoreRemoveCmd.Args(ignoreRemoveCmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Args(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestIgnoreRemoveCmdAlias(t *testing.T) {
	if len(ignoreRemoveCmd.Aliases) == 0 {
		t.Fatal("remove should have alias 'rm'")
	}
	if ignoreRemoveCmd.Aliases[0] != "rm" {
		t.Errorf("alias = %s, want rm", ignoreRemoveCmd.Aliases[0])
	}
}

func TestIgnoreCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "ignore" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ignoreCmd not registered on rootCmd")
	}
}
