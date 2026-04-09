// Package watch monitors service directories for file changes and triggers
// restarts for services that don't have their own hot-reload.
package watch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// excludedDirs are directories that should never be watched.
var excludedDirs = map[string]bool{
	"node_modules": true, ".git": true, "__pycache__": true,
	".pytest_cache": true, "vendor": true, "dist": true,
	"build": true, ".next": true, ".nuxt": true,
	"target": true, "bin": true, "obj": true,
	".DS_Store": true, ".idea": true, ".vscode": true,
}

// excludedExts are file extensions that should be ignored.
var excludedExts = map[string]bool{
	".swp": true, ".swo": true, ".tmp": true,
}

// RestartFunc is called when a service needs to be restarted.
type RestartFunc func(serviceName string)

// Watcher monitors file changes and triggers service restarts.
type Watcher struct {
	fsWatcher *fsnotify.Watcher
	resolver  *Resolver
	debouncer *Debouncer
	cancel    context.CancelFunc
}

// Config holds watcher configuration.
type Config struct {
	// ServicePaths maps service name → directory path to watch
	ServicePaths map[string]string
	// NativeWatch lists services that have their own hot-reload (skip these)
	NativeWatch map[string]bool
	// Debounce delay between file change and restart
	Debounce time.Duration
	// OnRestart is called when a service needs restarting
	OnRestart RestartFunc
}

// New creates and starts a new Watcher.
func New(cfg Config) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	if cfg.Debounce == 0 {
		cfg.Debounce = 500 * time.Millisecond
	}

	resolver := NewResolver(cfg.ServicePaths)
	debouncer := NewDebouncer(cfg.Debounce, cfg.OnRestart)

	w := &Watcher{
		fsWatcher: fsw,
		resolver:  resolver,
		debouncer: debouncer,
	}

	// Add watches for each service directory (excluding native hot-reload services)
	for name, path := range cfg.ServicePaths {
		if cfg.NativeWatch[name] {
			continue // Service handles its own hot-reload
		}
		if err := w.addRecursive(path); err != nil {
			fsw.Close()
			return nil, fmt.Errorf("failed to watch %s (%s): %w", name, path, err)
		}
	}

	return w, nil
}

// Run starts the event loop. Blocks until context is cancelled.
func (w *Watcher) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			if !isRelevantEvent(event) {
				continue
			}
			service := w.resolver.Resolve(event.Name)
			if service != "" {
				w.debouncer.Trigger(service)
			}
		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "[watch] error: %v\n", err)
		}
	}
}

// Close stops the watcher and releases resources.
func (w *Watcher) Close() {
	if w.cancel != nil {
		w.cancel()
	}
	w.debouncer.Stop()
	w.fsWatcher.Close()
}

// addRecursive adds a directory and all non-excluded subdirectories to the watcher.
func (w *Watcher) addRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors (permission denied, etc.)
		}
		if !d.IsDir() {
			return nil
		}
		if excludedDirs[d.Name()] {
			return filepath.SkipDir
		}
		return w.fsWatcher.Add(path)
	})
}

// isRelevantEvent filters out noise events.
func isRelevantEvent(event fsnotify.Event) bool {
	// Only care about writes and creates
	if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
		return false
	}
	// Skip excluded extensions
	ext := strings.ToLower(filepath.Ext(event.Name))
	if excludedExts[ext] {
		return false
	}
	// Skip hidden files
	base := filepath.Base(event.Name)
	if strings.HasPrefix(base, ".") && base != ".env" {
		return false
	}
	return true
}
