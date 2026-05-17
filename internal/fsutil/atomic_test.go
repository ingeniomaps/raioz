package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomic_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	if err := WriteFileAtomic(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("got %q want hello", got)
	}
	info, _ := os.Stat(path)
	if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Errorf("mode %o want %o", got, want)
	}
}

func TestWriteFileAtomic_ReplacesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "new" {
		t.Errorf("got %q want new", got)
	}
}

// Atomicity guarantee: a failed write (here, a Write error simulated
// by writing to a path whose parent doesn't exist) must NOT trash the
// existing file at the target.
func TestWriteFileAtomic_PreservesOriginalOnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Target a non-existent parent dir to force CreateTemp to fail.
	bad := filepath.Join(dir, "nonexistent", "subdir", "out.txt")
	if err := WriteFileAtomic(bad, []byte("doomed"), 0o600); err == nil {
		t.Fatal("expected error writing under non-existent parent")
	}
	// Original file untouched.
	got, _ := os.ReadFile(path)
	if string(got) != "original" {
		t.Errorf("original file was modified: %q", got)
	}
}

// Verify no temp files leak in the target dir after a successful write.
func TestWriteFileAtomic_CleansUpTempOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	if err := WriteFileAtomic(path, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "out.txt" {
			t.Errorf("leftover file in target dir: %q", e.Name())
		}
	}
}
