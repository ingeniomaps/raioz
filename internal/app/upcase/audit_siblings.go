package upcase

import (
	"fmt"
	"sort"

	"raioz/internal/config"
	"raioz/internal/domain/models"
	"raioz/internal/output"
)

// auditSiblingYAMLs runs ADR-036 hygiene gates against every sibling
// dependency yaml reachable from deps (mode A `project:` and mode B
// `siblingProject:`). Returns the first failure so the operator can
// fix one yaml at a time. Called from upcase.Execute when
// Options.AuditSiblings is set.
//
// Scope is transitive: when a direct sibling itself declares
// sibling-deps, those nested yamls are also audited. A breadth-first
// walk keyed by absolute yaml path keeps each file from being audited
// twice and breaks cycles silently — every visited path is recorded
// in the printed summary so the operator confirms the actual scope.
//
// Direct siblings are enumerated in sorted dep-name order so the
// failure surface is deterministic across runs.
func auditSiblingYAMLs(deps *models.Deps) error {
	queue := seedAuditQueue(deps)
	visited := make(map[string]bool)
	var scanned []string

	for len(queue) > 0 {
		t := queue[0]
		queue = queue[1:]

		sib, err := config.ResolveSibling(t.dir)
		if err != nil {
			// A sibling that can't even be resolved would fail later
			// in the spawn path anyway; treat it as a preflight
			// failure here so the operator sees one message.
			return fmt.Errorf("audit-siblings: %s at %s: %w",
				t.label, t.dir, err)
		}

		if visited[sib.Path] {
			continue
		}
		visited[sib.Path] = true

		if err := config.AuditYAMLStrict(sib.Path); err != nil {
			return fmt.Errorf("audit-siblings: %s: %w", t.label, err)
		}
		scanned = append(scanned, sib.Path)

		// Walk the sibling's own deps for nested project: /
		// siblingProject: refs. LoadYAML normalizes those paths to
		// absolute (relative to the sibling's yaml dir).
		nested, err := config.LoadYAML(sib.Path)
		if err != nil {
			// AuditYAMLStrict already called LoadYAML successfully
			// above; a second call should not fail. If it does,
			// treat it as a hard failure rather than silently
			// dropping the sub-tree.
			return fmt.Errorf("audit-siblings: %s: reload for transitive scan: %w",
				t.label, err)
		}
		queue = append(queue, nestedAuditTargets(t.label, nested)...)
	}

	if len(scanned) == 0 {
		output.PrintInfo("audit-siblings: no sibling deps to scan")
	} else {
		output.PrintInfo(fmt.Sprintf(
			"audit-siblings: scanned %d yaml(s) — %v", len(scanned), scanned))
	}
	return nil
}

// auditTarget pairs an absolute sibling dir with a human-readable
// label tracking where it was reached from. The label is used in
// error messages: a nested sibling that fails the audit names its
// parent path so the operator can find the offending dep.
type auditTarget struct {
	label, dir string
}

// seedAuditQueue extracts the direct siblings of the consumer's
// dependencies, sorted by dep name for deterministic failure order.
func seedAuditQueue(deps *models.Deps) []auditTarget {
	var seed []auditTarget
	for name, entry := range deps.Infra {
		if entry.Inline == nil {
			continue
		}
		dir := siblingDirOf(*entry.Inline)
		if dir == "" {
			continue
		}
		seed = append(seed, auditTarget{
			label: fmt.Sprintf("dep %q", name),
			dir:   dir,
		})
	}
	sort.Slice(seed, func(i, j int) bool {
		return seed[i].label < seed[j].label
	})
	return seed
}

// nestedAuditTargets returns every sibling-dep declared in nested,
// each labelled with the parent path so error messages trace the
// chain. Sorted by dep name for stable order.
func nestedAuditTargets(parent string, nested *config.RaiozConfig) []auditTarget {
	var out []auditTarget
	names := make([]string, 0, len(nested.Deps))
	for name := range nested.Deps {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		dep := nested.Deps[name]
		dir := ""
		switch {
		case dep.Project != "":
			dir = dep.Project
		case dep.SiblingProject != "":
			dir = dep.SiblingProject
		}
		if dir == "" {
			continue
		}
		out = append(out, auditTarget{
			label: fmt.Sprintf("%s → dep %q", parent, name),
			dir:   dir,
		})
	}
	return out
}

// siblingDirOf returns the directory of the sibling raioz project
// referenced by the dependency, or "" when the dep has no sibling
// surface. Mode A's `project:` wins; Mode B's `siblingProject:`
// fills in when the dep also carries an Image fallback. Both fields
// hold the absolute directory of the sibling raioz.yaml after
// config loading normalizes paths.
func siblingDirOf(infra models.Infra) string {
	if infra.Project != "" {
		return infra.Project
	}
	if infra.SiblingProject != "" {
		return infra.SiblingProject
	}
	return ""
}
