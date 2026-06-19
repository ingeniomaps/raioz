// Package auth defines the auth strategy used by raioz when cloning
// git repositories declared in raioz.yaml.
//
// The default (no `auth:` declared in yaml) is the strict provider:
// public-only clones with credential helper disabled, no askpass, no
// custom SSH command. Other providers — implemented in subsequent
// commits — opt the dev into:
//
//   - inherit: trust the user's existing git config (credential
//     helper, ssh-agent, etc.)
//   - gh: pull a one-shot token from the gh CLI's cache
//   - ssh: rewrite the URL to SSH form and trust ssh-agent
//
// ADR-036 owns the policy that secrets never live in the yaml; this
// package keeps that invariant by treating `auth:` as a *selector*
// rather than a credential carrier.
package auth

import "context"

// Provider declares an auth strategy. Each one decides three things:
// what the dev's environment needs (Validate), how to turn a repo
// URL into a runnable git invocation (Prepare), and what to tell the
// dev when their environment isn't ready (SuggestSetup).
type Provider interface {
	// Name is the canonical identifier ("", "inherit", "gh", "ssh").
	// The empty string means "strict / default" — public-only.
	Name() string

	// Validate is the preflight check, called from `raioz up` before
	// the first clone is attempted. Returns nil when the dev's
	// environment can satisfy this provider; otherwise an error whose
	// message points at the missing piece (and ideally references
	// SuggestSetup).
	Validate(ctx context.Context) error

	// Prepare gathers everything one git invocation needs from this
	// provider. Callers must:
	//
	//   - prepend GitArgs to whatever args they were going to pass
	//     (so "-c credential.helper=" appears before "clone").
	//   - append Env to os.Environ() before exec.
	//   - use URL instead of the original repoURL.
	//   - call Cleanup after the git command returns (always
	//     non-nil; safe to defer immediately after Prepare).
	Prepare(ctx context.Context, repoURL string) (PrepareResult, error)

	// SuggestSetup returns a short, human-readable hint for how to
	// configure this provider. Surfaced inside Validate's error
	// message so failures are actionable.
	SuggestSetup() string
}

// PrepareResult bundles everything one git invocation needs from a
// Provider. Providers that don't need a particular field leave it
// zero / empty (Cleanup must always be non-nil).
type PrepareResult struct {
	// GitArgs is prepended to the rest of the git args.
	GitArgs []string

	// Env is APPENDED to os.Environ() — does not replace it.
	Env []string

	// URL is the (possibly rewritten) repo URL to pass to git.
	URL string

	// Cleanup runs after the git command returns. Always non-nil;
	// no-op for providers without state to release.
	Cleanup func()
}
