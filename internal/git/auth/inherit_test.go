package auth

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestInheritProvider_Identity(t *testing.T) {
	p, err := ProviderFor("inherit")
	if err != nil {
		t.Fatalf("ProviderFor(\"inherit\"): %v", err)
	}
	if p.Name() != "inherit" {
		t.Errorf("Name(): want \"inherit\", got %q", p.Name())
	}
	if err := p.Validate(context.Background()); err != nil {
		t.Errorf("Validate() should always pass for inherit; got %v", err)
	}
	if p.SuggestSetup() == "" {
		t.Error("SuggestSetup() should return non-empty hint")
	}
}

// TestInheritProvider_Prepare_NoHardening is the load-bearing test:
// inherit's contract is "do NOT apply the strict hardening". If the
// strict env vars or git args leak in, the provider has betrayed
// its purpose.
func TestInheritProvider_Prepare_NoHardening(t *testing.T) {
	p, _ := ProviderFor("inherit")
	res, err := p.Prepare(context.Background(), "github.com/foo/bar")
	if err != nil {
		t.Fatalf("Prepare(): %v", err)
	}

	// URL passes through unchanged.
	if res.URL != "github.com/foo/bar" {
		t.Errorf("URL: want unchanged, got %q", res.URL)
	}

	// MUST NOT inject `-c credential.helper=` — that would disable
	// the user's credential helper, defeating the whole purpose.
	for _, arg := range res.GitArgs {
		if strings.Contains(arg, "credential.helper") {
			t.Errorf("GitArgs leaks credential.helper override: %v", res.GitArgs)
		}
	}

	// MUST NOT clear GIT_ASKPASS / GIT_SSH_COMMAND. Clearing them
	// would defeat the user's askpass program / SSH agent.
	for _, env := range res.Env {
		if env == "GIT_ASKPASS=" {
			t.Errorf("env clears GIT_ASKPASS, defeating user's askpass")
		}
		if env == "GIT_SSH_COMMAND=" {
			t.Errorf("env clears GIT_SSH_COMMAND, defeating ssh-agent")
		}
	}

	if res.Cleanup == nil {
		t.Fatal("Cleanup must be non-nil")
	}
	res.Cleanup()
}

// TestInheritProvider_Prepare_KeepsTerminalPromptOff documents the
// one piece of "hardening" inherit DOES apply: never let git open
// an interactive password prompt. raioz exec is non-interactive;
// a prompt would hang.
func TestInheritProvider_Prepare_KeepsTerminalPromptOff(t *testing.T) {
	p, _ := ProviderFor("inherit")
	res, err := p.Prepare(context.Background(), "github.com/foo/bar")
	if err != nil {
		t.Fatalf("Prepare(): %v", err)
	}
	if !slices.Contains(res.Env, "GIT_TERMINAL_PROMPT=0") {
		t.Errorf("env should include GIT_TERMINAL_PROMPT=0 to avoid hung prompts; got %v", res.Env)
	}
}

func TestInheritProvider_ImplementsInterface(t *testing.T) {
	var _ Provider = (*inheritProvider)(nil)
}
