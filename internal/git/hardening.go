package git

import (
	"context"
	"os"
	"os/exec"
)

// defaultHardenedCmd builds a `git` invocation with raioz's default
// auth hardening applied: credential helper disabled
// (`-c credential.helper=`), interactive prompts off
// (`GIT_TERMINAL_PROMPT=0`), no askpass program (`GIT_ASKPASS=`),
// and no custom SSH command (`GIT_SSH_COMMAND=`). The returned cmd
// streams to stdout/stderr.
//
// Public-only by design: with the helper disabled and no askpass,
// any private clone hangs the first credential prompt and we abort.
// This is the right default for the current "trust the user, public
// repos" posture, but it is the *exact* surface that issue 067 will
// extend with an `auth:` provider abstraction. Every clone-style
// call site funnels through this helper so swapping it for
// `provider.Prepare(...)` later is a one-place change.
func defaultHardenedCmd(ctx context.Context, args ...string) *exec.Cmd {
	hardened := append([]string{"-c", "credential.helper="}, args...)
	cmd := exec.CommandContext(ctx, "git", hardened...)
	cmd.Env = append(
		os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=", "GIT_SSH_COMMAND=",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}
