package audit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"raioz/internal/logging"
)

// Decodes audit.log into typed events. Returns one slice element per
// JSON line so the test can assert positions (start vs complete).
func readAuditEvents(t *testing.T, path string) []Event {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	var out []Event
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("unmarshal %q: %v", line, err)
		}
		out = append(out, ev)
	}
	return out
}

// pinAuditPath relocates RAIOZ_HOME (audit root) into a tempdir for
// the test's lifetime. Returns the resolved log path.
func pinAuditPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("RAIOZ_HOME", dir)
	t.Setenv(logging.CorrelationIDEnv, "")
	return filepath.Join(dir, auditLogFileName)
}

// TestLogLifecycleStart asserts the canonical shape of a start event:
// type, phase, project, workspace fields, and the human-readable
// message. The correlation ID inheritance is covered separately.
func TestLogLifecycleStart(t *testing.T) {
	path := pinAuditPath(t)

	ctx := logging.WithRequestID(context.Background())
	if err := LogLifecycleStart(ctx, "up", "myproj", "myws"); err != nil {
		t.Fatalf("LogLifecycleStart: %v", err)
	}

	events := readAuditEvents(t, path)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.Type != EventTypeLifecycle {
		t.Errorf("Type = %q, want %q", ev.Type, EventTypeLifecycle)
	}
	if ev.Details["operation"] != "up" {
		t.Errorf("operation = %v, want up", ev.Details["operation"])
	}
	if ev.Details["phase"] != "start" {
		t.Errorf("phase = %v, want start", ev.Details["phase"])
	}
	if ev.Details["project"] != "myproj" {
		t.Errorf("project = %v, want myproj", ev.Details["project"])
	}
	if ev.Details["workspace"] != "myws" {
		t.Errorf("workspace = %v, want myws", ev.Details["workspace"])
	}
	if ev.CorrelationID == "" {
		t.Error("CorrelationID should be populated from ctx")
	}
}

// TestLogLifecycleComplete_Success asserts status + duration + zero
// error on the success path.
func TestLogLifecycleComplete_Success(t *testing.T) {
	path := pinAuditPath(t)

	ctx := logging.WithRequestID(context.Background())
	if err := LogLifecycleComplete(
		ctx, "up", "myproj", "myws", "success", 250*time.Millisecond, nil,
	); err != nil {
		t.Fatalf("LogLifecycleComplete: %v", err)
	}

	ev := readAuditEvents(t, path)[0]
	if ev.Details["status"] != "success" {
		t.Errorf("status = %v, want success", ev.Details["status"])
	}
	if ev.Details["duration_ms"].(float64) != 250 {
		t.Errorf("duration_ms = %v, want 250", ev.Details["duration_ms"])
	}
	if _, hasErr := ev.Details["error"]; hasErr {
		t.Errorf("success path must not carry error field: %v", ev.Details["error"])
	}
}

// TestLogLifecycleComplete_Failure asserts the error message is
// embedded in the event when status=failure.
func TestLogLifecycleComplete_Failure(t *testing.T) {
	path := pinAuditPath(t)

	ctx := logging.WithRequestID(context.Background())
	if err := LogLifecycleComplete(
		ctx, "down", "myproj", "myws", "failure",
		time.Second, errors.New("boom"),
	); err != nil {
		t.Fatalf("LogLifecycleComplete: %v", err)
	}

	ev := readAuditEvents(t, path)[0]
	if ev.Details["status"] != "failure" {
		t.Errorf("status = %v, want failure", ev.Details["status"])
	}
	if got := ev.Details["error"]; got != "boom" {
		t.Errorf("error = %v, want boom", got)
	}
}

// TestLogLifecycle_CorrelationIDInheritsFromEnv exercises the
// parent→sibling propagation path: RAIOZ_CORRELATION_ID set in env
// becomes the correlation ID of subsequent events.
func TestLogLifecycle_CorrelationIDInheritsFromEnv(t *testing.T) {
	path := pinAuditPath(t)

	t.Setenv(logging.CorrelationIDEnv, "abcdef0123456789")
	ctx := logging.WithRequestID(context.Background())
	if err := LogLifecycleStart(ctx, "up", "myproj", ""); err != nil {
		t.Fatalf("LogLifecycleStart: %v", err)
	}

	ev := readAuditEvents(t, path)[0]
	if ev.CorrelationID != "abcdef0123456789" {
		t.Errorf("CorrelationID = %q, want inherited abcdef0123456789", ev.CorrelationID)
	}
}

// TestLogLifecycle_WorkspaceOmittedWhenEmpty documents that empty
// workspace stays out of Details — keeps records compact for the
// common "no workspace declared" case.
func TestLogLifecycle_WorkspaceOmittedWhenEmpty(t *testing.T) {
	path := pinAuditPath(t)

	ctx := logging.WithRequestID(context.Background())
	if err := LogLifecycleStart(ctx, "up", "myproj", ""); err != nil {
		t.Fatalf("LogLifecycleStart: %v", err)
	}

	ev := readAuditEvents(t, path)[0]
	if _, hasWorkspace := ev.Details["workspace"]; hasWorkspace {
		t.Errorf("workspace must be omitted when empty, got %v", ev.Details["workspace"])
	}
}
