package app

import (
	"bytes"
	"context"
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
