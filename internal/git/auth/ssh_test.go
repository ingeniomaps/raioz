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

func TestSshProvider_ImplementsInterface(t *testing.T) {
	var _ Provider = (*sshProvider)(nil)
}

func stubSSHLookPath(t *testing.T, fn func(string) (string, error)) {
	t.Helper()
	prev := sshExecLookPath
	sshExecLookPath = fn
	t.Cleanup(func() { sshExecLookPath = prev })
}

func TestSshProvider_Validate_OK(t *testing.T) {
	stubSSHLookPath(t, func(string) (string, error) { return "/usr/bin/ssh", nil })
	p, _ := ProviderFor("ssh")
	if err := p.Validate(context.Background()); err != nil {
		t.Errorf("Validate(): expected nil, got %v", err)
	}
}

func TestSshProvider_Validate_NotAvailable(t *testing.T) {
	stubSSHLookPath(t, func(string) (string, error) {
		return "", errors.New("not found")
	})
	p, _ := ProviderFor("ssh")
	err := p.Validate(context.Background())
	if !errors.Is(err, errSSHNotAvailable) {
		t.Errorf("expected errSSHNotAvailable, got %v", err)
	}
	if !strings.Contains(err.Error(), "ssh-agent") {
		t.Errorf("error %q should hint at ssh-agent setup", err.Error())
	}
}

func TestSshProvider_Prepare_HardensSSHCommand(t *testing.T) {
	p, _ := ProviderFor("ssh")
	res, err := p.Prepare(context.Background(), "https://github.com/foo/bar")
	if err != nil {
		t.Fatalf("Prepare(): %v", err)
	}
	var sshCmd string
	for _, e := range res.Env {
		if strings.HasPrefix(e, "GIT_SSH_COMMAND=") {
			sshCmd = strings.TrimPrefix(e, "GIT_SSH_COMMAND=")
		}
	}
	if sshCmd == "" {
		t.Fatal("expected GIT_SSH_COMMAND in Env")
	}
	for _, frag := range []string{
		"StrictHostKeyChecking=accept-new",
		"BatchMode=yes",
		"ConnectTimeout=",
	} {
		if !strings.Contains(sshCmd, frag) {
			t.Errorf("GIT_SSH_COMMAND missing %q: %q", frag, sshCmd)
		}
	}
	if res.Cleanup == nil {
		t.Error("Cleanup must always be non-nil (per Provider contract)")
	}
}

func TestRewriteToSSH(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"github https → ssh", "https://github.com/foo/bar", "git@github.com:foo/bar.git"},
		{"github https with .git → ssh", "https://github.com/foo/bar.git", "git@github.com:foo/bar.git"},
		{"gitlab https → ssh", "https://gitlab.com/foo/bar", "git@gitlab.com:foo/bar.git"},
		{"bitbucket https → ssh", "https://bitbucket.org/foo/bar", "git@bitbucket.org:foo/bar.git"},
		{"gitlab subgroup", "https://gitlab.com/org/team/repo", "git@gitlab.com:org/team/repo.git"},
		{"already SSH form", "git@github.com:foo/bar.git", "git@github.com:foo/bar.git"},
		{"ssh:// scheme", "ssh://git@example.com/foo.git", "ssh://git@example.com/foo.git"},
		{"unknown host passes through", "https://gitea.example.com/foo/bar", "https://gitea.example.com/foo/bar"},
		{"non-https unchanged", "/local/path", "/local/path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rewriteToSSH(tc.in)
			if got != tc.want {
				t.Errorf("rewriteToSSH(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSshProvider_Prepare_RewritesURL(t *testing.T) {
	p, _ := ProviderFor("ssh")
	res, err := p.Prepare(context.Background(), "https://github.com/foo/bar")
	if err != nil {
		t.Fatalf("Prepare(): %v", err)
	}
	if res.URL != "git@github.com:foo/bar.git" {
		t.Errorf("URL: got %q, want git@github.com:foo/bar.git", res.URL)
	}
}
