package proxy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCertsDir(t *testing.T) {
	dir := CertsDir()
	if dir == "" {
		t.Skip("no home dir available")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("expected absolute path, got %q", dir)
	}
}

func TestEnsureCerts_AlreadyExist(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	certsDir := filepath.Join(home, ".raioz", "certs")
	if err := os.MkdirAll(certsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Pre-create cert files so EnsureCerts returns early without calling mkcert
	os.WriteFile(filepath.Join(certsDir, certFileName), []byte("fake cert"), 0o644)
	os.WriteFile(filepath.Join(certsDir, keyFileName), []byte("fake key"), 0o644)

	got, err := EnsureCerts("localhost")
	if err != nil {
		t.Fatalf("EnsureCerts: %v", err)
	}
	if got != certsDir {
		t.Errorf("got %q, want %q", got, certsDir)
	}
}

func TestEnsureCerts_DefaultDomain(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	certsDir := filepath.Join(home, ".raioz", "certs")
	os.MkdirAll(certsDir, 0o755)
	os.WriteFile(filepath.Join(certsDir, certFileName), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(certsDir, keyFileName), []byte("x"), 0o644)

	// Empty domain should default to "localhost"
	got, err := EnsureCerts("")
	if err != nil {
		t.Fatalf("EnsureCerts: %v", err)
	}
	if got == "" {
		t.Error("expected non-empty dir")
	}
}

func TestCommandExists_True(t *testing.T) {
	// `sh` should exist on any Unix
	if !commandExists("sh") {
		t.Skip("sh not found — unusual env")
	}
}

func TestCommandExists_False(t *testing.T) {
	if commandExists("this-command-should-not-exist-anywhere-raioz-test") {
		t.Error("expected false for fake command")
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	os.WriteFile(file, []byte("x"), 0o644)

	if !fileExists(file) {
		t.Error("expected true for existing file")
	}
	if fileExists(filepath.Join(dir, "nope.txt")) {
		t.Error("expected false for non-existent file")
	}
	// Directory should return false
	if fileExists(dir) {
		t.Error("expected false for directory")
	}
}
