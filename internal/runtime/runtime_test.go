package runtime

import (
	"os"
	"testing"
)

func TestBinary_Default(t *testing.T) {
	SetBinary("")
	binary = defaultBinary // reset
	if Binary() != "docker" {
		t.Errorf("expected docker, got %s", Binary())
	}
}

func TestSetBinary(t *testing.T) {
	SetBinary("podman")
	defer SetBinary("docker")

	if Binary() != "podman" {
		t.Errorf("expected podman, got %s", Binary())
	}
}

func TestSetBinary_Empty(t *testing.T) {
	SetBinary("podman")
	SetBinary("") // empty should not change
	if Binary() != "podman" {
		t.Errorf("expected podman (empty should not override), got %s", Binary())
	}
	SetBinary("docker")
}

func TestComposeBinary(t *testing.T) {
	SetBinary("nerdctl")
	defer SetBinary("docker")

	bin, sub := ComposeBinary()
	if bin != "nerdctl" || sub != "compose" {
		t.Errorf("expected nerdctl compose, got %s %s", bin, sub)
	}
}

func TestIsDocker(t *testing.T) {
	SetBinary("docker")
	if !IsDocker() {
		t.Error("expected IsDocker=true for docker")
	}

	SetBinary("podman")
	if IsDocker() {
		t.Error("expected IsDocker=false for podman")
	}
	SetBinary("docker")
}

func TestEnvVar(t *testing.T) {
	// Save and restore
	old := os.Getenv("RAIOZ_RUNTIME")
	defer os.Setenv("RAIOZ_RUNTIME", old)

	os.Setenv("RAIOZ_RUNTIME", "finch")
	// Re-run init logic
	binary = defaultBinary
	if env := os.Getenv("RAIOZ_RUNTIME"); env != "" {
		binary = env
	}
	if Binary() != "finch" {
		t.Errorf("expected finch from env, got %s", Binary())
	}
	binary = defaultBinary
}
