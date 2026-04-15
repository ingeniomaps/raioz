package watch

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew_Basic(t *testing.T) {
	dir := t.TempDir()
	apiDir := filepath.Join(dir, "api")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	w, err := New(Config{
		ServicePaths: map[string]string{"api": apiDir},
		OnRestart:    func(string) {},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer w.Close()

	if w == nil {
		t.Fatal("expected non-nil watcher")
	}
}

func TestNew_DefaultDebounce(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	w, err := New(Config{
		ServicePaths: map[string]string{"s": dir},
		OnRestart:    func(string) {},
		// Debounce left at 0 → should default to 500ms
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer w.Close()

	if w.debouncer.delay != 500*time.Millisecond {
		t.Errorf("expected default debounce 500ms, got %v", w.debouncer.delay)
	}
}

func TestNew_SkipsNativeWatchServices(t *testing.T) {
	dir := t.TempDir()
	apiDir := filepath.Join(dir, "api")
	frontDir := filepath.Join(dir, "frontend")
	_ = os.MkdirAll(apiDir, 0o755)
	_ = os.MkdirAll(frontDir, 0o755)

	// Frontend is in NativeWatch, so it should be skipped.
	// We can't observe that directly without inspecting fsnotify internals,
	// but at minimum New should succeed.
	w, err := New(Config{
		ServicePaths: map[string]string{"api": apiDir, "frontend": frontDir},
		NativeWatch:  map[string]bool{"frontend": true},
		OnRestart:    func(string) {},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer w.Close()
}

func TestNew_NonexistentPathSilentlySkipped(t *testing.T) {
	// addRecursive swallows walk errors, so a nonexistent path is a no-op
	// rather than an error. This documents that behavior.
	w, err := New(Config{
		ServicePaths: map[string]string{"ghost": "/nonexistent/path/xyz/raioz-test"},
		OnRestart:    func(string) {},
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if w != nil {
		w.Close()
	}
}

func TestWatcher_RunDetectsFileWrite(t *testing.T) {
	dir := t.TempDir()
	apiDir := filepath.Join(dir, "api")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var triggered int32
	var wg sync.WaitGroup
	wg.Add(1)
	var once sync.Once

	w, err := New(Config{
		ServicePaths: map[string]string{"api": apiDir},
		Debounce:     20 * time.Millisecond,
		OnRestart: func(name string) {
			if name == "api" {
				atomic.AddInt32(&triggered, 1)
				once.Do(wg.Done)
			}
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	// Give fsnotify a moment to register
	time.Sleep(50 * time.Millisecond)

	// Write a .go file to trigger event
	f := filepath.Join(apiDir, "main.go")
	if err := os.WriteFile(f, []byte("package main"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Wait for debouncer callback with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for restart callback")
	}

	w.Close()
}

func TestWatcher_RunCancelContext(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	w, err := New(Config{
		ServicePaths: map[string]string{"s": dir},
		OnRestart:    func(string) {},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	// Cancel and expect Run to return quickly
	cancel()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Error("Run did not return after context cancel")
	}
}

func TestWatcher_Close(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	w, err := New(Config{
		ServicePaths: map[string]string{"s": dir},
		OnRestart:    func(string) {},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Close without Run should be safe (cancel is nil)
	w.Close()
}

func TestAddRecursive_SkipsExcludedDirs(t *testing.T) {
	dir := t.TempDir()
	// Create excluded and normal subdirs
	_ = os.MkdirAll(filepath.Join(dir, "node_modules", "pkg"), 0o755)
	_ = os.MkdirAll(filepath.Join(dir, "src"), 0o755)
	_ = os.MkdirAll(filepath.Join(dir, ".git"), 0o755)

	w, err := New(Config{
		ServicePaths: map[string]string{"svc": dir},
		OnRestart:    func(string) {},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer w.Close()
	// Success = excluded dirs don't cause errors.
}
