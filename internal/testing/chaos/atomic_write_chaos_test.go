// Package chaos hosts fault-injection / concurrency stress tests
// that complement the per-package unit tests. They exercise the
// concurrency-sensitive surfaces (state writes, audit rotation, lock
// contention) under load that the regular suites don't cover.
//
// Issue 046 — these tests are the "did the v0.8.x concurrency
// hardening actually hold under stress?" answer. Run via `go test
// -race ./internal/testing/chaos/...` (~few seconds total).
package chaos

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"raioz/internal/fsutil"
)

// 64 goroutines × 250 writes each = 16 000 atomic writes against the
// same target. After the storm, the file must be parseable JSON
// (i.e. the last write won cleanly with no torn output). Without
// fsutil.WriteFileAtomic (issue 034 fix) this would intermittently
// leave a zero-byte or truncated file.
func TestAtomicWrite_ConcurrentWritersAlwaysParseable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	const goroutines = 64
	const writesPer = 250

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(gID int) {
			defer wg.Done()
			for i := 0; i < writesPer; i++ {
				payload := map[string]any{"g": gID, "i": i}
				data, _ := json.Marshal(payload)
				if err := fsutil.WriteFileAtomic(path, data, 0o600); err != nil {
					t.Errorf("WriteFileAtomic: %v", err)
					return
				}
			}
		}(g)
	}
	wg.Wait()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after chaos: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("file is zero bytes after concurrent atomic writes — race not fixed")
	}
	var parsed map[string]any
	if err := json.Unmarshal(got, &parsed); err != nil {
		t.Fatalf("file content not parseable JSON after chaos: %v\ncontent=%q",
			err, got)
	}
	// Final file should be SOMEONE's last write — a valid JSON object
	// with the expected shape. We don't care which goroutine won.
	if _, ok := parsed["g"]; !ok {
		t.Errorf("parsed JSON missing expected key 'g': %v", parsed)
	}
}
