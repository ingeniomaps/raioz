package upcase

import (
	"context"
	"fmt"

	"raioz/internal/config"
	"raioz/internal/docker"
	"raioz/internal/i18n"
	"raioz/internal/output"
)

// siblingDecisionKind classifies what the up-flow should do with a dep
// that uses one of the sibling-project fields from issue #26.
type siblingDecisionKind int

const (
	// siblingProceed: not a sibling dep, or mode B with the sibling
	// not active — run the normal image/compose dispatcher path.
	siblingProceed siblingDecisionKind = iota
	// siblingSkipDeferred: mode B and the sibling project is currently
	// active. Skip the local image dispatch AND remember the skip in
	// LocalState so the matching `down` doesn't try to tear down a
	// container we never created.
	siblingSkipDeferred
	// siblingSpawnModeA: mode A and the sibling project is NOT active
	// yet. The orchestrator must run `raioz up` recursively in the
	// sibling's cwd before continuing with the consumer.
	siblingSpawnModeA
	// siblingSkipModeA: mode A and the sibling is already active.
	// Nothing to do — the dep is implicitly satisfied.
	siblingSkipModeA
)

// siblingDecision is the verdict for one dep, including the human-
// readable reason consumed by the i18n log line.
type siblingDecision struct {
	Kind   siblingDecisionKind
	Reason string
	// SiblingName is the sibling project's `project:` field — used by
	// log messages and `requiredHostname` validation in Phase 7.
	SiblingName string
	// SiblingInfo is the resolved sibling, populated for mode A
	// verdicts so the spawner doesn't have to reload the raioz.yaml.
	SiblingInfo *config.SiblingInfo
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

	switch {
	case inline.Project != "":
		return decideModeA(ctx, depName, inline.Project,
			inline.RequiredHostname, consumerWorkspace)
	case inline.SiblingProject != "":
		return decideModeB(ctx, depName, inline.SiblingProject,
			inline.RequiredHostname, consumerWorkspace)
	default:
		return siblingDecision{Kind: siblingProceed}, nil
	}
}

// decideModeA resolves a `project:` dep, checks for a recursion cycle
// against RAIOZ_SIBLING_STACK, and probes whether the sibling is
// already up. Returns a spawn verdict when the sibling needs to be
// brought up, or a skip verdict when it's already running.
func decideModeA(
	ctx context.Context,
	depName string,
	projectPath string,
	requiredHostname string,
	consumerWorkspace string,
) (siblingDecision, error) {
	sib, err := config.ResolveSibling(projectPath)
	if err != nil {
		return siblingDecision{}, fmt.Errorf(
			"resolve sibling for dep %q: %w", depName, err)
	}
	if err := config.ValidateSiblingWorkspace(consumerWorkspace, sib); err != nil {
		return siblingDecision{}, fmt.Errorf("dep %q: %w", depName, err)
	}
	if err := validateRequiredHostname(depName, requiredHostname, sib); err != nil {
		return siblingDecision{}, err
	}
	if err := checkSiblingCycle(depName, sib); err != nil {
		return siblingDecision{}, err
	}

	active, err := docker.IsProjectActive(ctx, sib.Workspace, sib.Project)
	if err != nil {
		return siblingDecision{}, fmt.Errorf(
			"probe sibling for dep %q: %w", depName, err)
	}
	if active {
		return siblingDecision{
			Kind:        siblingSkipModeA,
			SiblingName: sib.Project,
			SiblingInfo: sib,
			Reason: fmt.Sprintf(
				"sibling project %q already up — no spawn needed",
				sib.Project),
		}, nil
	}
	return siblingDecision{
		Kind:        siblingSpawnModeA,
		SiblingName: sib.Project,
		SiblingInfo: sib,
		Reason: fmt.Sprintf(
			"spawning `raioz up` in %s for dep %q", sib.Dir, depName),
	}, nil
}

