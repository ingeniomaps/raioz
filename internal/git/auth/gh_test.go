package auth

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
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

func TestGhProvider_ImplementsInterface(t *testing.T) {
	var _ Provider = (*ghProvider)(nil)
}

// stubGhExec swaps both seams for the test's lifetime and returns
// them to their production values on cleanup. lookPath decides
// whether `gh` is "installed"; cmdRunner returns an *exec.Cmd whose
// Run() reproduces the real binary's success / failure behavior.
func stubGhExec(t *testing.T,
	lookPath func(string) (string, error),
	cmdRunner func(ctx context.Context, name string, args ...string) *exec.Cmd,
) {
	t.Helper()
	prevLookPath := ghExecLookPath
	prevCommand := ghExecCommand
	ghExecLookPath = lookPath
	ghExecCommand = cmdRunner
	t.Cleanup(func() {
		ghExecLookPath = prevLookPath
		ghExecCommand = prevCommand
	})
}

func helperSuccess(ctx context.Context, _ string, _ ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "true")
}

func helperFailure(ctx context.Context, _ string, _ ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "false")
}

func TestGhProvider_Validate_OK(t *testing.T) {
	if _, err := exec.LookPath("true"); err != nil {
		t.Skip("`true` not in PATH on this OS")
	}
	stubGhExec(t,
		func(string) (string, error) { return "/usr/bin/gh", nil },
		helperSuccess,
	)
	p, _ := ProviderFor("gh")
	if err := p.Validate(context.Background()); err != nil {
		t.Errorf("Validate(): expected nil, got %v", err)
	}
}

func TestGhProvider_Validate_NotInstalled(t *testing.T) {
	stubGhExec(t,
		func(string) (string, error) { return "", errors.New("not found") },
		helperSuccess,
	)
	p, _ := ProviderFor("gh")
	err := p.Validate(context.Background())
	if !errors.Is(err, errGhNotInstalled) {
		t.Errorf("expected errGhNotInstalled, got %v", err)
	}
	if !strings.Contains(err.Error(), "GitHub CLI") {
		t.Errorf("error %q should mention GitHub CLI", err.Error())
	}
}

func TestGhProvider_Validate_NotLoggedIn(t *testing.T) {
	if _, err := exec.LookPath("false"); err != nil {
		t.Skip("`false` not in PATH on this OS")
	}
	stubGhExec(t,
		func(string) (string, error) { return "/usr/bin/gh", nil },
		helperFailure,
	)
	p, _ := ProviderFor("gh")
	err := p.Validate(context.Background())
	if !errors.Is(err, errGhNotLoggedIn) {
		t.Errorf("expected errGhNotLoggedIn, got %v", err)
	}
	if !strings.Contains(err.Error(), "gh auth login") {
		t.Errorf("error %q should mention `gh auth login`", err.Error())
	}
}

func TestGhProvider_Prepare_GitArgsAndEnv(t *testing.T) {
	p, _ := ProviderFor("gh")
	res, err := p.Prepare(context.Background(), "https://github.com/foo/bar")
	if err != nil {
		t.Fatalf("Prepare(): %v", err)
	}
	wantArgs := []string{
		"-c", "credential.helper=",
		"-c", "credential.helper=!gh auth git-credential",
	}
	if !reflect.DeepEqual(res.GitArgs, wantArgs) {
		t.Errorf("GitArgs:\n got %q\nwant %q", res.GitArgs, wantArgs)
	}
	gotTerminalProm := false
	for _, e := range res.Env {
		if e == "GIT_TERMINAL_PROMPT=0" {
			gotTerminalProm = true
		}
	}
	if !gotTerminalProm {
		t.Errorf("expected GIT_TERMINAL_PROMPT=0 in Env, got %v", res.Env)
	}
	if res.URL != "https://github.com/foo/bar" {
		t.Errorf("URL: gh provider must not rewrite, got %q", res.URL)
	}
	if res.Cleanup == nil {
		t.Error("Cleanup must always be non-nil (per Provider contract)")
	}
}
