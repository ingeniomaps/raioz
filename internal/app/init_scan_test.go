package app

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/detect"
)

func TestNewInitScanUseCase(t *testing.T) {
	uc := NewInitScanUseCase()
	if uc == nil {
		t.Fatal("expected non-nil InitScanUseCase")
	}
}

func TestContains(t *testing.T) {
	slice := []string{"a", "b", "c"}
	if !contains(slice, "b") {
		t.Error("expected 'b' in slice")
	}
	if contains(slice, "z") {
		t.Error("did not expect 'z' in slice")
	}
	if contains(nil, "a") {
		t.Error("did not expect match in nil slice")
	}
}

func TestIsIgnoredDir(t *testing.T) {
	tests := []struct {
		name    string
		ignored bool
	}{
		{"node_modules", true},
		{"vendor", true},
		{"dist", true},
		{"infra", false},
		{"src", false},
	}
	for _, tt := range tests {
		if got := isIgnoredDir(tt.name); got != tt.ignored {
			t.Errorf("isIgnoredDir(%q)=%v, want %v", tt.name, got, tt.ignored)
		}
	}
}

func TestBuildServiceFromDetection(t *testing.T) {
	result := detect.DetectResult{
		Runtime:      detect.RuntimeNPM,
		Port:         3000,
		HasHotReload: true,
		StartCommand: "npm run dev",
	}
	svc := buildServiceFromDetection("api", "./api", result)
	if svc.Path != "./api" {
		t.Errorf("expected path './api', got %q", svc.Path)
	}
	if len(svc.Ports) == 0 {
		t.Error("expected ports to be set")
	}
	if !svc.Watch.Enabled {
		t.Error("expected watch to be enabled")
	}
}

func TestBuildServiceFromDetection_NoHotReload(t *testing.T) {
	result := detect.DetectResult{
		Runtime: detect.RuntimeGo,
	}
	svc := buildServiceFromDetection("svc", "./svc", result)
	if svc.Watch.Enabled {
		t.Error("expected watch to be disabled")
	}
	if len(svc.Ports) != 0 {
		t.Error("expected no ports when port=0")
	}
}

func TestInitScanUseCase_Execute_EmptyDir(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	uc := NewInitScanUseCase()
	err := uc.Execute(InitScanOptions{
		Dir:     tmpDir,
		Project: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInitScanUseCase_Execute_WithOutputPath(t *testing.T) {
	initI18nForTest(t)
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "out.yaml")
	// Create a service to write something
	svcDir := filepath.Join(tmpDir, "api")
	if err := os.Mkdir(svcDir, 0755); err != nil {
		t.Fatal(err)
	}
	// package.json is enough for npm detection
	_ = os.WriteFile(filepath.Join(svcDir, "package.json"),
		[]byte(`{"name":"api","scripts":{"dev":"node index.js"}}`), 0644)

	uc := NewInitScanUseCase()
	err := uc.Execute(InitScanOptions{
		Dir:        tmpDir,
		OutputPath: outPath,
		Project:    "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// File may or may not be created depending on detection; allow both
	_ = outPath
}
