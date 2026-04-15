package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOSFileSystem_WriteAndReadFile(t *testing.T) {
	fs := NewOSFileSystem()
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	content := []byte("hello world")

	if err := fs.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	got, err := fs.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if string(got) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestOSFileSystem_ReadFile_NotFound(t *testing.T) {
	fs := NewOSFileSystem()
	_, err := fs.ReadFile(filepath.Join(t.TempDir(), "nope.txt"))
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestOSFileSystem_Stat(t *testing.T) {
	fs := NewOSFileSystem()
	dir := t.TempDir()
	path := filepath.Join(dir, "stat.txt")

	if err := fs.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	info, err := fs.Stat(path)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if info.Size() != 1 {
		t.Errorf("expected size 1, got %d", info.Size())
	}
}

func TestOSFileSystem_Stat_NotFound(t *testing.T) {
	fs := NewOSFileSystem()
	_, err := fs.Stat(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Error("expected error")
	}
}

func TestOSFileSystem_MkdirAllAndRemoveAll(t *testing.T) {
	fs := NewOSFileSystem()
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "c")

	if err := fs.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if _, err := os.Stat(nested); err != nil {
		t.Errorf("directory not created: %v", err)
	}

	if err := fs.RemoveAll(filepath.Join(dir, "a")); err != nil {
		t.Fatalf("RemoveAll failed: %v", err)
	}
	if _, err := os.Stat(nested); !os.IsNotExist(err) {
		t.Error("expected directory to be removed")
	}
}

func TestOSFileSystem_Open(t *testing.T) {
	fs := NewOSFileSystem()
	dir := t.TempDir()
	path := filepath.Join(dir, "open.txt")

	if err := fs.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	f, err := fs.Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	buf := make([]byte, 4)
	if _, err := f.Read(buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf) != "data" {
		t.Errorf("got %q, want %q", buf, "data")
	}
}

func TestOSFileSystem_Create(t *testing.T) {
	fs := NewOSFileSystem()
	path := filepath.Join(t.TempDir(), "created.txt")

	f, err := fs.Create(path)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := f.WriteString("new"); err != nil {
		t.Fatalf("write: %v", err)
	}
	f.Close()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if string(got) != "new" {
		t.Errorf("got %q, want %q", got, "new")
	}
}

func TestOSFileSystem_OpenFile(t *testing.T) {
	fs := NewOSFileSystem()
	path := filepath.Join(t.TempDir(), "append.txt")

	if err := os.WriteFile(path, []byte("a"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	f, err := fs.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("OpenFile failed: %v", err)
	}
	if _, err := f.WriteString("b"); err != nil {
		t.Fatalf("write: %v", err)
	}
	f.Close()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if string(got) != "ab" {
		t.Errorf("got %q, want %q", got, "ab")
	}
}
