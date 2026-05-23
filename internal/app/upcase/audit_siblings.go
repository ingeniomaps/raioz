package upcase

import (
	"fmt"
	"sort"

	"raioz/internal/config"
	"raioz/internal/domain/models"
	"raioz/internal/output"
)

// auditSiblingYAMLs runs ADR-036 hygiene gates against every sibling
// yaml reachable from deps, transitively. Breadth-first by absolute
// yaml path so each file is audited once and cycles terminate. The
// printed summary names every visited file. Returns the first
// failure (deterministic order: sorted by dep name).
func auditSiblingYAMLs(deps *models.Deps) error {
	queue := seedAuditQueue(deps)
	visited := make(map[string]bool)
	var scanned []string

	for len(queue) > 0 {
		t := queue[0]
		queue = queue[1:]

		sib, err := config.ResolveSibling(t.dir)
		if err != nil {
			// Surface the resolve failure here so the operator gets
			// one preflight error instead of a later spawn failure.
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

		// LoadYAML normalises nested project: / siblingProject: to
		// absolute paths so the BFS can enqueue them directly.
		// AuditYAMLStrict already parsed sib.Path successfully so
		// this re-read should not fail; if it does, hard-fail rather
		// than silently drop the sub-tree.
		nested, err := config.LoadYAML(sib.Path)
		if err != nil {
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

// auditTarget pairs a sibling dir with a label naming the chain that
// reached it; the label feeds error messages so a nested failure
// traces back to the offending dep.
type auditTarget struct {
	label, dir string
}

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

// siblingDirOf returns the absolute sibling-project dir referenced by
// a dep (mode A `project:` first, then mode B `siblingProject:`), or
// "" for deps without a sibling surface.
func siblingDirOf(infra models.Infra) string {
	if infra.Project != "" {
		return infra.Project
	}
	if infra.SiblingProject != "" {
		return infra.SiblingProject
	}
	return ""
}
