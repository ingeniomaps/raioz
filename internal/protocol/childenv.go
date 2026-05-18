// Package protocol holds the env-var contract between a raioz parent
// and the raioz children it spawns. Each constant here is a parent →
// child signal — defining them in one place keeps producer and
// consumer from drifting silently.
package protocol

import "os"

// RouterActive is set by the meta runner on consumer sub-up invocations
// when a router project is in charge of edge routing for the workspace
// (ADR-037). The consumer-side upcase reads it to suppress the bundled
// Caddy. Truthy values: "1", "true", "yes" (case-insensitive).
const RouterActive = "RAIOZ_ROUTER_ACTIVE"

// SiblingStack carries the call-chain of recursive `raioz up`
// invocations across a mode-A spawn (ADR-008). The parent appends its
// own project dir before exec; the child reads it to fail fast on
// A → B → A cycles and to bypass the project-lock acquisition that
// the parent already holds.
const SiblingStack = "RAIOZ_SIBLING_STACK"

// CorrelationID carries the request/correlation ID across recursive
// `raioz` invocations (mode A sibling spawn, ADR-024). The parent
// stamps its own ID into this env var when spawning a child so audit
// and log records share the value across the whole spawn tree.
// `internal/logging.CorrelationIDEnv` is a backwards-compat alias for
// pre-protocol callers; new code should import this constant directly.
const CorrelationID = "RAIOZ_CORRELATION_ID"

// RouterAssignedIP carries the bundled-Caddy convention IP that raioz
// would otherwise assign for the workspace's edge proxy. The meta
// runner sets it on the router project's sub-up (ADR-037 swap-in)
// so the router can bind the same IP and the operator's /etc/hosts /
// proxy.publish:false setup keeps working unchanged when alternating
// between the bundled Caddy and the router project. The router yaml
// is responsible for consuming this env var (e.g. via a template or
// the dep's image entrypoint that reads $RAIOZ_ROUTER_ASSIGNED_IP and
// passes it to `--ip` / network.assignedIP). Empty when raioz cannot
// derive the IP (no network.subnet declared).
const RouterAssignedIP = "RAIOZ_ROUTER_ASSIGNED_IP"

// IsRecursiveSiblingSpawn reports whether the current process is a
// mode-A recursive `raioz up` child (ADR-008). True when the parent
// stamped SiblingStack in the child's env. Lock acquirers consult
// this to skip workspace lock acquisition — the parent already
// holds it and re-acquiring would deadlock. Centralised so every
// lock site shares one truth (the upcase.acquireLock branch and
// the app.acquireWorkspaceMutatorLock branch must agree; before
// this helper they drifted).
func IsRecursiveSiblingSpawn() bool {
	return os.Getenv(SiblingStack) != ""
}

// MetaCompletedProjects carries the names of sub-projects the meta
// runner has already brought up successfully in the current run.
// Comma-separated. The meta runner appends to it after each
// successful sub-up and stamps the result into the env of every
// subsequent sub-up. Sub-up's mode-A sibling dispatch reads it as a
// short-circuit: when a dep's sibling project is on the list, skip
// the IsProjectActive probe and the sibling spawn entirely. The
// container the meta already kicked off may still be initialising
// (launcher pattern: `make start` spawning `docker compose up -d` in
// the background) when the next sub-up runs — without this hint the
// probe sees no container and triggers a redundant recursive `raioz
// up`. Disjoint from SiblingStack: that one's about cycle detection
// in the recursive call chain; this one's about "the meta already
// owns this".
const MetaCompletedProjects = "RAIOZ_META_COMPLETED_PROJECTS"
