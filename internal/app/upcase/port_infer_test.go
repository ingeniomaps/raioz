package upcase

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
	"raioz/internal/detect"
)

func TestInferServicePort_FromEnv(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".env"), []byte("PORT=9090\n"), 0644)

	svc := config.Service{Source: config.SourceConfig{Path: dir}}
	det := detect.DetectResult{Runtime: detect.RuntimeGo}

	port := inferServicePort(svc, det)
	if port != 9090 {
		t.Errorf("expected 9090, got %d", port)
	}
}

func TestInferServicePort_RuntimeDefault(t *testing.T) {
	tests := []struct {
		runtime detect.Runtime
		port    int
	}{
		{detect.RuntimeGo, 8080},
		{detect.RuntimeNPM, 3000},
		{detect.RuntimePython, 5000},
		{detect.RuntimeRust, 8080},
		{detect.RuntimePHP, 8000},
		{detect.RuntimeJava, 8080},
		{detect.RuntimeDotnet, 5000},
		{detect.RuntimeRuby, 3000},
		{detect.RuntimeElixir, 4000},
		{detect.RuntimeDeno, 3000},
		{detect.RuntimeBun, 3000},
	}

	for _, tt := range tests {
		t.Run(string(tt.runtime), func(t *testing.T) {
			svc := config.Service{} // no path, no ports
			det := detect.DetectResult{Runtime: tt.runtime}
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
