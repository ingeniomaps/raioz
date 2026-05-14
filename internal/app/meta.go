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
// are reported but don't abort the meta run. activeProfiles narrows the
// iteration to projects that have no Profiles declared (always-on) or
// whose Profiles intersect the list.
func (m *MetaRunner) Up(
	ctx context.Context, cfg *config.MetaConfig,
	args, activeProfiles []string,
) MetaSummaryList {
	return m.run(ctx, cfg, "up", args, activeProfiles, false)
}

// Down runs `raioz down` in each sub-project in REVERSE order. Errors are
// always tolerated — teardown should be best-effort. Profiles are
// deliberately ignored here so a sub-project started under a different
// `--meta-profile` set still gets cleaned up; you can't strand a service
// you brought up earlier.
func (m *MetaRunner) Down(
	ctx context.Context, cfg *config.MetaConfig, args []string,
) MetaSummaryList {
	return m.run(ctx, cfg, "down", args, nil, true)
}

// Status runs `raioz status` in each sub-project, in order. Errors are
// tolerated so a single missing sub doesn't blank the rest of the report.
// Respects activeProfiles for symmetry with Up — the report shows what
// the matching `raioz up` would have started.
func (m *MetaRunner) Status(
	ctx context.Context, cfg *config.MetaConfig,
	args, activeProfiles []string,
) MetaSummaryList {
	return m.run(ctx, cfg, "status", args, activeProfiles, false)
}

// shouldIncludeMetaProject decides whether a project participates in this
// meta run given the active profile set. Empty Profiles = always-on
// regardless of the active list. Non-empty Profiles require at least one
// match. A nil/empty activeProfiles list keeps only the always-on
// projects.
func shouldIncludeMetaProject(p config.MetaProject, active []string) bool {
	if len(p.Profiles) == 0 {
		return true
	}
	for _, a := range active {
		for _, pp := range p.Profiles {
			if pp == a {
				return true
			}
		}
	}
	return false
}

func (m *MetaRunner) run(
	ctx context.Context, cfg *config.MetaConfig,
	subCmd string, extraArgs, activeProfiles []string, reverse bool,
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
	if len(activeProfiles) > 0 || !reverse {
		// Filter by profile for up/status. Down passes activeProfiles=nil
		// so this branch keeps the full list.
		filtered := projects[:0:0]
		for _, p := range projects {
			if shouldIncludeMetaProject(p, activeProfiles) {
				filtered = append(filtered, p)
			}
		}
		projects = filtered
	}
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
