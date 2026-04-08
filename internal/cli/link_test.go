package cli

import (
	"testing"
)

func TestLinkCmd(t *testing.T) {
	if linkCmd == nil {
		t.Fatal("linkCmd should be initialized")
	}
	if linkCmd.Use != "link" {
		t.Errorf("Use = %s, want link", linkCmd.Use)
	}
	if linkCmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestLinkSubcommands(t *testing.T) {
	expected := []string{"add", "remove", "list"}
	registered := make(map[string]bool)
	for _, cmd := range linkCmd.Commands() {
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

func TestLinkAddCmdArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no args rejected", []string{}, true},
		{"one arg rejected", []string{"svc"}, true},
		{"two args accepted", []string{"svc", "/path"}, false},
		{"three args rejected", []string{"a", "b", "c"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := linkAddCmd.Args(linkAddCmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Args(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestLinkRemoveCmdArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no args rejected", []string{}, true},
		{"one arg accepted", []string{"svc"}, false},
		{"two args rejected", []string{"a", "b"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := linkRemoveCmd.Args(linkRemoveCmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Args(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestLinkRemoveCmdAliases(t *testing.T) {
	expected := map[string]bool{"rm": false, "unlink": false}
	for _, alias := range linkRemoveCmd.Aliases {
		expected[alias] = true
	}

	for alias, found := range expected {
		if !found {
			t.Errorf("alias %q not found", alias)
		}
	}
}

func TestLinkCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "link" {
			found = true
			break
		}
	}
	if !found {
		t.Error("linkCmd not registered on rootCmd")
	}
}
