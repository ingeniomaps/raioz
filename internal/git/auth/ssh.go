package auth

import (
	"context"
	"errors"
)

// sshProvider is a placeholder for the SSH URL-rewriting integration
// that lands in fase 3 of issue 067. Same shape as ghProvider: both
// Validate and Prepare return a "not implemented" error and the
// SuggestSetup points the dev at `auth: inherit` (which already
// honors ssh-agent if the global git config has SSH URLs).
type sshProvider struct{}

// errSSHNotImplemented is the sentinel returned by both Validate and
// Prepare so callers can detect placeholder state via errors.Is.
var errSSHNotImplemented = errors.New(
	"auth: ssh is not yet implemented (issue 067 fase 3). " +
		"Use `auth: inherit` with SSH-form repo URLs in your git " +
		"config to get the same effect today")

func (s *sshProvider) Name() string {
	return "ssh"
}

func (s *sshProvider) Validate(_ context.Context) error {
	return errSSHNotImplemented
}

func (s *sshProvider) Prepare(_ context.Context, _ string) (PrepareResult, error) {
	return PrepareResult{}, errSSHNotImplemented
}

func (s *sshProvider) SuggestSetup() string {
	return "auth: ssh lands in fase 3 of issue 067. Until then, use " +
		"`auth: inherit` and write the repo URL in SSH form " +
		"(`git@github.com:owner/repo.git`) — ssh-agent picks it up " +
		"automatically"
}
