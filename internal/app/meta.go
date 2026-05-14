package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"raioz/internal/config"
	"raioz/internal/output"
)

// MetaRunner orchestrates a meta-orchestrator config: a raioz.yaml with
// `kind: meta` whose sole job is to delegate up/down/status to N sub-projects
// in order. Each sub-project keeps its own raioz.yaml, .raioz.state.json, and
// lifecycle.
//
// The implementation deliberately shells out to the current binary
// (os.Args[0]) instead of re-using the in-process use cases. Two reasons:
//   - Each sub-project gets a clean process with its own config loader,
//     i18n state, and naming prefix. No global-state contamination.
//   - Failure isolation is automatic: a panic / fatal in one sub doesn't
//     drag the meta runner down.
type MetaRunner struct {
	// Binary is the raioz executable to invoke for sub-projects. Defaults
	// to os.Args[0] when empty. Tests inject a fake here.
	Binary string
	// Stdout / Stderr default to os.Stdout / os.Stderr. Tests inject buffers.
	Stdout, Stderr *os.File
}

// MetaSummary describes the outcome of a meta run, one entry per sub-project.
type MetaSummary struct {
	Project string
	Path    string
	Err     error // nil on success
	Skipped bool  // true when an optional sub failed and was tolerated
}

// MetaSummaryList is a typed slice so HasFailures can hang off a method.
// Defining it lets callers write `summary.HasFailures()` instead of needing
// a free function.
type MetaSummaryList []MetaSummary

// HasFailures reports whether any non-optional sub failed. Used as the
// command-level exit status.
func (s MetaSummaryList) HasFailures() bool {
	for _, e := range s {
		if e.Err != nil && !e.Skipped {
			return true
		}
	}
	return false
}

// Up runs `raioz up` in each sub-project, in order. Optional subs that fail
// are reported but don't abort the meta run.
func (m *MetaRunner) Up(ctx context.Context, cfg *config.MetaConfig, args []string) MetaSummaryList {
	return m.run(ctx, cfg, "up", args, false)
}

// Down runs `raioz down` in each sub-project in REVERSE order. Errors are
// always tolerated — teardown should be best-effort.
func (m *MetaRunner) Down(ctx context.Context, cfg *config.MetaConfig, args []string) MetaSummaryList {
	return m.run(ctx, cfg, "down", args, true)
}

// Status runs `raioz status` in each sub-project, in order. Errors are
// tolerated so a single missing sub doesn't blank the rest of the report.
func (m *MetaRunner) Status(ctx context.Context, cfg *config.MetaConfig, args []string) MetaSummaryList {
	return m.run(ctx, cfg, "status", args, false)
}

func (m *MetaRunner) run(
	ctx context.Context, cfg *config.MetaConfig,
	subCmd string, extraArgs []string, reverse bool,
) MetaSummaryList {
	binary := m.Binary
	if binary == "" {
		binary = os.Args[0]
	}
	stdout := m.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := m.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	projects := cfg.Projects
	if reverse {
		projects = reverseMetaProjects(projects)
	}

	results := make(MetaSummaryList, 0, len(projects))
	for _, p := range projects {
		printMetaBanner(stdout, subCmd, p)

		args := append([]string{subCmd}, extraArgs...)
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir = p.Path
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		// Pass through environment so workspace lookups, paths, etc. work.
		cmd.Env = os.Environ()

		err := cmd.Run()
		entry := MetaSummary{Project: p.Name, Path: p.Path, Err: err}
		switch {
		case err == nil:
			// success
		case p.Optional && subCmd == "up":
			entry.Skipped = true
			output.PrintWarning(fmt.Sprintf(
				"meta: optional project %q failed (%s) — continuing", p.Name, err,
			))
		case subCmd == "down" || subCmd == "status":
			// Best-effort: keep going on remaining subs even if this one
			// errored. The error is recorded in the summary.
			output.PrintWarning(fmt.Sprintf(
				"meta: %s for %q returned %s — continuing", subCmd, p.Name, err,
			))
		default:
			results = append(results, entry)
			return results // hard fail on first non-optional up failure
		}
		results = append(results, entry)
	}
	return results
}

func reverseMetaProjects(in []config.MetaProject) []config.MetaProject {
	out := make([]config.MetaProject, len(in))
	for i, p := range in {
		out[len(in)-1-i] = p
	}
	return out
}

func printMetaBanner(w *os.File, subCmd string, p config.MetaProject) {
	tag := strings.ToUpper(subCmd)
	if p.Optional {
		fmt.Fprintf(w, "\n=== [%s] %s (optional) ===\n", tag, p.Name)
	} else {
		fmt.Fprintf(w, "\n=== [%s] %s ===\n", tag, p.Name)
	}
}
