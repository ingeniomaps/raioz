package app

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestNewDoctorUseCase(t *testing.T) {
	uc := NewDoctorUseCase()
	if uc == nil {
		t.Fatal("expected non-nil DoctorUseCase")
	}
	if uc.Out == nil {
		t.Error("expected non-nil Out writer")
	}
}

func TestDoctorCheckOS(t *testing.T) {
	initI18nForTest(t)
	uc := NewDoctorUseCase()
	check := uc.checkOS()
	if check.Status != "ok" {
		t.Errorf("expected os check status 'ok', got %q", check.Status)
	}
	if check.Message == "" {
		t.Error("expected non-empty os message")
	}
}

func TestDoctorCheckDiskSpace(t *testing.T) {
	initI18nForTest(t)
	uc := NewDoctorUseCase()
	check := uc.checkDiskSpace()
	// Should return one of ok/warning/error, not empty
	if check.Status != "ok" && check.Status != "warning" && check.Status != "error" {
		t.Errorf("unexpected status %q", check.Status)
	}
	if check.Name == "" {
		t.Error("expected non-empty name")
	}
}

func TestDoctorCheckRaiozDir(t *testing.T) {
	initI18nForTest(t)
	uc := NewDoctorUseCase()
	check := uc.checkRaiozDir()
	// Either warning (missing) or ok; both are valid test outcomes
	if check.Status == "" {
		t.Error("expected non-empty status")
	}
}

func TestDoctorCheckGit(t *testing.T) {
	initI18nForTest(t)
	uc := NewDoctorUseCase()
	// Git is typically installed in test envs; either ok or error is fine.
	check := uc.checkGit(context.Background())
	if check.Name != "Git" {
		t.Errorf("expected name 'Git', got %q", check.Name)
	}
}

func TestDoctorCheckDocker(t *testing.T) {
	initI18nForTest(t)
	uc := NewDoctorUseCase()
	check := uc.checkDocker(context.Background())
	if check.Name != "Docker" {
		t.Errorf("expected name 'Docker', got %q", check.Name)
	}
}

func TestDoctorCheckDockerCompose(t *testing.T) {
	initI18nForTest(t)
	uc := NewDoctorUseCase()
	check := uc.checkDockerCompose(context.Background())
	if check.Name != "Docker Compose" {
		t.Errorf("expected name 'Docker Compose', got %q", check.Name)
	}
}

func TestDoctorCheckCaddy(t *testing.T) {
	initI18nForTest(t)
	uc := NewDoctorUseCase()
	check := uc.checkCaddy(context.Background())
	if check.Name != "Caddy" {
		t.Errorf("expected name 'Caddy', got %q", check.Name)
	}
}

func TestDoctorCheckMkcert(t *testing.T) {
	initI18nForTest(t)
	uc := NewDoctorUseCase()
	check := uc.checkMkcert(context.Background())
	if check.Name != "mkcert" {
		t.Errorf("expected name 'mkcert', got %q", check.Name)
	}
}

func TestDoctorCheckRuntimes(t *testing.T) {
	initI18nForTest(t)
	uc := NewDoctorUseCase()
	check := uc.checkRuntimes(context.Background())
	if check.Name != "Runtimes" {
		t.Errorf("expected name 'Runtimes', got %q", check.Name)
	}
}

func TestDoctorExecute(t *testing.T) {
	initI18nForTest(t)
	var buf bytes.Buffer
	uc := &DoctorUseCase{Out: &buf}
	// Execute may return error if docker is not running, but should not panic.
	_ = uc.Execute(context.Background())
	if buf.Len() == 0 {
		t.Error("expected some output from doctor execute")
	}
}

func TestGetFreeDiskSpaceGB(t *testing.T) {
	gb := getFreeDiskSpaceGB()
	// Should return a sensible value (non-negative or -1 on error)
	if gb < -1 {
		t.Errorf("unexpected value: %f", gb)
	}
}

// TestDoctorCheckEnvironment_NoOverrides pins issue 062's "ok" branch:
// when no env override is set, the doctor reports "no overrides".
func TestDoctorCheckEnvironment_NoOverrides(t *testing.T) {
	// Make sure neither launcher env var leaks from the host shell.
	os.Unsetenv("RAIOZ_LAUNCHER_TIMEOUT")
	os.Unsetenv("RAIOZ_LAUNCHER_DRAIN_TIMEOUT")

	uc := NewDoctorUseCase()
	check := uc.checkEnvironment()
	if check.Status != "ok" {
		t.Errorf("expected ok, got %q (msg=%s)", check.Status, check.Message)
	}
	if !strings.Contains(check.Message, "no overrides") {
		t.Errorf("expected 'no overrides' in message, got %q", check.Message)
	}
}

func TestDoctorCheckEnvironment_ValidOverride(t *testing.T) {
	t.Setenv("RAIOZ_LAUNCHER_TIMEOUT", "120s")
	t.Setenv("RAIOZ_LAUNCHER_DRAIN_TIMEOUT", "45s")

	uc := NewDoctorUseCase()
	check := uc.checkEnvironment()
	if check.Status != "ok" {
		t.Errorf("expected ok for valid overrides, got %q (msg=%s)", check.Status, check.Message)
	}
	for _, want := range []string{"RAIOZ_LAUNCHER_TIMEOUT=2m0s", "RAIOZ_LAUNCHER_DRAIN_TIMEOUT=45s"} {
		if !strings.Contains(check.Message, want) {
			t.Errorf("expected %q in message; got %q", want, check.Message)
		}
	}
}

func TestDoctorCheckEnvironment_MalformedSurfaces(t *testing.T) {
	// Typo: "60" without a unit — the bug from issue 062.
	t.Setenv("RAIOZ_LAUNCHER_TIMEOUT", "60")
	os.Unsetenv("RAIOZ_LAUNCHER_DRAIN_TIMEOUT")

	uc := NewDoctorUseCase()
	check := uc.checkEnvironment()
	if check.Status != "error" {
		t.Errorf("expected error status for malformed env, got %q (msg=%s)", check.Status, check.Message)
	}
	for _, want := range []string{"RAIOZ_LAUNCHER_TIMEOUT", "60", "60s", "expected Go duration"} {
		if !strings.Contains(check.Message, want) {
			t.Errorf("expected %q in message; got %q", want, check.Message)
		}
	}
}
