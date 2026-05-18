package app

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

// secretKeyPattern matches env-var keys that LOOK like they carry a
// secret. False positives are fine — the goal is to redact aggressively
// so an operator running PrintSpawnEnv in a shared session doesn't
// paste their AWS_SECRET_ACCESS_KEY value into chat.
//
// The patterns intentionally don't try to be exhaustive — a value
// matching `*_TOKEN_HINT` or similar would also be redacted by the
// suffix match. False negatives are the dangerous case; better to
// over-redact than to leak.
var secretKeyPattern = regexp.MustCompile(
	`(?i)(token|secret|password|passwd|key|credential|api[_-]?key|auth|bearer)` +
		`(_|$)|^aws_|^github_|^slack_|^stripe_|^twilio_|^sentry_`,
)

// raiozReadsKnown enumerates the env vars raioz itself reads. Drawn
// from CLAUDE.md / CONFIG_REFERENCE.md inventory + the protocol
// package's child-env contract. Used by PrintSpawnEnv so the operator
// knows what `env -i` invocations need to whitelist.
var raiozReadsKnown = []string{
	// raioz config knobs
	"RAIOZ_HOME",
	"RAIOZ_RUNTIME",
	"RAIOZ_LANG",
	"RAIOZ_LOG_LEVEL",
	"RAIOZ_LOG_JSON",
	"RAIOZ_LAUNCHER_TIMEOUT",
	"RAIOZ_LAUNCHER_DRAIN_TIMEOUT",
	"RAIOZ_SIBLING_TIMEOUT",
	"RAIOZ_LOCK_STALE_AGE",
	"RAIOZ_META_SUB_TIMEOUT",
	// internal protocol (parent → child)
	"RAIOZ_SIBLING_STACK",
	"RAIOZ_CORRELATION_ID",
	"RAIOZ_ROUTER_ACTIVE",
	// XDG bases (RaiozStateDir / RaiozConfigDir fall back through these)
	"XDG_STATE_HOME",
	"XDG_CONFIG_HOME",
	// universal
	"HOME",
	"PATH",
}

// PrintSpawnEnv writes the env raioz WOULD inherit when spawning a
// sub-process (hooks, sibling spawn, custom stop, meta runSingle —
// SECURITY.md § "Meta env inheritance"). Secret-shaped keys are
// listed with `[SECRET-SHAPED]` but their VALUES are redacted.
// Operators use this to compose a sandboxed `env -i ...` invocation
// for CI runs against untrusted yamls.
func PrintSpawnEnv(w io.Writer) {
	envs := os.Environ()
	sort.Strings(envs)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "=== Env raioz would inherit on sub-spawn ===")
	for _, kv := range envs {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			continue
		}
		key := kv[:eq]
		val := kv[eq+1:]
		if secretKeyPattern.MatchString(key) {
			fmt.Fprintf(w, "  %s=<redacted> [SECRET-SHAPED]\n", key)
		} else {
			fmt.Fprintf(w, "  %s=%s\n", key, val)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "=== Env vars raioz actively reads ===")
	for _, k := range raiozReadsKnown {
		v, ok := os.LookupEnv(k)
		if ok {
			if secretKeyPattern.MatchString(k) {
				fmt.Fprintf(w, "  %s=<redacted>\n", k)
			} else {
				fmt.Fprintf(w, "  %s=%s\n", k, v)
			}
		} else {
			fmt.Fprintf(w, "  %s=(unset)\n", k)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "=== Recommended sandbox for CI / untrusted yamls ===")
	fmt.Fprintln(w, "  env -i HOME=$HOME PATH=$PATH RAIOZ_HOME=$RAIOZ_HOME raioz up")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Add other RAIOZ_* env vars to the whitelist as needed.")
	fmt.Fprintln(w, "See docs/SECURITY.md § Meta env inheritance for the full threat model.")
}
