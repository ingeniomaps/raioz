package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readMetaAuditEvents decodes the audit log into a slice of raw event
// maps. Used by meta audit tests so they don't depend on internal/audit
// package types.
func readMetaAuditEvents(t *testing.T, path string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	var out []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("unmarshal %q: %v", line, err)
		}
		out = append(out, ev)
	}
	return out
}

// pinMetaAuditHome relocates RAIOZ_HOME to a tempdir and returns the
// audit log path so meta tests can assert what was written.
func pinMetaAuditHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("RAIOZ_HOME", dir)
	return filepath.Join(dir, "audit.log")
}

func detailsOf(ev map[string]any) map[string]any {
	if d, ok := ev["details"].(map[string]any); ok {
		return d
	}
	return nil
}

// TestMetaRunner_UpEmitsLifecycleEvents asserts that meta_up writes a
// start and a complete event with workspace + project = workspace name.
func TestMetaRunner_UpEmitsLifecycleEvents(t *testing.T) {
	logPath := pinMetaAuditHome(t)

	bin := stagePassingBinary(t)
	cfg, _ := makeMetaProjects(t, "api", "ui")
	cfg.Workspace = "audit-ws"

	r := &MetaRunner{Binary: bin}
	_ = r.Up(context.Background(), cfg, nil, nil, MetaUpOptions{})

	events := readMetaAuditEvents(t, logPath)
	var start, complete map[string]any
	for _, ev := range events {
		d := detailsOf(ev)
		if d == nil {
			continue
		}
		if d["operation"] != "meta_up" {
			continue
		}
		switch d["phase"] {
		case "start":
			start = d
		case "complete":
			complete = d
		}
	}
	if start == nil {
		t.Fatalf("expected a meta_up start event; got events=%+v", events)
	}
	if complete == nil {
		t.Fatalf("expected a meta_up complete event; got events=%+v", events)
	}
	if start["workspace"] != "audit-ws" {
		t.Errorf("start workspace = %v, want audit-ws", start["workspace"])
	}
	if complete["status"] != "success" {
		t.Errorf("complete status = %v, want success", complete["status"])
	}
}

// TestMetaRunner_DownFailureEmitsPerSubAuditEvent asserts that issue 021
// is closed — a failing sub during best-effort down leaves an audit
// breadcrumb naming the sub and the error.
func TestMetaRunner_DownFailureEmitsPerSubAuditEvent(t *testing.T) {
	logPath := pinMetaAuditHome(t)

	bin := stageFailingBinary(t)
	cfg, _ := makeMetaProjects(t, "good", "bad")
	cfg.Workspace = "audit-down-ws"

	r := &MetaRunner{Binary: bin}
	_ = r.Down(context.Background(), cfg, nil)

	events := readMetaAuditEvents(t, logPath)
	var perSub []map[string]any
	for _, ev := range events {
		d := detailsOf(ev)
		if d == nil {
			continue
		}
		if op, _ := d["operation"].(string); strings.HasPrefix(op, "meta_sub_") {
			perSub = append(perSub, d)
		}
	}
	if len(perSub) == 0 {
		t.Fatalf("expected meta_sub_* failure events for best-effort down; got %+v", events)
	}
	for _, d := range perSub {
		if d["best_effort"] != true {
			t.Errorf("best_effort flag missing/false: %+v", d)
		}
		if d["sub_project"] == nil {
			t.Errorf("sub_project missing: %+v", d)
		}
	}
}
