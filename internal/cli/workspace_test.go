package cli

import (
	"testing"
)

func TestWorkspaceCmd(t *testing.T) {
	if workspaceCmd == nil {
		t.Fatal("workspaceCmd should be initialized")
	}
	if workspaceCmd.Use != "workspace" {
		t.Errorf("Use = %s, want workspace", workspaceCmd.Use)
	}
	if workspaceCmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if workspaceCmd.RunE == nil {
		t.Error("parent command should have RunE (shows current workspace)")
	}
}

func TestWorkspaceSubcommands(t *testing.T) {
	expected := []string{"use", "list", "delete"}
	registered := make(map[string]bool)
	for _, cmd := range workspaceCmd.Commands() {
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

func TestWorkspaceUseCmdArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no args rejected", []string{}, true},
		{"one arg accepted", []string{"my-ws"}, false},
		{"two args rejected", []string{"a", "b"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := workspaceUseCmd.Args(workspaceUseCmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Args(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestWorkspaceDeleteCmdArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no args rejected", []string{}, true},
		{"one arg accepted", []string{"my-ws"}, false},
		{"two args rejected", []string{"a", "b"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := workspaceDeleteCmd.Args(workspaceDeleteCmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Args(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestWorkspaceCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "workspace" {
			found = true
			break
		}
	}
	if !found {
		t.Error("workspaceCmd not registered on rootCmd")
	}
}
