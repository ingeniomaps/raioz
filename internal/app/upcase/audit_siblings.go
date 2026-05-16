package upcase

import (
	"fmt"
	"sort"

	"raioz/internal/config"
	"raioz/internal/domain/models"
)

// auditSiblingYAMLs runs ADR-036 hygiene gates against every sibling
// dependency yaml referenced from deps (mode A `project:` and mode B
// `siblingProject:`). Returns the first failure so the operator can
// fix one yaml at a time. Called from upcase.Execute when
// Options.AuditSiblings is set.
//
// Order is deterministic (sorted dep name) so the same yaml always
// trips first across runs.
func auditSiblingYAMLs(deps *models.Deps) error {
	type target struct {
		dep, dir string
	}
	var targets []target
	for name, entry := range deps.Infra {
		if entry.Inline == nil {
			continue
		}
		dir := siblingDirOf(*entry.Inline)
		if dir == "" {
			continue
		}
		targets = append(targets, target{dep: name, dir: dir})
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].dep < targets[j].dep
	})
	for _, t := range targets {
		sib, err := config.ResolveSibling(t.dir)
		if err != nil {
			// A sibling that can't even be resolved would fail later
			// in the spawn path anyway; treat it as a preflight
			// failure here so the operator sees one message.
			return fmt.Errorf("audit-siblings: dep %q at %s: %w",
				t.dep, t.dir, err)
		}
		if err := config.AuditYAMLStrict(sib.Path); err != nil {
			return fmt.Errorf("audit-siblings: dep %q: %w", t.dep, err)
		}
	}
	return nil
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
