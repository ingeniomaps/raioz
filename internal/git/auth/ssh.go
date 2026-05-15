package auth

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// sshProvider rewrites HTTPS git URLs to their SSH form and runs git
// over SSH with secure-by-default options. Auth itself stays with
// ssh-agent / the user's identity file — raioz does not touch keys.
//
// What it does:
//
//   - Rewrites `https://github.com/owner/repo[.git]` to
//     `git@github.com:owner/repo.git` (same for gitlab.com and
//     bitbucket.org — the three hosts with stable, well-known
//     mappings). Other hosts pass through unchanged on the
//     assumption the user already wrote an SSH URL or the host
//     doesn't follow the gh-style mapping.
//   - Sets `GIT_SSH_COMMAND` to `ssh -o StrictHostKeyChecking=accept-new
//     -o BatchMode=yes -o ConnectTimeout=10`. accept-new pins the host
//     key on first contact (avoids the interactive "yes/no?" prompt)
//     while still rejecting later key changes (MITM defense). BatchMode
//     refuses password / passphrase prompts so an unsealed key fails
//     fast instead of hanging.
//
// What it does NOT do:
//
//   - Inject identity files (`-i ~/.ssh/foo_ed25519`). raioz trusts
//     the user's ssh-agent or ~/.ssh/config to pick the right key.
//     Per-service key selection is a deliberate v2 follow-up.
//   - Cache anything. SSH connection sharing (ControlMaster) is the
//     user's call, not raioz's.
type sshProvider struct{}

var errSSHNotAvailable = errors.New(
	"auth: ssh requires the ssh binary on PATH")

// sshExecLookPath is the lookup seam so tests can fake the binary's
// presence without affecting the runner.
var sshExecLookPath = exec.LookPath

func (s *sshProvider) Name() string {
	return "ssh"
}

// Validate confirms that ssh(1) is reachable. We do NOT probe the
// agent / try to list identities here — those checks would gate the
// preflight on user state that is fine to discover at clone time
// with a clearer git error message.
func (s *sshProvider) Validate(_ context.Context) error {
	if _, err := sshExecLookPath("ssh"); err != nil {
		return fmt.Errorf("%w: %s", errSSHNotAvailable, s.SuggestSetup())
	}
	return nil
}

// Prepare returns the SSH-form URL plus the hardened GIT_SSH_COMMAND.
// URL rewriting is conservative: only the three well-known hosts
// where the mapping is unambiguous. Other inputs pass through, so a
// user with a self-hosted Gitea / Gerrit / cgit either provides an
// SSH URL up-front or stays on `auth: inherit`.
func (s *sshProvider) Prepare(_ context.Context, repoURL string) (PrepareResult, error) {
	return PrepareResult{
		GitArgs: nil,
		Env: []string{
			"GIT_SSH_COMMAND=ssh -o StrictHostKeyChecking=accept-new " +
				"-o BatchMode=yes -o ConnectTimeout=10",
			"GIT_TERMINAL_PROMPT=0",
		},
		URL:     rewriteToSSH(repoURL),
		Cleanup: func() {},
	}, nil
}

func (s *sshProvider) SuggestSetup() string {
	return "load your private key into ssh-agent (`ssh-add ~/.ssh/id_ed25519`) " +
		"or set up ~/.ssh/config with a Host alias; raioz rewrites HTTPS URLs " +
		"for github.com / gitlab.com / bitbucket.org and runs git over SSH"
}

// rewriteToSSH maps HTTPS clone URLs to SSH for the three hosts where
// the mapping is unambiguous and matches the form returned by their
// own "Clone with SSH" UI. Other inputs (already-SSH URLs,
// self-hosted hosts, unknown shapes) return unchanged — the caller
// gets exactly what the user wrote.
func rewriteToSSH(repoURL string) string {
	// Already SSH-shaped: nothing to do.
	if strings.HasPrefix(repoURL, "git@") || strings.HasPrefix(repoURL, "ssh://") {
		return repoURL
	}

	for _, host := range []string{"github.com", "gitlab.com", "bitbucket.org"} {
		prefix := "https://" + host + "/"
		if !strings.HasPrefix(repoURL, prefix) {
			continue
		}
		rest := strings.TrimPrefix(repoURL, prefix)
		rest = strings.TrimSuffix(rest, ".git")
		return "git@" + host + ":" + rest + ".git"
	}
	return repoURL
}
