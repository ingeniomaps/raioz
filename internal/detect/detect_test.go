package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetect_ComposeProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "docker-compose.yml", "version: '3'\nservices:\n  app:\n    image: node\n")

	result := Detect(dir)

	if result.Runtime != RuntimeCompose {
		t.Errorf("expected RuntimeCompose, got %s", result.Runtime)
	}
	if result.ComposeFile == "" {
		t.Error("expected ComposeFile to be set")
	}
	if !result.IsDocker() {
		t.Error("compose should be docker")
	}
}

func TestDetect_Dockerfile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Dockerfile", "FROM node:18\nCOPY . .\nCMD [\"node\", \"app.js\"]\n")

	result := Detect(dir)

	if result.Runtime != RuntimeDockerfile {
		t.Errorf("expected RuntimeDockerfile, got %s", result.Runtime)
	}
	if result.Dockerfile == "" {
		t.Error("expected Dockerfile path to be set")
	}
}

func TestDetect_NPM(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"scripts":{"dev":"next dev","start":"next start"}}`)

	result := Detect(dir)

	if result.Runtime != RuntimeNPM {
		t.Errorf("expected RuntimeNPM, got %s", result.Runtime)
	}
	if result.StartCommand != "npm run dev" {
		t.Errorf("expected 'npm run dev', got '%s'", result.StartCommand)
	}
	if !result.HasHotReload {
		t.Error("next dev should have hot-reload")
	}
	if !result.IsHost() {
		t.Error("npm should be host")
	}
}

func TestDetect_Go(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/api\n\ngo 1.22\n")

	result := Detect(dir)

	if result.Runtime != RuntimeGo {
		t.Errorf("expected RuntimeGo, got %s", result.Runtime)
	}
	if result.StartCommand != "go run ." {
		t.Errorf("expected 'go run .', got '%s'", result.StartCommand)
	}
}

func TestDetect_Makefile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Makefile", ".PHONY: dev\ndev:\n\tgo run .\n")

	result := Detect(dir)

	if result.Runtime != RuntimeMake {
		t.Errorf("expected RuntimeMake, got %s", result.Runtime)
	}
	if result.StartCommand != "make dev" {
		t.Errorf("expected 'make dev', got '%s'", result.StartCommand)
	}
}

func TestDetect_Python(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", "[project]\nname = \"myapp\"\n")

	result := Detect(dir)

	if result.Runtime != RuntimePython {
		t.Errorf("expected RuntimePython, got %s", result.Runtime)
	}
}

func TestDetect_Rust(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Cargo.toml", "[package]\nname = \"myapp\"\n")

	result := Detect(dir)

	if result.Runtime != RuntimeRust {
		t.Errorf("expected RuntimeRust, got %s", result.Runtime)
	}
	if result.StartCommand != "cargo run" {
		t.Errorf("expected 'cargo run', got '%s'", result.StartCommand)
	}
}

func TestDetect_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	result := Detect(dir)

	if result.Runtime != RuntimeUnknown {
		t.Errorf("expected RuntimeUnknown, got %s", result.Runtime)
	}
}

func TestDetect_NonexistentDir(t *testing.T) {
	result := Detect("/nonexistent/path/that/does/not/exist")

	if result.Runtime != RuntimeUnknown {
		t.Errorf("expected RuntimeUnknown, got %s", result.Runtime)
	}
}

func TestDetect_Priority_ComposeOverDockerfile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "docker-compose.yml", "version: '3'\n")
	writeFile(t, dir, "Dockerfile", "FROM node:18\n")
	writeFile(t, dir, "package.json", `{"scripts":{"start":"node app.js"}}`)

	result := Detect(dir)

	if result.Runtime != RuntimeCompose {
		t.Errorf("compose should take priority, got %s", result.Runtime)
	}
}

func TestDetect_Priority_DockerfileOverNPM(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Dockerfile", "FROM node:18\n")
	writeFile(t, dir, "package.json", `{"scripts":{"start":"node app.js"}}`)

	result := Detect(dir)

	if result.Runtime != RuntimeDockerfile {
		t.Errorf("dockerfile should take priority over npm, got %s", result.Runtime)
	}
}

func TestDetect_NPMPortInference(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"scripts":{"dev":"vite --port 8080"}}`)

	result := Detect(dir)

	if result.Port != 8080 {
		t.Errorf("expected port 8080, got %d", result.Port)
	}
}

func TestDetect_NPMViteDefaultPort(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"scripts":{"dev":"vite"}}`)

	result := Detect(dir)

	if result.Port != 5173 {
		t.Errorf("expected vite default port 5173, got %d", result.Port)
	}
}

func TestForImage(t *testing.T) {
	result := ForImage("postgres:16")

	if result.Runtime != RuntimeImage {
		t.Errorf("expected RuntimeImage, got %s", result.Runtime)
	}
	if result.IsHost() {
		t.Error("image should not be host")
	}
	if !result.IsDocker() {
		t.Error("image should be docker")
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}
}
