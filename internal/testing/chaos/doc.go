// Package chaos hosts fault-injection and concurrency stress tests
// that complement — but never replace — the per-package unit tests.
//
// # Convention
//
// Per-package unit tests stay co-located with their source under the
// package they exercise. A test belongs here only when at least one
// of these is true:
//
//   - It crosses package boundaries to exercise an invariant that
//     spans two or more packages (e.g. fsutil + workspace + lock).
//   - It is a fault-injection / stress test whose runtime cost or
//     parallelism would be inappropriate in the package's own
//     `go test` run (dozens of goroutines, multi-second storms).
//   - It asserts a property under deliberate concurrency, not a
//     functional contract — the regular tests stay the source of
//     truth for correctness, this package stays the source of truth
//     for non-corruption under load.
//
// Run with the race detector:
//
//	go test -race ./internal/testing/chaos/...
//
// All tests in this package must complete in a few seconds total
// when the suite is healthy; longer suggests a regression in the
// contention surface they exercise.
package chaos
