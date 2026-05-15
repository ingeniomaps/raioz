package auth

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// ghProvider delegates HTTPS credential resolution to the GitHub CLI
// (`gh`). The runtime contract piggybacks on `gh auth git-credential`,
// the same credential helper `gh auth setup-git` installs globally —
// we only scope it to this raioz invocation instead of mutating the
// user's git config.
//
// Why this approach:
//
//   - The token never appears on the command line (no `-c
//     http.extraheader=Authorization:...`); `ps`-visible secrets are
//     a non-starter.
//   - Token resolution stays inside the gh process, which already
//     handles re-auth, scope checks, and host-specific routing
//     (github.com vs ghe.example.com).
//   - When the user rotates / re-auths via gh, raioz picks it up on
//     the next invocation without any cache to invalidate.
//
// Trade-off: requires `gh` in PATH and `gh auth login` completed for
// the relevant host. Validate surfaces both gaps with a single hint
// pointing at SuggestSetup.
type ghProvider struct{}

// errGhNotInstalled / errGhNotLoggedIn keep the failure modes
// distinguishable for callers. Both produce the same user-visible
// hint via SuggestSetup, but tests assert on the specific cause.
var (
	errGhNotInstalled = errors.New(
		"auth: gh requires the GitHub CLI (`gh`) on PATH")
	errGhNotLoggedIn = errors.New(
		"auth: gh requires `gh auth login` to have completed")
)

// ghExecLookPath / ghExecCommand are seams for tests so we can fake
// the binary without messing with the runner's $PATH. Production
// points them at the real os/exec functions.
var (
	ghExecLookPath = exec.LookPath
	ghExecCommand  = exec.CommandContext
)

func (g *ghProvider) Name() string {
	return "gh"
}

// Validate fails fast when (a) gh is not in PATH or (b) the user is
// not logged in. The second check shells out to `gh auth status` —
// gh exits 1 with a clear message when no token is cached, so we
// don't need to parse the output.
func (g *ghProvider) Validate(ctx context.Context) error {
	if _, err := ghExecLookPath("gh"); err != nil {
		return fmt.Errorf("%w: %s", errGhNotInstalled, g.SuggestSetup())
	}
	// `gh auth status` exits 0 only when at least one host has a
	// valid cached token. Stdout/stderr inherit (silent on success;
	// gh prints diagnostic on failure which we let surface via the
	// wrapped error).
	cmd := ghExecCommand(ctx, "gh", "auth", "status")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", errGhNotLoggedIn, g.SuggestSetup())
	}
	return nil
}

// Prepare scopes the gh credential helper to this single git
// invocation. Precedence:
//
//   - `credential.helper=` (empty) strips whatever the user had
//     globally so we don't accidentally combine helpers.
//   - `credential.helper=!gh auth git-credential` registers gh as the
//     resolver for THIS git command only.
//
// GIT_TERMINAL_PROMPT=0 keeps git from blocking when the helper
// returns no credential (e.g. user is logged into github.com but
// hits a ghe.example.com URL gh doesn't know about). Fast failure
// beats a hung CI run.
func (g *ghProvider) Prepare(_ context.Context, repoURL string) (PrepareResult, error) {
	return PrepareResult{
		GitArgs: []string{
			"-c", "credential.helper=",
			"-c", "credential.helper=!gh auth git-credential",
		},
		Env: []string{
			"GIT_TERMINAL_PROMPT=0",
		},
		URL:     repoURL,
		Cleanup: func() {},
	}, nil
}

func (g *ghProvider) SuggestSetup() string {
	return "install the GitHub CLI (https://cli.github.com) and run " +
		"`gh auth login`; raioz invokes `gh auth git-credential` per-clone, " +
		"so no global git config change is required"
}
