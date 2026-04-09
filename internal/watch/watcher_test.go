package watch

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestDebouncer_SingleTrigger(t *testing.T) {
	var mu sync.Mutex
	var triggered []string

	d := NewDebouncer(50*time.Millisecond, func(name string) {
		mu.Lock()
		triggered = append(triggered, name)
		mu.Unlock()
	})
	defer d.Stop()

	d.Trigger("api")
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(triggered) != 1 || triggered[0] != "api" {
		t.Errorf("expected [api], got %v", triggered)
	}
}

func TestDebouncer_MultipleTriggersSameService(t *testing.T) {
	var mu sync.Mutex
	var count int

	d := NewDebouncer(50*time.Millisecond, func(name string) {
		mu.Lock()
		count++
		mu.Unlock()
	})
	defer d.Stop()

	// Rapid triggers should be debounced to one
	d.Trigger("api")
	d.Trigger("api")
	d.Trigger("api")
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if count != 1 {
		t.Errorf("expected 1 trigger, got %d", count)
	}
}

func TestDebouncer_DifferentServices(t *testing.T) {
	var mu sync.Mutex
	var triggered []string

	d := NewDebouncer(50*time.Millisecond, func(name string) {
		mu.Lock()
		triggered = append(triggered, name)
		mu.Unlock()
	})
	defer d.Stop()

	d.Trigger("api")
	d.Trigger("frontend")
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(triggered) != 2 {
		t.Errorf("expected 2 triggers, got %d", len(triggered))
	}
}

func TestResolver_Basic(t *testing.T) {
	dir := t.TempDir()
	apiDir := filepath.Join(dir, "api")
	frontendDir := filepath.Join(dir, "frontend")
	os.MkdirAll(apiDir, 0755)
	os.MkdirAll(frontendDir, 0755)

	r := NewResolver(map[string]string{
		"api":      apiDir,
		"frontend": frontendDir,
	})

	if name := r.Resolve(filepath.Join(apiDir, "main.go")); name != "api" {
		t.Errorf("expected 'api', got '%s'", name)
	}
	if name := r.Resolve(filepath.Join(frontendDir, "src", "App.tsx")); name != "frontend" {
		t.Errorf("expected 'frontend', got '%s'", name)
	}
	if name := r.Resolve("/some/other/path"); name != "" {
		t.Errorf("expected empty, got '%s'", name)
	}
}

func TestResolver_NestedPaths(t *testing.T) {
	dir := t.TempDir()
	svcDir := filepath.Join(dir, "services", "api")
	os.MkdirAll(svcDir, 0755)

	r := NewResolver(map[string]string{
		"api": svcDir,
	})

	if name := r.Resolve(filepath.Join(svcDir, "internal", "handler.go")); name != "api" {
		t.Errorf("expected 'api', got '%s'", name)
	}
}

func TestIsRelevantEvent(t *testing.T) {
	tests := []struct {
		name string
		event fsnotify.Event
		want  bool
	}{
		{"write go file", fsnotify.Event{Name: "main.go", Op: fsnotify.Write}, true},
		{"create file", fsnotify.Event{Name: "new.ts", Op: fsnotify.Create}, true},
		{"rename", fsnotify.Event{Name: "old.go", Op: fsnotify.Rename}, false},
		{"swap file", fsnotify.Event{Name: "file.swp", Op: fsnotify.Write}, false},
		{"hidden file", fsnotify.Event{Name: ".hidden", Op: fsnotify.Write}, false},
		{".env file", fsnotify.Event{Name: ".env", Op: fsnotify.Write}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRelevantEvent(tt.event); got != tt.want {
				t.Errorf("isRelevantEvent(%v) = %v, want %v", tt.event, got, tt.want)
			}
		})
	}
}
