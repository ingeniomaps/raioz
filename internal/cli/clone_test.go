package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCloneCmd(t *testing.T) {
	if cloneCmd == nil {
		t.Fatal("cloneCmd should be initialized")
	}
	if cloneCmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cloneCmd.Long == "" {
		t.Error("Long should not be empty")
	}
	if !cloneCmd.SilenceUsage {
		t.Error("SilenceUsage should be true")
	}
}

func TestCloneCmdFlags(t *testing.T) {
	flags := []struct {
		name      string
		shorthand string
	}{
		{"no-up", "n"},
		{"branch", "b"},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			f := cloneCmd.Flags().Lookup(tt.name)
			if f == nil {
				t.Fatalf("flag %q not registered", tt.name)
			}
			if f.Shorthand != tt.shorthand {
				t.Errorf("flag %q shorthand = %s, want %s",
					tt.name, f.Shorthand, tt.shorthand)
			}
		})
	}
}

func TestCloneCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "clone" {
			found = true
			break
		}
	}
	if !found {
		t.Error("cloneCmd not registered on rootCmd")
	}
}

func TestDirNameFromRepo(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"git@github.com:acme/platform.git", "platform"},
		{"https://github.com/acme/platform.git", "platform"},
		{"https://github.com/acme/platform", "platform"},
		{"git@github.com:acme/platform", "platform"},
		{"platform", "platform"},
		{"platform.git", "platform"},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := dirNameFromRepo(tt.url)
			if got != tt.want {
				t.Errorf("dirNameFromRepo(%q) = %q, want %q",
					tt.url, got, tt.want)
			}
		})
	}
}

func TestFindConfig(t *testing.T) {
	t.Run("finds raioz.yaml", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "raioz.yaml")
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		got := findConfig(dir)
		if got != p {
			t.Errorf("findConfig = %q, want %q", got, p)
		}
	})

	t.Run("finds raioz.yml", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "raioz.yml")
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		got := findConfig(dir)
		if got != p {
			t.Errorf("findConfig = %q, want %q", got, p)
		}
	})

	t.Run("finds .raioz.json", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, ".raioz.json")
		if err := os.WriteFile(p, []byte("{}"), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		got := findConfig(dir)
		if got != p {
			t.Errorf("findConfig = %q, want %q", got, p)
		}
	})

	t.Run("empty when none", func(t *testing.T) {
		dir := t.TempDir()
		if got := findConfig(dir); got != "" {
			t.Errorf("findConfig = %q, want empty", got)
		}
	})
}

func TestCloneCmdRunInvalidRepo(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	// Passing an obviously invalid repo URL should fail cleanly.
	err := cloneCmd.RunE(cloneCmd, []string{"/nonexistent/fake-repo-xyz-123"})
	if err == nil {
		t.Error("expected error for invalid repo, got nil")
	}
}
