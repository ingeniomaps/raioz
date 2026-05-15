package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSshProvider_Identity(t *testing.T) {
	p, err := ProviderFor("ssh")
	if err != nil {
		t.Fatalf("ProviderFor(\"ssh\"): %v", err)
	}
	if p.Name() != "ssh" {
		t.Errorf("Name(): want \"ssh\", got %q", p.Name())
	}
	if p.SuggestSetup() == "" {
		t.Error("SuggestSetup(): expected non-empty hint")
	}
}

func TestSshProvider_ValidateFailsWithSentinel(t *testing.T) {
	p, _ := ProviderFor("ssh")
	err := p.Validate(context.Background())
	if err == nil {
		t.Fatal("Validate() should fail for placeholder ssh")
	}
	if !errors.Is(err, errSSHNotImplemented) {
		t.Errorf("expected errSSHNotImplemented sentinel, got: %v", err)
	}
	if !strings.Contains(err.Error(), "inherit") {
		t.Errorf("error %q should point users at inherit as the workaround",
			err.Error())
	}
}

func TestSshProvider_PrepareFails(t *testing.T) {
	p, _ := ProviderFor("ssh")
	res, err := p.Prepare(context.Background(), "github.com/foo/bar")
	if err == nil {
		t.Fatal("Prepare() should fail for placeholder ssh")
	}
	if !errors.Is(err, errSSHNotImplemented) {
		t.Errorf("expected errSSHNotImplemented sentinel, got: %v", err)
	}
	if res.GitArgs != nil {
		t.Errorf("placeholder Prepare should return zero PrepareResult; "+
			"got GitArgs=%v", res.GitArgs)
	}
}

func TestSshProvider_ImplementsInterface(t *testing.T) {
	var _ Provider = (*sshProvider)(nil)
}
