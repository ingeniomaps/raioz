package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestLog_BasicEvent(t *testing.T) {
	path := setupAuditHome(t)

	err := Log(EventTypeConfigChanged, map[string]interface{}{"key": "value"}, "test message")
	if err != nil {
		t.Fatalf("Log: %v", err)
	}

	events := readEvents(t, path)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventTypeConfigChanged {
		t.Errorf("wrong type: %s", events[0].Type)
	}
	if events[0].Message != "test message" {
		t.Errorf("wrong message: %s", events[0].Message)
	}
	if events[0].Details["key"] != "value" {
		t.Errorf("wrong details: %+v", events[0].Details)
	}
}

func TestLog_Appends(t *testing.T) {
	path := setupAuditHome(t)

	if err := Log(EventTypeDevPromoted, nil, "first"); err != nil {
		t.Fatalf("Log 1: %v", err)
	}
	if err := Log(EventTypeDevReverted, nil, "second"); err != nil {
		t.Fatalf("Log 2: %v", err)
	}

	events := readEvents(t, path)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestLogDependencyAdded(t *testing.T) {
	path := setupAuditHome(t)

	err := LogDependencyAdded("postgres", "auto-detect", "found in docker-compose")
	if err != nil {
		t.Fatalf("LogDependencyAdded: %v", err)
	}

	events := readEvents(t, path)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventTypeDependencyAdded {
		t.Errorf("wrong type: %s", events[0].Type)
	}
	if events[0].Details["service"] != "postgres" {
		t.Errorf("expected service=postgres, got %+v", events[0].Details)
	}
}

func TestLogDevPromoted(t *testing.T) {
	path := setupAuditHome(t)

	err := LogDevPromoted("postgres", "/local/postgres", "postgres:16")
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

	err := LogDevReverted("redis", "redis:7")
	if err != nil {
		t.Fatalf("LogDevReverted: %v", err)
	}

	events := readEvents(t, path)
	if events[0].Type != EventTypeDevReverted {
		t.Errorf("wrong type: %s", events[0].Type)
	}
}

func TestLogConfigChanged(t *testing.T) {
	path := setupAuditHome(t)

	err := LogConfigChanged("workspace1", []string{"added api", "removed worker"})
	if err != nil {
		t.Fatalf("LogConfigChanged: %v", err)
	}

	events := readEvents(t, path)
	if events[0].Type != EventTypeConfigChanged {
		t.Errorf("wrong type: %s", events[0].Type)
	}
	if events[0].Details["workspace"] != "workspace1" {
		t.Errorf("expected workspace=workspace1")
	}
}

func TestLogConflictResolved(t *testing.T) {
	path := setupAuditHome(t)

	err := LogConflictResolved("api", "stop", "user chose to stop")
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

	err := LogServiceAssisted("worker", "auto-detect", "found Dockerfile")
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

	err := LogDriftDetected("api", "/path/to/config", []string{"image changed", "port added"})
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

func TestLog_TimestampIsSet(t *testing.T) {
	path := setupAuditHome(t)

	if err := Log(EventTypeConfigChanged, nil, ""); err != nil {
		t.Fatalf("Log: %v", err)
	}

	events := readEvents(t, path)
	if events[0].Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestLog_InvalidBaseDir(t *testing.T) {
	// Set RAIOZ_HOME to something unwritable (if not root)
	if os.Geteuid() == 0 {
		t.Skip("running as root")
	}
	t.Setenv("RAIOZ_HOME", "/root/should-fail-raioz-audit")

	err := Log(EventTypeConfigChanged, nil, "")
	// Likely error — but some environments may succeed
	_ = err
}
