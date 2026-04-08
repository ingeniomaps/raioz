package app

import (
	"testing"
)

func TestNewInitUseCase(t *testing.T) {
	deps := &Dependencies{}
	uc := NewInitUseCase(deps)

	if uc == nil {
		t.Fatal("NewInitUseCase should return non-nil")
	}
	if uc.useCase == nil {
		t.Fatal("useCase should be initialized")
	}
}

func TestInitOptionsDefaults(t *testing.T) {
	opts := InitOptions{}
	if opts.OutputPath != "" {
		t.Errorf("default OutputPath should be empty, got %q", opts.OutputPath)
	}
}
