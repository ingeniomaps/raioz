package auth

import (
	"context"
	"slices"
	"testing"
)

func TestStrictProvider_Identity(t *testing.T) {
	p := &strictProvider{}
	if p.Name() != "" {
		t.Errorf("Name() should be empty for strict; got %q", p.Name())
	}
	if err := p.Validate(context.Background()); err != nil {
		t.Errorf("Validate() should always pass for strict; got %v", err)
	}
	if p.SuggestSetup() == "" {
		t.Error("SuggestSetup() should return non-empty hint")
	}
}

func TestStrictProvider_Prepare_ReproducesV01Hardening(t *testing.T) {
	p := &strictProvider{}
	res, err := p.Prepare(context.Background(), "github.com/foo/bar")
	if err != nil {
		t.Fatalf("Prepare() unexpected error: %v", err)
	}

	// URL passes through unchanged.
	if res.URL != "github.com/foo/bar" {
		t.Errorf("URL should pass through; want %q, got %q",
			"github.com/foo/bar", res.URL)
	}

	// GitArgs must include "-c credential.helper=" before anything
	// else — git option ordering matters (any `-c` must precede the
	// subcommand).
	wantArgs := []string{"-c", "credential.helper="}
	if !slices.Equal(res.GitArgs, wantArgs) {
		t.Errorf("GitArgs: want %v, got %v", wantArgs, res.GitArgs)
	}

	// Env exactly matches the v0.1 hardening that commit 1's
	// defaultHardenedCmd helper applies. If this diverges, the
	// strict provider is no longer a drop-in replacement.
	wantEnv := []string{
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=",
		"GIT_SSH_COMMAND=",
	}
	if !slices.Equal(res.Env, wantEnv) {
		t.Errorf("Env: want %v, got %v", wantEnv, res.Env)
	}

	// Cleanup must never be nil. Callers defer it unconditionally.
	if res.Cleanup == nil {
		t.Fatal("Cleanup must be non-nil")
	}
	// And it must be safely callable (no-op for strict).
	res.Cleanup()
}

func TestStrictProvider_Prepare_URLNotMutatedOnEmptyInput(t *testing.T) {
	p := &strictProvider{}
	res, err := p.Prepare(context.Background(), "")
	if err != nil {
		t.Fatalf("Prepare(\"\"): %v", err)
	}
	if res.URL != "" {
		t.Errorf("empty URL should stay empty; got %q", res.URL)
	}
}

// TestStrictProvider_ImplementsInterface is a compile-time guard:
// if strictProvider drifts away from the Provider interface, this
// stops the build before runtime.
func TestStrictProvider_ImplementsInterface(t *testing.T) {
	var _ Provider = (*strictProvider)(nil)
}
