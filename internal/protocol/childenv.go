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
