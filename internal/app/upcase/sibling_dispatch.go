package upcase

import (
	"context"
	"fmt"

	"raioz/internal/config"
	"raioz/internal/docker"
)

// siblingDecisionKind classifies what the up-flow should do with a dep
// that uses one of the sibling-project fields from issue #26.
type siblingDecisionKind int

const (
	// siblingProceed: not a sibling dep, or sibling not active — run
	// the normal image/compose dispatcher path.
	siblingProceed siblingDecisionKind = iota
	// siblingSkipDeferred: sibling project is currently active in the
	// workspace; skip the local image dispatch AND remember the skip
	// in LocalState so the matching `down` doesn't try to tear down a
	// container we never created.
	siblingSkipDeferred
	// siblingErrorModeA: dep uses `project:` (mode A) which needs a
	// recursive `raioz up` spawn. That ships in a follow-up; for now
	// we surface a clear error rather than silently proceed.
	siblingErrorModeA
)

// siblingDecision is the verdict for one dep, including the human-
// readable reason consumed by the i18n log line.
type siblingDecision struct {
	Kind   siblingDecisionKind
	Reason string
	// SiblingName is the sibling project's `project:` field — used by
	// log messages and `requiredHostname` validation in Phase 7.
	SiblingName string
}

// decideSibling consults the sibling-project fields of `inline` against
// the live docker state and returns whether the orchestration loop
// should skip, error out, or proceed for this dep. Pure with respect
// to the codebase: the only side effects are an os.Stat on the sibling
// raioz.yaml plus a docker ps probe.
//
// Mode A (`project:`) is recognized but rejected with a structured
// error — the recursive `raioz up` spawn lands in a later phase. This
// keeps the YAML validator from being a liar (Phase 1 accepts the
// field) without silently doing the wrong thing.
func decideSibling(
	ctx context.Context,
	depName string,
	inline *config.Infra,
	consumerWorkspace string,
) (siblingDecision, error) {
	if inline == nil {
		return siblingDecision{Kind: siblingProceed}, nil
	}

	if inline.Project != "" {
		return siblingDecision{
			Kind: siblingErrorModeA,
			Reason: fmt.Sprintf(
				"dep %q declares 'project: %s' (mode A) which needs a "+
					"recursive raioz up — not yet wired. Use 'siblingProject:' "+
					"with an image fallback for now, or run 'raioz up' in %s "+
					"yourself before this project",
				depName, inline.Project, inline.Project),
		}, nil
	}

	if inline.SiblingProject == "" {
		return siblingDecision{Kind: siblingProceed}, nil
	}

	sib, err := config.ResolveSibling(inline.SiblingProject)
	if err != nil {
		return siblingDecision{}, fmt.Errorf(
			"resolve sibling for dep %q: %w", depName, err)
	}
	if err := config.ValidateSiblingWorkspace(consumerWorkspace, sib); err != nil {
		return siblingDecision{}, fmt.Errorf("dep %q: %w", depName, err)
	}

	active, err := docker.IsProjectActive(ctx, sib.Workspace, sib.Project)
	if err != nil {
		return siblingDecision{}, fmt.Errorf(
			"probe sibling for dep %q: %w", depName, err)
	}
	if active {
		return siblingDecision{
			Kind:        siblingSkipDeferred,
			SiblingName: sib.Project,
			Reason: fmt.Sprintf(
				"sibling project %q is active — using its container",
				sib.Project),
		}, nil
	}

	// Sibling not active → fall through to the normal image/compose
	// runner; Phase 4's ClearDeferred is implicitly handled by the
	// orchestrator overwriting LocalState.DeferredToSibling each up.
	return siblingDecision{Kind: siblingProceed}, nil
}
