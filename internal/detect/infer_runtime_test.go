package detect

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/domain/models"
)

func TestInferCommandsForRuntime_NPM_PrefersDevScript(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"scripts":{"dev":"next dev","start":"next start"}}`
	mustWrite(t, filepath.Join(dir, "package.json"), pkg)

	var result models.DetectResult
	InferCommandsForRuntime(&result, dir, models.RuntimeNPM)

	if result.DevCommand != "npm run dev" {
		t.Errorf("DevCommand = %q, want %q", result.DevCommand, "npm run dev")
	}
	if result.StartCommand != "npm run dev" {
		t.Errorf("StartCommand = %q, want %q", result.StartCommand, "npm run dev")
	}
	if !result.HasHotReload {
		t.Errorf("HasHotReload = false, want true (next dev triggers hot-reload)")
	}
}

func TestInferCommandsForRuntime_NPM_PnpmLockfileSwapsPrefix(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"scripts":{"dev":"vite"}}`
	mustWrite(t, filepath.Join(dir, "package.json"), pkg)
	mustWrite(t, filepath.Join(dir, "pnpm-lock.yaml"), "lockfileVersion: 6.0\n")

	var result models.DetectResult
	InferCommandsForRuntime(&result, dir, models.RuntimeNPM)

	if result.DevCommand != "pnpm run dev" {
		t.Errorf("DevCommand = %q, want %q (pnpm-lock.yaml present)",
			result.DevCommand, "pnpm run dev")
	}
}

func TestInferCommandsForRuntime_NPM_NoPackageJSONFallback(t *testing.T) {
	dir := t.TempDir() // empty dir, override requested anyway

	var result models.DetectResult
	InferCommandsForRuntime(&result, dir, models.RuntimeNPM)

	// inferFromPackageJSON falls back to "npm start" when it can't read
	// the file — same behaviour as Detect(). The npm runner then surfaces
	// a clear "missing manifest" error downstream.
	if result.StartCommand != "npm start" {
		t.Errorf("StartCommand = %q, want %q (fallback)",
			result.StartCommand, "npm start")
	}
}

func TestInferCommandsForRuntime_Go_DetectsAir(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module x\n")
	mustWrite(t, filepath.Join(dir, ".air.toml"), "")

	var result models.DetectResult
	InferCommandsForRuntime(&result, dir, models.RuntimeGo)

	if result.DevCommand != "air" {
		t.Errorf("DevCommand = %q, want %q", result.DevCommand, "air")
	}
	if !result.HasHotReload {
		t.Errorf("HasHotReload = false, want true (.air.toml present)")
	}
}

func TestInferCommandsForRuntime_Go_NoAirUsesGoRun(t *testing.T) {
	dir := t.TempDir()

	var result models.DetectResult
	InferCommandsForRuntime(&result, dir, models.RuntimeGo)

	if result.StartCommand != "go run ." {
		t.Errorf("StartCommand = %q, want %q", result.StartCommand, "go run .")
	}
	if result.HasHotReload {
		t.Errorf("HasHotReload = true, want false (no .air.toml)")
	}
}

func TestInferCommandsForRuntime_Dockerfile_DoesNotShadowNpm(t *testing.T) {
	// When the user keeps `runtime: dockerfile` explicitly, the docker
	// build/run command is the expected output (the override matches the
	// auto-detect; no surprise).
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "Dockerfile"), "FROM alpine\n")

	var result models.DetectResult
	InferCommandsForRuntime(&result, dir, models.RuntimeDockerfile)

	if result.StartCommand == "" {
		t.Errorf("StartCommand empty; expected docker build/run line")
	}
	if result.DevCommand != "" {
		t.Errorf("DevCommand = %q, want empty (dockerfile has no dev shortcut)",
			result.DevCommand)
	}
}

func TestInferCommandsForRuntime_Make_PicksDevTarget(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "Makefile"), "dev:\n\techo dev\nstart:\n\techo start\n")

	var result models.DetectResult
	InferCommandsForRuntime(&result, dir, models.RuntimeMake)

	if result.StartCommand != "make dev" {
		t.Errorf("StartCommand = %q, want %q", result.StartCommand, "make dev")
	}
}

func TestInferCommandsForRuntime_Bun_LiteralCommand(t *testing.T) {
	dir := t.TempDir()

	var result models.DetectResult
	InferCommandsForRuntime(&result, dir, models.RuntimeBun)

	if result.StartCommand != "bun run dev" {
		t.Errorf("StartCommand = %q, want %q", result.StartCommand, "bun run dev")
	}
	if !result.HasHotReload {
		t.Error("HasHotReload should be true for bun")
	}
	if result.Port != 3000 {
		t.Errorf("Port = %d, want 3000", result.Port)
	}
}

func TestInferCommandsForRuntime_Image_NoOp(t *testing.T) {
	// Image runtime isn't an override target — leave commands alone.
	var result models.DetectResult
	InferCommandsForRuntime(&result, t.TempDir(), models.RuntimeImage)

	if result.StartCommand != "" {
		t.Errorf("StartCommand = %q, want empty", result.StartCommand)
	}
}

func TestInferCommandsForRuntime_NilResult_NoPanic(t *testing.T) {
	// Defensive: nil result must not panic. Callers in
	// ResolveServiceDetection always pass non-nil, but the package-level
	// helper deserves the guard.
	InferCommandsForRuntime(nil, t.TempDir(), models.RuntimeNPM)
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
