package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AuditYAMLStrict treats a valid yaml as success.
func TestAuditYAMLStrict_HappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raioz.yaml")
	body := "project: ok\nservices:\n  api:\n    path: .\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AuditYAMLStrict(path); err != nil {
		t.Errorf("happy path must pass, got %v", err)
	}
}

// H3 (image pinning) — `:latest` triggers the gate under audit.
func TestAuditYAMLStrict_LatestTagFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raioz.yaml")
	body := "project: tagcheck\n" +
		"services:\n  api:\n    path: .\n" +
		"dependencies:\n  redis:\n    image: redis:latest\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	err := AuditYAMLStrict(path)
	if err == nil {
		t.Fatal("expected H3 image-pinning failure for redis:latest")
	}
	if !strings.Contains(err.Error(), "image pinning") {
		t.Errorf("error should call out the failing gate, got %v", err)
	}
}

// H3 — un-tagged image also trips.
func TestAuditYAMLStrict_UntaggedImageFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raioz.yaml")
	body := "project: tagcheck\n" +
		"services:\n  api:\n    path: .\n" +
		"dependencies:\n  redis:\n    image: redis\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	err := AuditYAMLStrict(path)
	if err == nil {
		t.Fatal("expected H3 failure for un-tagged redis image")
	}
}
