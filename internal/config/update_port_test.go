package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdatePortLines(t *testing.T) {
	servicesYAML := []string{
		"project: demo",
		"services:",
		"  api:",
		"    path: ./api",
		"    port: 3000",
		"  web:",
		"    path: ./web",
		"    port: 8080",
		"dependencies:",
		"  postgres:",
		"    image: postgres:16",
		"    publish: 5432",
	}

	tests := []struct {
		name    string
		key     string
		kind    string
		oldPort int
		newPort int
		want    string
		wantNil bool
	}{
		{
			name:    "service port replaced",
			key:     "api",
			kind:    "service",
			oldPort: 3000,
			newPort: 3001,
			want:    "    port: 3001",
		},
		{
			name:    "second service port replaced without disturbing first",
			key:     "web",
			kind:    "service",
			oldPort: 8080,
			newPort: 9090,
			want:    "    port: 9090",
		},
		{
			name:    "dep publish replaced",
			key:     "postgres",
			kind:    "dep",
			oldPort: 5432,
			newPort: 5433,
			want:    "    publish: 5433",
		},
		{
			name:    "wrong port returns nil (no change)",
			key:     "api",
			kind:    "service",
			oldPort: 9999,
			newPort: 1111,
			wantNil: true,
		},
		{
			name:    "missing key returns nil",
			key:     "ghost",
			kind:    "service",
			oldPort: 3000,
			newPort: 3001,
			wantNil: true,
		},
		{
			name:    "unknown kind returns nil",
			key:     "api",
			kind:    "frobnicator",
			oldPort: 3000,
			newPort: 3001,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := append([]string(nil), servicesYAML...)
			got := updatePortLines(input, tt.key, tt.kind, tt.oldPort, tt.newPort)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected updated lines, got nil")
			}
			joined := strings.Join(got, "\n")
			if !strings.Contains(joined, tt.want) {
				t.Errorf("output missing %q. Got:\n%s", tt.want, joined)
			}
		})
	}
}

func TestUpdatePort_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raioz.yaml")
	original := `project: demo
services:
  api:
    path: ./api
    port: 3000
`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	if err := UpdatePort(path, "api", "service", 3000, 3001); err != nil {
		t.Fatalf("UpdatePort: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "port: 3001") {
		t.Errorf("file not updated:\n%s", got)
	}
}

func TestUpdatePort_MissingFile(t *testing.T) {
	err := UpdatePort("/nonexistent/raioz.yaml", "api", "service", 3000, 3001)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestUpdatePort_NoChangeNoWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raioz.yaml")
	original := "project: demo\nservices:\n  api:\n    port: 3000\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture mtime, then call with non-matching old port.
	stat, _ := os.Stat(path)
	mtime := stat.ModTime()

	if err := UpdatePort(path, "api", "service", 9999, 1234); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stat2, _ := os.Stat(path)
	if !stat2.ModTime().Equal(mtime) {
		t.Error("file mtime changed even though no substitution should have happened")
	}
}
