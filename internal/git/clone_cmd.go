package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"raioz/internal/domain/models"
	"raioz/internal/git/auth"
)

// newAuthenticatedCloneCmd builds the shallow-clone command for src
// honoring the auth provider declared on `src.Auth`. The default
// (empty `Auth`) is the strict provider — reproduces the v0.1
// hardening (credential.helper disabled, no askpass, no SSH
// command, no interactive prompts) so existing public-repo flows
// are bit-for-bit unchanged.
//
// The returned cleanup is always non-nil; defer it immediately
// after the successful return so provider state (e.g. gh's tempfile
// token) is released even if the clone errors.
//
// URL substitution: the provider may rewrite src.Repo (e.g. ssh
// provider in fase 3 turns `github.com/foo/bar` into
// `git@github.com:foo/bar.git`); the returned command uses the
// rewritten value, not the raw src.Repo.
func newAuthenticatedCloneCmd(
	ctx context.Context, src models.SourceConfig, target string,
) (*exec.Cmd, func(), error) {
	provider, err := auth.ProviderFor(src.Auth)
	if err != nil {
		return nil, nil, fmt.Errorf("auth provider %q: %w", src.Auth, err)
	}
	pr, err := provider.Prepare(ctx, src.Repo)
	if err != nil {
		return nil, nil, fmt.Errorf("auth %q prepare: %w", provider.Name(), err)
	}

	gitArgs := append([]string{}, pr.GitArgs...)
	gitArgs = append(gitArgs, "clone", "--depth", "1", "-b", src.Branch, pr.URL, target)

	cmd := exec.CommandContext(ctx, "git", gitArgs...)
	cmd.Env = append(os.Environ(), pr.Env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd, pr.Cleanup, nil
}
