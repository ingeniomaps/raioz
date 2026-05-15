package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestGhProvider_Identity(t *testing.T) {
	p, err := ProviderFor("gh")
	if err != nil {
		t.Fatalf("ProviderFor(\"gh\"): %v", err)
	}
	if p.Name() != "gh" {
		t.Errorf("Name(): want \"gh\", got %q", p.Name())
	}
	if p.SuggestSetup() == "" {
		t.Error("SuggestSetup(): expected non-empty hint")
	}
}

// TestGhProvider_ValidateFailsWithSentinel is load-bearing: when
// fase 2 lands and the real impl replaces this placeholder, the
// sentinel error goes away. Until then, the sentinel makes the
// placeholder state programmatically detectable.
func TestGhProvider_ValidateFailsWithSentinel(t *testing.T) {
	p, _ := ProviderFor("gh")
	err := p.Validate(context.Background())
	if err == nil {
		t.Fatal("Validate() should fail for placeholder gh")
	}
	if !errors.Is(err, errGhNotImplemented) {
		t.Errorf("expected errGhNotImplemented sentinel, got: %v", err)
	}
	if !strings.Contains(err.Error(), "inherit") {
		t.Errorf("error %q should point users at inherit as the workaround",
			err.Error())
	}
}

func TestGhProvider_PrepareFails(t *testing.T) {
	p, _ := ProviderFor("gh")
	res, err := p.Prepare(context.Background(), "github.com/foo/bar")
	if err == nil {
		t.Fatal("Prepare() should fail for placeholder gh")
	}
	if !errors.Is(err, errGhNotImplemented) {
		t.Errorf("expected errGhNotImplemented sentinel, got: %v", err)
	}
	// The zero-value PrepareResult is fine — callers must not use it
	// when err != nil. Explicit check on the GitArgs field is enough
	// to confirm we did not partially populate the struct.
	if res.GitArgs != nil {
		t.Errorf("placeholder Prepare should return zero PrepareResult; "+
			"got GitArgs=%v", res.GitArgs)
	}
}

func TestGhProvider_ImplementsInterface(t *testing.T) {
	var _ Provider = (*ghProvider)(nil)
}
