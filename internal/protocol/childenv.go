// Package protocol holds the env-var contract between a raioz parent
// and the raioz children it spawns. Each constant here is a parent →
// child signal — defining them in one place keeps producer and
// consumer from drifting silently.
package protocol

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
// `internal/logging.CorrelationIDEnv` is an alias kept for backwards
// compatibility with pre-protocol callers; new code should import this
// constant directly (issue 034).
const CorrelationID = "RAIOZ_CORRELATION_ID"
