package git

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"raioz/internal/domain/models"
	"raioz/internal/git/auth"
	"raioz/internal/i18n"
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
	gitArgs = append(gitArgs, "clone", "--depth", "1")
	if src.Branch != "" {
		gitArgs = append(gitArgs, "-b", src.Branch)
	}
	gitArgs = append(gitArgs, pr.URL, target)

	cmd := exec.CommandContext(ctx, "git", gitArgs...)
	// LC_ALL=C keeps git's messages in the one language runClone can
	// read; what the user is told comes from raioz, in theirs.
	cmd.Env = append(append(os.Environ(), pr.Env...), "LC_ALL=C")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd, pr.Cleanup, nil
}

// runClone runs a clone command built by newAuthenticatedCloneCmd and, when
// it fails for a reason the user can act on, says so: git's own message
// ("could not read Username", "exit status 128") names neither the yaml
// field to add nor the branch that is wrong.
func runClone(cmd *exec.Cmd, src models.SourceConfig) error {
	var stderr bytes.Buffer
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	err := cmd.Run()
	if err == nil {
		return nil
	}
	if reason := explainCloneFailure(stderr.String(), src); reason != "" {
		return fmt.Errorf("%s: %w", reason, err)
	}
	return err //nolint:wrapcheck // callers wrap it with their own context
}

// cloneAuthFailures are the lines git prints when the remote wants
// credentials it did not get.
var cloneAuthFailures = []string{
	"could not read Username",
	"could not read Password",
	"terminal prompts disabled",
	"Authentication failed",
	"Permission denied (publickey",
	"Could not read from remote repository",
	"Invalid username or",
}

// explainCloneFailure turns git's stderr into the sentence the user needs,
// "" when the failure is not one of the recognised kinds.
func explainCloneFailure(stderr string, src models.SourceConfig) string {
	if strings.Contains(stderr, "Remote branch") && strings.Contains(stderr, "not found") {
		return i18n.T("error.git_branch_missing", src.Branch, src.Repo)
	}
	for _, marker := range cloneAuthFailures {
		if !strings.Contains(stderr, marker) {
			continue
		}
		if src.Auth == "" {
			return i18n.T("error.git_clone_needs_auth", src.Repo)
		}
		hint := ""
		if provider, err := auth.ProviderFor(src.Auth); err == nil {
			hint = provider.SuggestSetup()
		}
		return i18n.T("error.git_clone_auth_failed", src.Repo, src.Auth, hint)
	}
	return ""
}
