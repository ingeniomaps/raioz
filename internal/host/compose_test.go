package host

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("services: {}\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestDetectComposePathExplicitAbsolute(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "custom.yml")
	writeFile(t, abs)

	got := DetectComposePath(dir, "make up", abs)
	if got != abs {
		t.Errorf("DetectComposePath() = %q, want %q", got, abs)
	}
}

func TestDetectComposePathExplicitRelative(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "sub", "compose.yml"))

	got := DetectComposePath(dir, "make up", "sub/compose.yml")
	want := filepath.Join(dir, "sub", "compose.yml")
	if got != want {
		t.Errorf("DetectComposePath() = %q, want %q", got, want)
	}
}

func TestDetectComposePathFromCommandFlag(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "dev.yml"))

	got := DetectComposePath(dir, "docker-compose -f dev.yml up", "")
	want := filepath.Join(dir, "dev.yml")
	if got != want {
		t.Errorf("DetectComposePath() = %q, want %q", got, want)
	}
}

func TestDetectComposePathFromCommandLongFlag(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "prod.yml"))

	got := DetectComposePath(dir, "docker compose --file prod.yml up -d", "")
	want := filepath.Join(dir, "prod.yml")
	if got != want {
		t.Errorf("DetectComposePath() = %q, want %q", got, want)
	}
}

func TestDetectComposePathFromCommandAbsolute(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "abs.yml")
	writeFile(t, abs)

	got := DetectComposePath(dir, "docker-compose -f "+abs+" up", "")
	if got != abs {
		t.Errorf("DetectComposePath() = %q, want %q", got, abs)
	}
}

func TestDetectComposePathDefaultFiles(t *testing.T) {
	cases := []struct {
		name string
		file string
	}{
		{"docker-compose.yml", "docker-compose.yml"},
		{"docker-compose.yaml", "docker-compose.yaml"},
		{"compose.yml", "compose.yml"},
		{"compose.yaml", "compose.yaml"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, tc.file))

			got := DetectComposePath(dir, "make up", "")
			want := filepath.Join(dir, tc.file)
			if got != want {
				t.Errorf("DetectComposePath() = %q, want %q", got, want)
			}
		})
	}
}

func TestDetectComposePathSubdirectory(t *testing.T) {
	cases := []string{"docker", "compose", ".docker", "deploy", "deployment"}
	for _, subdir := range cases {
		t.Run(subdir, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, subdir, "docker-compose.yml"))

			got := DetectComposePath(dir, "make up", "")
			want := filepath.Join(dir, subdir, "docker-compose.yml")
			if got != want {
				t.Errorf("DetectComposePath() = %q, want %q", got, want)
			}
		})
	}
}

func TestDetectComposePathNotFound(t *testing.T) {
	dir := t.TempDir()
	got := DetectComposePath(dir, "make up", "")
	if got != "" {
		t.Errorf("DetectComposePath() = %q, want empty", got)
	}
}

func TestDetectComposePathExplicitNonexistent(t *testing.T) {
	dir := t.TempDir()
	// Also create a default file so fallback kicks in
	writeFile(t, filepath.Join(dir, "docker-compose.yml"))

	got := DetectComposePath(dir, "make up", "missing.yml")
	want := filepath.Join(dir, "docker-compose.yml")
	if got != want {
		t.Errorf("DetectComposePath() = %q, want %q", got, want)
	}
}

func TestDetectComposePathCommandNoFlag(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "docker-compose.yml"))

	got := DetectComposePath(dir, "docker-compose up -d", "")
	want := filepath.Join(dir, "docker-compose.yml")
	if got != want {
		t.Errorf("DetectComposePath() = %q, want %q", got, want)
	}
}
