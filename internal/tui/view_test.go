package tui

import (
	"strings"
	"testing"
)

func TestView_TooSmallTerminal(t *testing.T) {
	m := testModel()
	m.width = 30
	m.height = 10

	output := m.View()
	if !strings.Contains(output, "too small") {
		t.Error("expected 'too small' message for tiny terminal")
	}
}

func TestView_NormalRender(t *testing.T) {
	m := testModel()
	m.width = 100
	m.height = 30

	output := m.View()

	if !strings.Contains(output, "test-app") {
		t.Error("expected project name in view")
	}
	if !strings.Contains(output, "SERVICE") {
		t.Error("expected table header in view")
	}
	if !strings.Contains(output, "api") {
		t.Error("expected 'api' service in view")
	}
	if !strings.Contains(output, "frontend") {
		t.Error("expected 'frontend' service in view")
	}
	if !strings.Contains(output, "[r]") || !strings.Contains(output, "[q]") {
		t.Error("expected keyboard shortcuts in footer")
	}
}

func TestView_WithWorkspace(t *testing.T) {
	m := testModel()
	m.width = 100
	m.height = 30

	output := m.View()
	if !strings.Contains(output, "acme / test-app") {
		t.Error("expected 'acme / test-app' header with workspace")
	}
}

func TestView_Quitting(t *testing.T) {
	m := testModel()
	m.quitting = true

	output := m.View()
	if !strings.Contains(output, "still running") {
		t.Error("expected quit message about services still running")
	}
}

func TestView_LogsPanel(t *testing.T) {
	m := testModel()
	m.width = 100
	m.height = 30
	m.addLogLine("api", "[14:00:00] GET /health 200")

	output := m.View()
	if !strings.Contains(output, "Logs (api)") {
		t.Error("expected logs panel title")
	}
	if !strings.Contains(output, "GET /health") {
		t.Error("expected log content in view")
	}
}

func TestFormatActionResult(t *testing.T) {
	tests := []struct {
		msg  ActionResultMsg
		want string
	}{
		{ActionResultMsg{Service: "api", Action: "restart", Err: nil}, "restart api: done"},
		{ActionResultMsg{Service: "api", Action: "stop", Err: nil}, "stop api: done"},
	}
	for _, tt := range tests {
		got := formatActionResult(tt.msg)
		if got != tt.want {
			t.Errorf("formatActionResult = %q, want %q", got, tt.want)
		}
	}
}
