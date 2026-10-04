package netutil

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestGitRemoteEndpoint(t *testing.T) {
	tests := []struct {
		repo, want string
		ok         bool
	}{
		{"https://github.com/acme/api.git", "github.com:443", true},
		{"http://git.acme.dev/api", "git.acme.dev:80", true},
		{"https://git.acme.dev:8443/api.git", "git.acme.dev:8443", true},
		{"https://token@github.com/acme/api.git", "github.com:443", true},
		{"git@github.com:acme/api.git", "github.com:22", true},
		{"bitbucket.org:acme/api.git", "bitbucket.org:22", true},
		{"ssh://git@git.acme.dev:2222/acme/api.git", "git.acme.dev:2222", true},
		{"ssh://git@git.acme.dev/acme/api.git", "git.acme.dev:22", true},
		{"git://git.acme.dev/api.git", "git.acme.dev:9418", true},
		{"../sibling/api", "", false},
		{"/srv/git/api.git", "", false},
		{"./api:latest", "", false},
		{`C:\repos\api`, "", false},
		{"file:///srv/git/api.git", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.repo, func(t *testing.T) {
			got, ok := GitRemoteEndpoint(tt.repo)
			if got != tt.want || ok != tt.ok {
				t.Errorf("got %q, %v; want %q, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestUnreachableGitHosts(t *testing.T) {
	var dialled []string
	prev := dialGitHost
	dialGitHost = func(_ context.Context, hostport string) error {
		dialled = append(dialled, hostport)
		if hostport == "github.com:443" {
			return nil
		}
		return errors.New("no route")
	}
	t.Cleanup(func() { dialGitHost = prev })

	got := UnreachableGitHosts(context.Background(), []string{
		"https://github.com/acme/api.git",
		"https://github.com/acme/web.git", // same host: asked once
		"git@git.acme.dev:acme/core.git",
		"https://zeta.example/x.git",
		"../local/checkout", // no host: not asked
	})
	if want := []string{"git.acme.dev", "zeta.example"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if len(dialled) != 3 {
		t.Errorf("each host is asked once, dialled %v", dialled)
	}
	if got := UnreachableGitHosts(context.Background(), nil); got != nil {
		t.Errorf("a project that clones nothing asks nobody, got %v", got)
	}
}
