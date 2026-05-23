package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/config"
	"raioz/internal/host"
	"raioz/internal/i18n"
)

// runSingle invokes the raioz binary once for a single sub-project and
// returns the per-project summary entry. extraEnv is appended to the
// inherited process environment so callers can layer flags (router
// active, future per-call signals) without rewriting the whole env.
func (m *MetaRunner) runSingle(
	ctx context.Context, subCmd string,
	p config.MetaProject, extraArgs, extraEnv []string,
) MetaSummary {
	binary, err := m.resolveBinary()
	if err != nil {
		return MetaSummary{Project: p.Name, Path: p.Path, Err: err}
	}
	stdout := m.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := m.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	printMetaBanner(stdout, subCmd, p)

	// Per-sub timeout: hung sub-ups would otherwise pin
	// the whole meta workspace. RAIOZ_META_SUB_TIMEOUT (default 5m)
	// gives the operator a tunable cap. Timeout error distinguishes
	// "hung past deadline" from regular sub-process exit-non-zero.
	subCtx, cancel := context.WithTimeout(ctx, host.MetaSubTimeout())
	defer cancel()

	cmd := m.buildSubCmd(subCtx, binary, subCmd, p, extraArgs, extraEnv, stdout, stderr)
	runErr := cmd.Run()
	if subCtx.Err() == context.DeadlineExceeded {
		runErr = fmt.Errorf(
			"sub-project %q hung past RAIOZ_META_SUB_TIMEOUT=%s",
			p.Name, host.MetaSubTimeout())
	}
	return MetaSummary{Project: p.Name, Path: p.Path, Err: runErr}
}

// resolveBinary picks the raioz executable to invoke for a sub-project
// spawn. Resolution order:
//
//  1. m.Binary when set (tests inject a fake binary here).
//  2. Under `go test`, refuse to fall back further — os.Executable()
//     would point at the test runner and runSingle would recurse into
//     the suite. Callers must set m.Binary explicitly.
//  3. os.Executable() — the path the kernel sees for this process. Stable
//     under PATH changes and survives cwd switches.
//  4. filepath.Abs(os.Args[0]) as a last-resort fallback. Required because
//     runSingle sets cmd.Dir to the sub-project path before exec, which
//     turns a relative os.Args[0] (e.g. "./raioz" from a dev build) into
//     an unfindable path inside the sub-project dir.
func (m *MetaRunner) resolveBinary() (string, error) {
	if m.Binary != "" {
		return m.Binary, nil
	}
	if testing.Testing() {
		return "", fmt.Errorf(
			"MetaRunner.Binary must be set under go test; " +
				"os.Executable() points at the test runner")
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		return exe, nil
	}
	if len(os.Args) > 0 && os.Args[0] != "" {
		abs, err := filepath.Abs(os.Args[0])
		if err != nil {
			return "", fmt.Errorf("resolve raioz binary path: %w", err)
		}
		return abs, nil
	}
	return "", fmt.Errorf("cannot resolve raioz binary path for meta dispatch")
}

// buildSubCmd constructs the *exec.Cmd for a sub-project invocation.
// cmd.Cancel fires Kill only on DeadlineExceeded so the deferred
// subCancel in runSingle does not race against launcher grandchildren
// (issue 020-meta).
func (m *MetaRunner) buildSubCmd(
	ctx context.Context, binary, subCmd string, p config.MetaProject,
	extraArgs, extraEnv []string, stdout, stderr *os.File,
) *exec.Cmd {
	args := append([]string{subCmd}, extraArgs...)
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Cancel = func() error {
		if ctx.Err() == context.DeadlineExceeded {
			return cmd.Process.Kill()
		}
		return nil
	}
	cmd.Dir = p.Path
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	env := os.Environ()
	if len(extraEnv) > 0 {
		env = append(env, extraEnv...)
	}
	cmd.Env = env
	// Router + consumer subprocesses must die with the meta parent;
	// otherwise a SIGKILL leaves N raioz children each mid-`docker
	// compose up`, each still holding their own project locks.
	host.AttachPdeathsig(cmd)
	return cmd
}

func printMetaBanner(w *os.File, subCmd string, p config.MetaProject) {
	tag := strings.ToUpper(subCmd)
	if p.Optional {
		fmt.Fprintln(w, "\n"+i18n.T("meta.banner_optional", tag, p.Name))
	} else {
		fmt.Fprintln(w, "\n"+i18n.T("meta.banner", tag, p.Name))
	}
}
