package audit

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// setupAuditHome creates a temp RAIOZ_HOME and returns the audit log path.
func setupAuditHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("RAIOZ_HOME", dir)
	return filepath.Join(dir, auditLogFileName)
}

func readEvents(t *testing.T, path string) []Event {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open audit log: %v", err)
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var ev Event
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			t.Fatalf("unmarshal event: %v", err)
		}
		events = append(events, ev)
	}
	return events
}

func TestGetAuditLogPath(t *testing.T) {
	setupAuditHome(t)
	path, err := GetAuditLogPath()
	if err != nil {
		t.Fatalf("GetAuditLogPath: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path")
	}
	if !strings.HasSuffix(path, auditLogFileName) {
		t.Errorf("expected path to end with %s, got %s", auditLogFileName, path)
	}
}

func TestLogDevPromoted(t *testing.T) {
	path := setupAuditHome(t)

	err := LogDevPromoted(context.Background(), "postgres", "/local/postgres", "postgres:16")
	if err != nil {
		t.Fatalf("LogDevPromoted: %v", err)
	}

	events := readEvents(t, path)
	if events[0].Type != EventTypeDevPromoted {
		t.Errorf("wrong type: %s", events[0].Type)
	}
	if events[0].Details["dependency"] != "postgres" {
		t.Errorf("expected dependency=postgres")
	}
}

func TestLogDevReverted(t *testing.T) {
	path := setupAuditHome(t)

	err := LogDevReverted(context.Background(), "redis", "redis:7")
	if err != nil {
		t.Fatalf("LogDevReverted: %v", err)
	}

	events := readEvents(t, path)
	if events[0].Type != EventTypeDevReverted {
		t.Errorf("wrong type: %s", events[0].Type)
	}
}

func TestLogConflictResolved(t *testing.T) {
	path := setupAuditHome(t)

	err := LogConflictResolved(context.Background(), "api", "stop", "user chose to stop")
	if err != nil {
		t.Fatalf("LogConflictResolved: %v", err)
	}

	events := readEvents(t, path)
	if events[0].Type != EventTypeConflictResolved {
		t.Errorf("wrong type: %s", events[0].Type)
	}
}

func TestLogServiceAssisted(t *testing.T) {
	path := setupAuditHome(t)

	err := LogServiceAssisted(context.Background(), "worker", "auto-detect", "found Dockerfile")
	if err != nil {
		t.Fatalf("LogServiceAssisted: %v", err)
	}

	events := readEvents(t, path)
	if events[0].Type != EventTypeServiceAssisted {
		t.Errorf("wrong type: %s", events[0].Type)
	}
}

func TestLogDriftDetected(t *testing.T) {
	path := setupAuditHome(t)

	err := LogDriftDetected(context.Background(), "api", "/path/to/config", []string{"image changed", "port added"})
	if err != nil {
		t.Fatalf("LogDriftDetected: %v", err)
	}

	events := readEvents(t, path)
	if events[0].Type != EventTypeDriftDetected {
		t.Errorf("wrong type: %s", events[0].Type)
	}
	if events[0].Details["count"] != float64(2) {
		t.Errorf("expected count=2, got %v", events[0].Details["count"])
	}
}

func TestRotation_NoOpUnderCap(t *testing.T) {
	path := setupAuditHome(t)
	if err := os.WriteFile(path, []byte("seed-event\n"), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rotateIfOverCap(path, maxAuditSize)

	if _, err := os.Stat(path + ".1"); !os.IsNotExist(err) {
		t.Errorf("expected no rotation when file is under cap, but .1 exists (err=%v)", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat path: %v", err)
	}
	if info.Size() == 0 {
		t.Error("expected original file to remain untouched")
	}
}

func TestRotation_TriggersWhenOverCap(t *testing.T) {
	path := setupAuditHome(t)
	// Write maxAuditSize + 1 bytes to push the file past the cap.
	if err := os.WriteFile(path, bytes.Repeat([]byte{'x'}, int(maxAuditSize)+1), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rotateIfOverCap(path, maxAuditSize)

	// Original path must now be absent (it was renamed to .1).
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected original path to be gone after rotation, err=%v", err)
	}
	rotated := path + ".1"
	info, err := os.Stat(rotated)
	if err != nil {
		t.Fatalf("expected rotated file at %s, got %v", rotated, err)
	}
	if info.Size() == 0 {
		t.Errorf("rotated file should carry the original contents")
	}
}

// Regression: N concurrent goroutines × M events each must
// produce N×M valid JSONL lines across audit.log + any rotation
// backups. Before the sidecar-flock fix, two writers crossing the
// size cap simultaneously could both rename to `.1`, trashing one
// write's events; concurrent appends near the pipe-buffer threshold
// could also interleave bytes. The flock around rotation+append
// closes both windows.
func TestLogWithContext_ConcurrentWritersNoLoss(t *testing.T) {
	path := setupAuditHome(t)
	ctx := context.Background()

	const goroutines = 8
	const eventsPerGoroutine = 200

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(gID int) {
			defer wg.Done()
			for i := 0; i < eventsPerGoroutine; i++ {
				_ = LogWithContext(ctx, EventTypeLifecycle, map[string]any{
					"goroutine": gID,
					"seq":       i,
				}, "concurrent write")
			}
		}(g)
	}
	wg.Wait()

	// Sum events across the live log + any rotation backups.
	total := 0
	for _, suffix := range []string{"", ".1", ".2", ".3"} {
		p := path + suffix
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		// Audit events with large details fields can exceed 64 KiB
		// briefly during MarshalIndent; raise the scanner cap so the
		// test counts correctly even under that pressure.
		sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
		for sc.Scan() {
			var ev map[string]any
			if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
				t.Errorf("malformed JSON line in %s: %v\nline=%q",
					p, err, sc.Text())
				continue
			}
			total++
		}
		f.Close()
	}
	want := goroutines * eventsPerGoroutine
	if total != want {
		t.Errorf("event count mismatch: got %d, want %d (concurrent writers lost events)",
			total, want)
	}
}
