package config

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/i18n"
)

// captureStdout reroutes os.Stdout for the duration of fn and returns
// whatever was written. PrintWarning writes to stdout, so we have to
// intercept the fd to assert on the banner.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()
	_ = w.Close()
	<-done
	return buf.String()
}

func writeMinimalJSON(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	content := `{
		"schemaVersion": "1.0",
		"project": {"name": "p", "network": "n"},
		"services": {},
		"infra": {},
		"env": {"useGlobal": true, "files": []}
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestLoadDeps_DeprecationWarningFiresOnce pins ADR-038: the JSON
// deprecation banner emits to stdout once per process even when
// LoadDeps is called multiple times (dependency_assist scans
// sub-projects, migrate yaml re-reads, etc).
func TestLoadDeps_DeprecationWarningFiresOnce(t *testing.T) {
	i18n.Init("en")
	ResetJSONDeprecationWarningForTest()

	dir := t.TempDir()
	first := writeMinimalJSON(t, dir, "first.json")
	second := writeMinimalJSON(t, dir, "second.json")

	out := captureStdout(t, func() {
		if _, _, err := LoadDeps(first); err != nil {
			t.Fatalf("first LoadDeps: %v", err)
		}
		if _, _, err := LoadDeps(second); err != nil {
			t.Fatalf("second LoadDeps: %v", err)
		}
	})

	count := strings.Count(out, ".raioz.json")
	if count != 1 {
		t.Fatalf("deprecation banner should fire exactly once, fired %d times. output:\n%s", count, out)
	}
	if !strings.Contains(out, "raioz migrate yaml") {
		t.Errorf("banner missing migration hint: %q", out)
	}
	if !strings.Contains(out, "ADR-038") {
		t.Errorf("banner missing ADR reference: %q", out)
	}
}

// TestLoadDeps_DeprecationWarningResets makes sure the test seam
// works — if it didn't, every test that hit LoadDeps would have
// to depend on package-init ordering.
func TestLoadDeps_DeprecationWarningResets(t *testing.T) {
	i18n.Init("en")
	ResetJSONDeprecationWarningForTest()
	dir := t.TempDir()
	path := writeMinimalJSON(t, dir, "cfg.json")

	first := captureStdout(t, func() {
		if _, _, err := LoadDeps(path); err != nil {
			t.Fatalf("LoadDeps: %v", err)
		}
	})
	if !strings.Contains(first, ".raioz.json") {
		t.Fatalf("first call should fire banner, got %q", first)
	}

	second := captureStdout(t, func() {
		if _, _, err := LoadDeps(path); err != nil {
			t.Fatalf("LoadDeps: %v", err)
		}
	})
	if second != "" {
		t.Fatalf("second call should be silent, got %q", second)
	}

	ResetJSONDeprecationWarningForTest()
	third := captureStdout(t, func() {
		if _, _, err := LoadDeps(path); err != nil {
			t.Fatalf("LoadDeps: %v", err)
		}
	})
	if !strings.Contains(third, ".raioz.json") {
		t.Fatalf("post-reset call should fire banner, got %q", third)
	}
}
