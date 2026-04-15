package tui

import (
	"testing"
)

func testModel() Model {
	return New(Config{
		Project:   "test-app",
		Workspace: "acme",
		Services: []ServiceRow{
			{Name: "api", Runtime: "go", Status: "running"},
			{Name: "frontend", Runtime: "npm", Status: "running"},
			{Name: "postgres", Runtime: "image", Status: "running"},
		},
	})
}

func TestNew(t *testing.T) {
	m := testModel()
	if m.config.Project != "test-app" {
		t.Errorf("expected project 'test-app', got '%s'", m.config.Project)
	}
	if len(m.services) != 3 {
		t.Errorf("expected 3 services, got %d", len(m.services))
	}
	if m.selected != 0 {
		t.Errorf("expected selected 0, got %d", m.selected)
	}
}

func TestSelectedService(t *testing.T) {
	m := testModel()
	if name := m.SelectedService(); name != "api" {
		t.Errorf("expected 'api', got '%s'", name)
	}
}

func TestSelectNavigation(t *testing.T) {
	m := testModel()

	m.selectNext()
	if m.SelectedService() != "frontend" {
		t.Errorf("expected 'frontend' after next, got '%s'", m.SelectedService())
	}

	m.selectNext()
	if m.SelectedService() != "postgres" {
		t.Errorf("expected 'postgres' after next, got '%s'", m.SelectedService())
	}

	// Should not go past last
	m.selectNext()
	if m.SelectedService() != "postgres" {
		t.Error("should not go past last service")
	}

	m.selectPrev()
	if m.SelectedService() != "frontend" {
		t.Errorf("expected 'frontend' after prev, got '%s'", m.SelectedService())
	}

	// Should not go before first
	m.selectPrev()
	m.selectPrev()
	if m.SelectedService() != "api" {
		t.Error("should not go before first service")
	}
}

func TestAddLogLine(t *testing.T) {
	m := testModel()

	m.addLogLine("api", "line 1")
	m.addLogLine("api", "line 2")

	if len(m.logs["api"]) != 2 {
		t.Errorf("expected 2 log lines, got %d", len(m.logs["api"]))
	}
}

func TestAddLogLine_RingBuffer(t *testing.T) {
	m := testModel()

	// Add more than maxLogLines
	for i := 0; i < maxLogLines+100; i++ {
		m.addLogLine("api", "line")
	}

	if len(m.logs["api"]) != maxLogLines {
		t.Errorf("expected %d log lines (ring buffer), got %d", maxLogLines, len(m.logs["api"]))
	}
}

func TestUpdateStats(t *testing.T) {
	m := testModel()

	m.updateStats(map[string]ServiceStats{
		"api":      {CPU: "5.2%", Memory: "128MB", Status: "healthy"},
		"postgres": {CPU: "1.0%", Memory: "50MB"},
	})

	if m.services[0].CPU != "5.2%" {
		t.Errorf("expected CPU '5.2%%', got '%s'", m.services[0].CPU)
	}
	if m.services[0].Status != "healthy" {
		t.Errorf("expected status 'healthy', got '%s'", m.services[0].Status)
	}
	if m.services[2].CPU != "1.0%" {
		t.Errorf("expected postgres CPU '1.0%%', got '%s'", m.services[2].CPU)
	}
	// Status should not be overwritten if empty
	if m.services[2].Status != "running" {
		t.Errorf("expected postgres status 'running' (unchanged), got '%s'", m.services[2].Status)
	}
}

func TestStatusIcon(t *testing.T) {
	// Just verify they don't panic and return non-empty
	icons := []string{
		statusIcon("running"),
		statusIcon("healthy"),
		statusIcon("stopped"),
		statusIcon("unknown"),
	}
	for _, icon := range icons {
		if icon == "" {
			t.Error("expected non-empty status icon")
		}
	}
}
