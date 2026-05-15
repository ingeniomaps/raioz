package auth

import (
	"context"
	"errors"
)

// ghProvider is a placeholder for the GitHub CLI integration that
// lands in fase 2 of issue 067. Both Validate and Prepare return a
// "not implemented" error pointing the dev at `auth: inherit` as
// the working alternative.
//
// The placeholder exists in fase 1 so the schema enum
// {"", inherit, gh, ssh} stabilizes BEFORE the runtime ships: the
// validator (commit 6) and the corpus can reference `gh` now, and
// when fase 2 lands it is a single-file change.
type ghProvider struct{}

// errGhNotImplemented is the sentinel returned by both Validate and
// Prepare so callers can detect placeholder state via errors.Is.
var errGhNotImplemented = errors.New(
	"auth: gh is not yet implemented (issue 067 fase 2). " +
		"Use `auth: inherit` to delegate to your global git config")

func (g *ghProvider) Name() string {
	return "gh"
}

func (g *ghProvider) Validate(_ context.Context) error {
	return errGhNotImplemented
}

func (g *ghProvider) Prepare(_ context.Context, _ string) (PrepareResult, error) {
	return PrepareResult{}, errGhNotImplemented
}

func (g *ghProvider) SuggestSetup() string {
	return "auth: gh lands in fase 2 of issue 067. Until then, use " +
		"`auth: inherit` and configure gh's git credential helper " +
		"globally (`gh auth setup-git`)"
}
