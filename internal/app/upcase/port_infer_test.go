package upcase

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/domain/models"
)

func TestInferServicePort_FromEnv(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".env"), []byte("PORT=9090\n"), 0644)

	svc := models.Service{Source: models.SourceConfig{Path: dir}}
	det := models.DetectResult{Runtime: models.RuntimeGo}

	port := inferServicePort(svc, det)
	if port != 9090 {
		t.Errorf("expected 9090, got %d", port)
	}
}

func TestInferServicePort_RuntimeDefault(t *testing.T) {
	tests := []struct {
		runtime models.Runtime
		port    int
	}{
		{models.RuntimeGo, 8080},
		{models.RuntimeNPM, 3000},
		{models.RuntimePython, 5000},
		{models.RuntimeRust, 8080},
		{models.RuntimePHP, 8000},
		{models.RuntimeJava, 8080},
		{models.RuntimeDotnet, 5000},
		{models.RuntimeRuby, 3000},
		{models.RuntimeElixir, 4000},
		{models.RuntimeDeno, 3000},
		{models.RuntimeBun, 3000},
	}

	for _, tt := range tests {
		t.Run(string(tt.runtime), func(t *testing.T) {
			svc := models.Service{} // no path, no ports
			det := models.DetectResult{Runtime: tt.runtime}
			port := inferServicePort(svc, det)
			if port != tt.port {
				t.Errorf("expected %d, got %d", tt.port, port)
			}
		})
	}
}

func TestParseFirstPort(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"8080", 8080},
		{"3000:8080", 3000},
		{"5432", 5432},
		{"", 0},
		{"abc", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseFirstPort(tt.input)
			if got != tt.expected {
				t.Errorf("parseFirstPort(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}
