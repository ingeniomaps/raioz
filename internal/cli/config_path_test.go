package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveConfigPathExplicit(t *testing.T) {
	got := ResolveConfigPath("/tmp/custom.yaml")
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
}

func TestResolveConfigPathRelative(t *testing.T) {
	got := ResolveConfigPath("custom.yaml")
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path from relative, got %q", got)
	}
	if filepath.Base(got) != "custom.yaml" {
		t.Errorf("base = %q, want custom.yaml", filepath.Base(got))
	}
}

func TestResolveConfigPathAutoDetectEmpty(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	got := ResolveConfigPath("")
	if got != AutoDetectMarker {
		t.Errorf("ResolveConfigPath() = %q, want %q", got, AutoDetectMarker)
	}
}

func TestResolveConfigPathFindsYAML(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "raioz.yaml")
	if err := os.WriteFile(yamlPath, []byte("project: test\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	got := ResolveConfigPath("")
	if got != "raioz.yaml" {
		t.Errorf("ResolveConfigPath() = %q, want raioz.yaml", got)
	}
}

func TestResolveConfigPathFindsYML(t *testing.T) {
	dir := t.TempDir()
	ymlPath := filepath.Join(dir, "raioz.yml")
	if err := os.WriteFile(ymlPath, []byte("project: test\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	got := ResolveConfigPath("")
	if got != "raioz.yml" {
		t.Errorf("ResolveConfigPath() = %q, want raioz.yml", got)
	}
}

func TestResolveConfigPathFindsJSON(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, ".raioz.json")
	if err := os.WriteFile(jsonPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	got := ResolveConfigPath("")
	if got != ".raioz.json" {
		t.Errorf("ResolveConfigPath() = %q, want .raioz.json", got)
	}
}

func TestResolveConfigPathYAMLPriorityOverJSON(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"raioz.yaml", "raioz.yml", ".raioz.json"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	got := ResolveConfigPath("")
	if got != "raioz.yaml" {
		t.Errorf("priority: got %q, want raioz.yaml", got)
	}
}

func TestIsAutoDetect(t *testing.T) {
	if !IsAutoDetect(AutoDetectMarker) {
		t.Errorf("IsAutoDetect(%q) = false, want true", AutoDetectMarker)
	}
	if IsAutoDetect("") {
		t.Error("IsAutoDetect(\"\") = true, want false")
	}
	if IsAutoDetect("raioz.yaml") {
		t.Error("IsAutoDetect(\"raioz.yaml\") = true, want false")
	}
}