// decideModeB resolves a `siblingProject:` dep and probes whether the
// sibling is currently up. When the sibling is active, returns the
// skip-deferred verdict so the local image is bypassed; otherwise the
// caller runs the normal image dispatcher.
//
// requiredHostname is checked against the sibling's declared hostnames
// only when we'd actually defer to it — when falling back to the
// image, the requirement does not apply because the local image has
// its own hostnames.
func decideModeB(
	ctx context.Context,
	depName string,
	siblingPath string,
	requiredHostname string,
	consumerWorkspace string,
) (siblingDecision, error) {
	sib, err := config.ResolveSibling(siblingPath)
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
		if err := validateRequiredHostname(depName, requiredHostname, sib); err != nil {
			return siblingDecision{}, err
		}
		return siblingDecision{
			Kind:        siblingSkipDeferred,
			SiblingName: sib.Project,
			SiblingInfo: sib,
			Reason: fmt.Sprintf(
				"sibling project %q is active — using its container",
				sib.Project),
		}, nil
	}
	// Sibling not active → fall through to the normal image runner.
	// LocalState.DeferredToSibling is overwritten on every up, so a
	// previously-deferred entry for this dep clears itself.
	return siblingDecision{Kind: siblingProceed}, nil
}

// resolveSiblingVerdicts runs decideSibling for every infra dep up front
// so the orchestrator knows — before printing "Starting infrastructure
// (N)" — how many deps will actually be dispatched. Without the pre-pass,
// N includes sibling-deferred deps that never reach a runner. As a side
// benefit, sibling config errors (workspace mismatch, missing raioz.yaml,
// cycles) surface before any container starts.
func resolveSiblingVerdicts(
	ctx context.Context,
	infraNames []string,
	deps *config.Deps,
) (map[string]siblingDecision, int, error) {
	verdicts := make(map[string]siblingDecision, len(infraNames))
	toDispatch := 0
	for _, name := range infraNames {
		v, err := decideSibling(
			ctx, name, deps.Infra[name].Inline, deps.Workspace)
		if err != nil {
			return nil, 0, err
		}
		verdicts[name] = v
		if v.Kind == siblingProceed {
			toDispatch++
		}
	}
	return verdicts, toDispatch, nil
}

// applySiblingVerdict performs the side effects implied by a verdict:
// removes the dep from `detections` (sibling-mode deps have no
// container in this project's namespace, so downstream consumers like
// buildEndpoints / startProxy / checkInfraHealth must skip them),
// spawns recursive raioz up when mode A demands it, and tracks
// deferred mode B deps for the matching down. Returns true when the
// orchestrator should skip the regular dispatcher path for this dep.
//
// Pass deferredDeps as a pointer so we can append in place without
// returning a new slice every iteration of the orchestration loop.
func applySiblingVerdict(
	ctx context.Context,
	depName string,
	verdict siblingDecision,
	projectDir string,
	detections DetectionMap,
	deferredDeps *[]string,
) (skip bool, err error) {
	switch verdict.Kind {
	case siblingSpawnModeA:
		delete(detections, depName)
		if err := spawnSibling(ctx, projectDir, depName, verdict.SiblingInfo); err != nil {
			return false, err
		}
		return true, nil
	case siblingSkipModeA:
		delete(detections, depName)
		output.PrintProgress(
			i18n.T("up.sibling_modea_already_up", depName, verdict.SiblingName))
		return true, nil
	case siblingSkipDeferred:
		delete(detections, depName)
		output.PrintProgress(
			i18n.T("up.sibling_dep_skipped", depName, verdict.Reason))
		*deferredDeps = append(*deferredDeps, depName)
		return true, nil
	}
	return false, nil
}

// validateRequiredHostname errors when the consumer asked for a
// specific hostname (`requiredHostname:` on the dep) and the sibling's
// raioz.yaml does not declare it on any service or routed dep. Empty
// requirement is a no-op.
//
// We trust the sibling's declared hostnames — verifying against the
// live Caddyfile is more thorough but requires the proxy to be raioz-
// managed and reachable, which `raioz check` already covers.
func validateRequiredHostname(
	depName string,
	host string,
	sib *config.SiblingInfo,
) error {
	if host == "" {
		return nil
	}
	if sib.SiblingHasHostname(host) {
		return nil
	}
	return fmt.Errorf(
		"dep %q: sibling project %q at %s does not declare hostname %q — "+
			"add `hostname: %s` to a service in its raioz.yaml, or drop "+
			"`requiredHostname:` from this consumer (declared hostnames: %v)",
		depName, sib.Project, sib.Path, host, host, sib.Hostnames)
}
