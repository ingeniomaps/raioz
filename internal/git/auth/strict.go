package auth

import "context"

// strictProvider is the default auth strategy used when raioz.yaml
// does not declare `services.<n>.auth`. It reproduces the hardening
// raioz has applied since v0.1: credential helper disabled,
// terminal prompts off, no askpass, no custom SSH command. Public
// repos clone successfully; private repos fail fast.
//
// The Prepare output mirrors the `defaultHardenedCmd` helper from
// commit 1 of issue 067 — when commit 5 wires the provider into the
// clone sites, this is the path the default takes.
type strictProvider struct{}

// Name returns the empty string by convention: strict is the
// default that callers select by omitting `auth:` in the yaml.
func (s *strictProvider) Name() string {
	return ""
}

// Validate is a no-op: strict requires nothing from the dev's
// environment beyond a working git binary, which is already a
// requirement raioz checks elsewhere (preflight).
func (s *strictProvider) Validate(_ context.Context) error {
	return nil
}

// Prepare emits the hardening flags + env vars. URL passes through
// unchanged.
func (s *strictProvider) Prepare(_ context.Context, repoURL string) (PrepareResult, error) {
	return PrepareResult{
		GitArgs: []string{"-c", "credential.helper="},
		Env: []string{
			"GIT_TERMINAL_PROMPT=0",
			"GIT_ASKPASS=",
			"GIT_SSH_COMMAND=",
		},
		URL:     repoURL,
		Cleanup: func() {},
	}, nil
}

// SuggestSetup is intentionally short — there's nothing to set up
// for the default path.
func (s *strictProvider) SuggestSetup() string {
	return "no setup required (default; only public repos clone)"
}
