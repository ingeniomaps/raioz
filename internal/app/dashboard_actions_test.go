package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeRaioz writes a stand-in raioz that records its arguments and exits
// with the given code after printing output.
func fakeRaioz(t *testing.T, output string, exitCode string) (argsFile string) {
	t.Helper()
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args")
	bin := filepath.Join(dir, "raioz")
	script := "#!/bin/sh\necho \"$PWD|$*\" > " + argsFile + "\nprintf '" + output + "'\nexit " + exitCode + "\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := selfBinaryFn
	selfBinaryFn = func() (string, error) { return bin, nil }
	t.Cleanup(func() { selfBinaryFn = orig })
	return argsFile
}

func TestHostServiceAction(t *testing.T) {
	projectDir := t.TempDir()
	cfg := filepath.Join(projectDir, "raioz.yaml")

	tests := []struct {
		name, action, wantVerb string
	}{
		{"restart runs raioz restart", "restart", "restart"},
		{"stop runs a selective down", "stop", "down"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argsFile := fakeRaioz(t, "", "0")
			if err := HostServiceAction(context.Background(), cfg, tt.action, "web"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, _ := os.ReadFile(argsFile)
			want := projectDir + "|" + tt.wantVerb + " --file " + cfg + " web"
			if strings.TrimSpace(string(got)) != want {
				t.Errorf("ran %q, want %q", strings.TrimSpace(string(got)), want)
			}
		})
	}

	t.Run("failure reports the command's last line without colour", func(t *testing.T) {
		fakeRaioz(t, `  working\n  \033[31m[error]\033[0m the lock already exists\n\n`, "1")
		err := HostServiceAction(context.Background(), cfg, "restart", "web")
		if err == nil || err.Error() != "the lock already exists" {
			t.Errorf("got %v", err)
		}
	})

	t.Run("silent failure still names the command", func(t *testing.T) {
		fakeRaioz(t, "", "3")
		err := HostServiceAction(context.Background(), cfg, "stop", "web")
		if err == nil || !strings.Contains(err.Error(), "raioz down web") {
			t.Errorf("got %v", err)
		}
	})

	t.Run("unknown action runs nothing", func(t *testing.T) {
		argsFile := fakeRaioz(t, "", "0")
		if err := HostServiceAction(context.Background(), cfg, "promote", "web"); err == nil {
			t.Error("expected an error")
		}
		if _, err := os.Stat(argsFile); err == nil {
			t.Error("no command should have run")
		}
	})
}
