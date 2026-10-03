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

// `-p <name>` with no config in the cwd and no such active project is an
// error, never a silent "nothing to do".
func TestResolveProjectConfigPath(t *testing.T) {
	t.Setenv("RAIOZ_HOME", t.TempDir())
	empty := t.TempDir()
	t.Chdir(empty)

	if _, err := ResolveProjectConfigPath("", "ghost"); err == nil {
		t.Error("expected an error for an inactive project with no config in the cwd")
	}

	got, err := ResolveProjectConfigPath("", "")
	if err != nil || got != AutoDetectMarker {
		t.Errorf("no project, no file: got %q err=%v, want the auto-detect marker", got, err)
	}

	if err := os.WriteFile("raioz.yaml", []byte("project: here\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = ResolveProjectConfigPath("", "ghost")
	if err != nil || got != "raioz.yaml" {
		t.Errorf("config in the cwd: got %q err=%v, want raioz.yaml", got, err)
	}

	explicit := filepath.Join(empty, "other.yaml")
	got, err = ResolveProjectConfigPath(explicit, "ghost")
	if err != nil || got != explicit {
		t.Errorf("explicit file: got %q err=%v, want %q", got, err, explicit)
	}
}
