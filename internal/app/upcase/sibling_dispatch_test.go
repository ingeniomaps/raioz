package upcase

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"raioz/internal/config"
	rt "raioz/internal/runtime"
)

// writeFakeRuntime points runtime.SetBinary at a shell script that
// echoes `stdout` and exits 0. Used to mock docker.IsProjectActive
// without a daemon.
func writeFakeRuntime(t *testing.T, stdout string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake binary scripts are POSIX-only")
	}
	dir := t.TempDir()
	body := "#!/bin/sh\nprintf '%s' '" + stdout + "'\nexit 0\n"
	p := filepath.Join(dir, "fakedocker")
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	prev := rt.Binary()
	rt.SetBinary(p)
	t.Cleanup(func() { rt.SetBinary(prev) })
}

func writeSiblingYAML(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "raioz.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	return dir
}

// --- non-sibling deps ----------------------------------------------------

func TestDecideSibling_NilInline(t *testing.T) {
	got, err := decideSibling(context.Background(), "redis", nil, "ws")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Kind != siblingProceed {
		t.Errorf("expected siblingProceed for nil inline, got %d", got.Kind)
	}
}

func TestDecideSibling_RegularImageDep(t *testing.T) {
	inline := &config.Infra{Image: "postgres", Tag: "16"}
	got, err := decideSibling(context.Background(), "postgres", inline, "ws")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Kind != siblingProceed {
		t.Errorf("regular dep should proceed, got %d", got.Kind)
	}
}

// --- mode A (project:) ---------------------------------------------------

func TestDecideSibling_ModeAProducesError(t *testing.T) {
	inline := &config.Infra{Project: "/abs/keycloak"}
	got, err := decideSibling(context.Background(), "keycloak", inline, "hypixo")
	if err != nil {
		t.Fatalf("decideSibling itself should not return Go error, got %v", err)
	}
	if got.Kind != siblingErrorModeA {
		t.Errorf("expected siblingErrorModeA, got %d", got.Kind)
	}
	if !strings.Contains(got.Reason, "siblingProject") {
		t.Errorf("reason should hint at the workaround, got %q", got.Reason)
	}
}

// --- mode B (siblingProject:) --------------------------------------------

func TestDecideSibling_ModeB_SiblingActive(t *testing.T) {
	siblingDir := writeSiblingYAML(t,
		"workspace: hypixo\nproject: keycloak\n"+
			"services:\n  keycloak:\n    path: .\n")
	writeFakeRuntime(t, "hypixo-keycloak\n") // docker ps reports the container

	inline := &config.Infra{
		Image:          "keycloak",
		Tag:            "24",
		SiblingProject: siblingDir,
	}
	got, err := decideSibling(context.Background(), "keycloak", inline, "hypixo")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Kind != siblingSkipDeferred {
		t.Errorf("expected siblingSkipDeferred, got %d (reason: %s)", got.Kind, got.Reason)
	}
	if got.SiblingName != "keycloak" {
		t.Errorf("SiblingName = %q, want keycloak", got.SiblingName)
	}
}

func TestDecideSibling_ModeB_SiblingInactive(t *testing.T) {
	siblingDir := writeSiblingYAML(t,
		"workspace: hypixo\nproject: keycloak\n"+
			"services:\n  keycloak:\n    path: .\n")
	writeFakeRuntime(t, "") // docker ps returns nothing → not active

	inline := &config.Infra{
		Image:          "keycloak",
		SiblingProject: siblingDir,
	}
	got, err := decideSibling(context.Background(), "keycloak", inline, "hypixo")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Kind != siblingProceed {
		t.Errorf("inactive sibling should fall back to image, got %d", got.Kind)
	}
}

func TestDecideSibling_ModeB_SiblingMissing(t *testing.T) {
	inline := &config.Infra{
		Image:          "keycloak",
		SiblingProject: "/does/not/exist",
	}
	_, err := decideSibling(context.Background(), "keycloak", inline, "hypixo")
	if err == nil || !strings.Contains(err.Error(), "resolve sibling") {
		t.Errorf("expected resolve-sibling error, got %v", err)
	}
}

func TestDecideSibling_ModeB_WorkspaceMismatch(t *testing.T) {
	siblingDir := writeSiblingYAML(t,
		"workspace: acme\nproject: keycloak\n"+
			"services:\n  keycloak:\n    path: .\n")
	// Fake runtime not needed — workspace check fails before probing.

	inline := &config.Infra{
		Image:          "keycloak",
		SiblingProject: siblingDir,
	}
	_, err := decideSibling(context.Background(), "keycloak", inline, "hypixo")
	if err == nil || !strings.Contains(err.Error(), "workspace") {
		t.Errorf("expected workspace mismatch error, got %v", err)
	}
}
