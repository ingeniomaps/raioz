package upcase

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"

	"raioz/internal/config"
	"raioz/internal/host"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// siblingStackEnv carries the call-chain of recursive `raioz up`
// invocations across a Mode A spawn. The parent appends its own
// project dir before exec; the child reads the variable in
// checkSiblingCycle to fail fast on A → B → A loops instead of running
// forever.
const siblingStackEnv = "RAIOZ_SIBLING_STACK"

// readSiblingStack returns the absolute project directories already on
// the recursive-up call chain. Empty when this raioz was invoked
// directly by the user.
func readSiblingStack() []string {
	raw := os.Getenv(siblingStackEnv)
	if raw == "" {
		return nil
	}
	return strings.Split(raw, string(os.PathListSeparator))
}

// pushSiblingStack returns the env-var line to attach to a spawned
// child raioz, appending the consumer's project dir AND the sibling
// target dir to whatever the current invocation inherited. Pushing
// both keeps the chain self-describing inside the child: when the
// child runs its own cycle check, the error message can render the
// full path (parent → … → child → target) instead of an ambiguous
// pair.
func pushSiblingStack(consumerDir, siblingDir string) string {
	cur := append(readSiblingStack(), consumerDir, siblingDir)
	return siblingStackEnv + "=" + strings.Join(cur, string(os.PathListSeparator))
}

// checkSiblingCycle returns an error when sib.Dir is already on the
// recursive-up stack — i.e. spawning `raioz up` for sib would recurse
// into a project we're already in the middle of upping. The chain is
// included in the message so the user can see exactly which loop they
// configured.
func checkSiblingCycle(depName string, sib *config.SiblingInfo) error {
	stack := readSiblingStack()
	if !slices.Contains(stack, sib.Dir) {
		return nil
	}
	chain := strings.Join(append(stack, sib.Dir), " → ")
	return fmt.Errorf(
		"sibling cycle: dep %q points back at %s which is already in the "+
			"recursive-up chain (%s) — break the cycle by removing one of "+
			"the `project:` declarations or use `siblingProject:` (mode B) "+
			"on one side instead",
		depName, sib.Dir, chain)
}

// spawnRaiozBinary returns the path to the raioz executable that
// should be used for recursive Mode A spawns. Override is honored for
// tests; otherwise falls back to os.Executable() so the child runs the
// exact same binary as the parent.
var spawnRaiozBinary = func() (string, error) {
	return os.Executable()
}

// spawnSibling runs `raioz up` in sib.Dir and streams its stdout +
// stderr back to the parent prefixed with `[sibling: depName] `. The
// consumer's project dir is pushed onto RAIOZ_SIBLING_STACK so the
// child can detect cycles.
//
// Returns an error including the sibling cwd so the user can copy-
// paste a `cd <dir> && raioz up` to diagnose without having to recall
// where the spawn came from.
func spawnSibling(
	ctx context.Context,
	consumerDir string,
	depName string,
	sib *config.SiblingInfo,
) error {
	bin, err := spawnRaiozBinary()
	if err != nil {
		return fmt.Errorf("locate raioz binary for sibling spawn: %w", err)
	}

	output.PrintProgress(i18n.T("up.sibling_spawn_starting", depName, sib.Dir))

	// Cap the wait so a hung sibling can't block the parent forever.
	timeout := host.SiblingSpawnTimeout()
	childCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(childCtx, bin, "up")
	cmd.Dir = sib.Dir
	cmd.Env = append(os.Environ(), pushSiblingStack(consumerDir, sib.Dir))
	// Propagate the audit/log correlation ID so the child's events
	// share the same value as the parent's — grep on correlation_id
	// reconstructs the whole spawn tree (ADR-024).
	if cid := logging.GetRequestID(ctx); cid != "" {
		cmd.Env = append(cmd.Env, logging.CorrelationIDEnv+"="+cid)
	}
	// ADR-026: Pdeathsig on Linux so a Ctrl+C on the
	// parent reaps spawned siblings instead of orphaning them. No-op
	// on macOS/Windows; cmd.Context() cancellation covers the
	// portable half.
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	setPdeathsig(cmd.SysProcAttr)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("pipe stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("pipe stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf(
			"start `raioz up` in %s for sibling dep %q: %w",
			sib.Dir, depName, err)
	}

	prefix := "[sibling: " + depName + "] "
	go streamPrefixed(stdout, prefix)
	go streamPrefixed(stderr, prefix)

	if err := cmd.Wait(); err != nil {
		if childCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf(
				"sibling project %q timed out after %s — set "+
					"RAIOZ_SIBLING_TIMEOUT higher, or investigate the "+
					"hang in %s",
				depName, timeout, sib.Dir)
		}
		return fmt.Errorf(
			"sibling project %q failed to come up — run "+
				"`cd %s && raioz up` to diagnose: %w",
			depName, sib.Dir, err)
	}
	output.PrintProgressDone(i18n.T("up.sibling_spawn_done", depName))
	return nil
}

// streamPrefixed reads lines from r and forwards each one to PrintInfo
// with prefix prepended, until r is closed. Used by spawnSibling to
// keep the recursive raioz output legible.
func streamPrefixed(r io.ReadCloser, prefix string) {
	defer func() { _ = r.Close() }()
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		output.PrintInfo(prefix + sc.Text())
	}
}
