package auth

import "context"

// inheritProvider is the escape hatch: raioz applies no auth
// hardening for this service, delegating fully to the dev's global
// git config (credential helper, ssh-agent, OS keychain, corporate
// Kerberos, smart card, …). Whatever `git clone <repo>` would do
// in the dev's shell is what raioz does here.
//
// Use when:
//   - your private repo lives behind a credential mechanism raioz
//     doesn't natively support (Kerberos, hardware tokens, …)
//   - you already have `git config credential.helper` set up
//     globally and want raioz to honor it
//
// Trade-off: less predictable across machines. A teammate with a
// different (or missing) credential helper sees different
// behavior. That's the point — it's an escape hatch, not a
// recommendation. See ADR-036 § auth providers for the policy.
type inheritProvider struct{}

func (i *inheritProvider) Name() string {
	return "inherit"
}

// Validate is a no-op: by definition we trust whatever the dev has
// configured globally. There's nothing to verify ahead of time.
func (i *inheritProvider) Validate(_ context.Context) error {
	return nil
}

// Prepare emits NO `-c credential.helper=` and NO `GIT_ASKPASS=` /
// `GIT_SSH_COMMAND=` overrides — the whole point of inherit is to
// leave the user's git config untouched. The one exception is
// `GIT_TERMINAL_PROMPT=0`: raioz runs git in a non-interactive
// context where a hung password prompt is worse than a fast
// failure. If the user's helper/agent works the prompt never
// triggers; if it doesn't, git exits with a clear "could not read
// password" rather than blocking forever.
func (i *inheritProvider) Prepare(_ context.Context, repoURL string) (PrepareResult, error) {
	return PrepareResult{
		GitArgs: nil,
		Env: []string{
			"GIT_TERMINAL_PROMPT=0",
		},
		URL:     repoURL,
		Cleanup: func() {},
	}, nil
}

func (i *inheritProvider) SuggestSetup() string {
	return "configure git globally (`git config credential.helper`, " +
		"ssh-agent, OS keychain, …) so `git clone <repo>` works from " +
		"your shell — raioz mirrors that environment"
}
