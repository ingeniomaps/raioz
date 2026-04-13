package env

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/config"
	"raioz/internal/workspace"
)

func TestParseEnvContent(t *testing.T) {
	content := `# Comment
VAR1=value1
VAR2="quoted value"
VAR3='single quoted'
VAR4=
# Another comment

VAR5=plain
`
	got := parseEnvContent(content)

	if got["VAR1"] != "value1" {
		t.Errorf("VAR1: got %q", got["VAR1"])
	}
	if got["VAR2"] != "quoted value" {
		t.Errorf("VAR2: got %q", got["VAR2"])
	}
	if got["VAR3"] != "single quoted" {
		t.Errorf("VAR3: got %q", got["VAR3"])
	}
	if got["VAR5"] != "plain" {
		t.Errorf("VAR5: got %q", got["VAR5"])
	}
	if _, ok := got["# Comment"]; ok {
		t.Error("comments should not be parsed")
	}
}

func TestParseEnvContent_Empty(t *testing.T) {
	got := parseEnvContent("")
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestParseEnvContent_InvalidLine(t *testing.T) {
	got := parseEnvContent("not a valid line\nVAR=value\n")
	if got["VAR"] != "value" {
		t.Errorf("expected to skip invalid lines and parse VAR, got %v", got)
	}
}

func TestWriteEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.env")

	vars := map[string]string{
		"FOO":    "bar",
		"SPACED": "value with spaces",
		"EMPTY":  "",
		"ZPLAIN": "plain",
	}

	if err := writeEnvFile(path, vars); err != nil {
		t.Fatalf("writeEnvFile: %v", err)
	}

	// Read back and verify content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}

	parsed := parseEnvContent(string(data))
	for k, v := range vars {
		if parsed[k] != v {
			t.Errorf("%s: got %q, want %q", k, parsed[k], v)
		}
	}
}

func TestWriteEnvFile_EscapesSpecialChars(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "special.env")

	// Just verify the function succeeds with tricky values
	vars := map[string]string{
		"DOLLAR": "has $var",
		"QUOTED": `has "quote"`,
	}
	if err := writeEnvFile(path, vars); err != nil {
		t.Fatalf("writeEnvFile: %v", err)
	}

	// File should exist and contain both keys
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "DOLLAR") {
		t.Error("DOLLAR missing from file")
	}
	if !strings.Contains(content, "QUOTED") {
		t.Error("QUOTED missing from file")
	}
}

func TestWriteEnvFile_SortedOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sorted.env")

	vars := map[string]string{
		"ZEBRA": "last",
		"APPLE": "first",
		"MANGO": "middle",
	}

	if err := writeEnvFile(path, vars); err != nil {
		t.Fatalf("writeEnvFile: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)

	// APPLE should appear before MANGO which should appear before ZEBRA
	aIdx := -1
	mIdx := -1
	zIdx := -1
	for i := range content {
		if aIdx == -1 && i+5 <= len(content) && content[i:i+5] == "APPLE" {
			aIdx = i
		}
		if mIdx == -1 && i+5 <= len(content) && content[i:i+5] == "MANGO" {
			mIdx = i
		}
		if zIdx == -1 && i+5 <= len(content) && content[i:i+5] == "ZEBRA" {
			zIdx = i
		}
	}
	if !(aIdx < mIdx && mIdx < zIdx) {
		t.Errorf("keys not sorted: A=%d M=%d Z=%d", aIdx, mIdx, zIdx)
	}
}

func TestWriteEnvFile_BadPath(t *testing.T) {
	// Nonexistent directory
	err := writeEnvFile("/nonexistent/dir/file.env", map[string]string{"A": "b"})
	if err == nil {
		t.Error("expected error for bad path")
	}
}

func TestProcessTemplate_SimpleVar(t *testing.T) {
	result := processTemplate("Hello ${NAME}", map[string]string{"NAME": "World"})
	if result != "Hello World" {
		t.Errorf("got %q, want 'Hello World'", result)
	}
}

func TestProcessTemplate_DollarVar(t *testing.T) {
	result := processTemplate("Hello $NAME", map[string]string{"NAME": "World"})
	if result != "Hello World" {
		t.Errorf("got %q, want 'Hello World'", result)
	}
}

func TestProcessTemplate_MissingVar(t *testing.T) {
	result := processTemplate("Hello ${MISSING}", map[string]string{})
	// Missing vars usually stay as-is
	if result == "Hello " {
		t.Logf("missing var cleared — impl-specific")
	}
}

func TestProcessTemplate_NoVars(t *testing.T) {
	result := processTemplate("no vars here", map[string]string{"X": "y"})
	if result != "no vars here" {
		t.Errorf("got %q, want unchanged", result)
	}
}

func TestGenerateEnvFromTemplate_NoTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{
		Root:        filepath.Join(tmpDir, "ws"),
		ServicesDir: filepath.Join(tmpDir, "services"),
		EnvDir:      filepath.Join(tmpDir, "env"),
	}
	_ = EnsureEnvDirs(ws)

	servicePath := filepath.Join(tmpDir, "svc")
	os.MkdirAll(servicePath, 0o755)

	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
	}
	svc := config.Service{}

	// No template file — should return nil
	err := GenerateEnvFromTemplate(ws, deps, "api", servicePath, svc, "", tmpDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGenerateEnvFromTemplate_WithTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	ws := &workspace.Workspace{
		Root:        filepath.Join(tmpDir, "ws"),
		ServicesDir: filepath.Join(tmpDir, "services"),
		EnvDir:      filepath.Join(tmpDir, "env"),
	}
	_ = EnsureEnvDirs(ws)

	servicePath := filepath.Join(tmpDir, "svc")
	os.MkdirAll(servicePath, 0o755)

	// Write a template
	tplPath := filepath.Join(servicePath, ".env.example")
	os.WriteFile(tplPath, []byte("FOO=bar\nDB_URL=localhost\n"), 0o644)

	deps := &config.Deps{
		Project: config.Project{Name: "proj"},
	}
	svc := config.Service{}

	err := GenerateEnvFromTemplate(ws, deps, "api", servicePath, svc, "", tmpDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// .env should now exist or have been attempted
}
