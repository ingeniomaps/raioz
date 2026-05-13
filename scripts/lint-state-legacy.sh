#!/bin/bash
# Fail if a new caller of the legacy state snapshot API appears.
#
# The "legacy state" is the .state.json file that mirrors the whole
# *models.Deps (state.Save / state.Load / state.Exists, exposed to the app
# layer as StateManager.{Save,Load,Exists}). Every new caller adds another
# path along which the file can diverge from what Docker actually has
# running. See ADR-011 for the full rationale.
#
# This linter is intentionally narrow: it whitelists today's callers
# (frozen at the time of issue 029) and fails CI if any other production
# file in the repo grows a reference to the deprecated API. Removing a
# caller from the whitelist is encouraged — issues 030 / 031 do that
# incrementally. Adding one is rejected.
#
# The grep patterns target:
#   - bare `state.Save(`, `state.Load(`, `state.Exists(`
#   - method calls `StateManager.Save(`, `.Save(ws, ...)` style on the
#     interface (we match by suffix `StateManager.<verb>(` to keep the
#     grep simple and avoid false positives on unrelated `.Save(`s).

set -euo pipefail

# Whitelist: existing callers we tolerate while 030/031 migrate them.
# Files listed here are allowed to reference state.Save/Load/Exists or
# StateManager.{Save,Load,Exists}. New callers must NOT be added —
# extend LocalState (internal/state/project_state.go) instead.
ALLOWED_RE='^(internal/state/.*|internal/infra/state/.*|internal/mocks/.*|internal/domain/interfaces/state\.go|internal/app/checkcase/usecase\.go|internal/app/down\.go|internal/app/exec\.go|internal/app/list\.go|internal/app/logs\.go|internal/app/restart\.go|internal/app/status\.go|internal/app/volumes\.go|internal/app/upcase/duplicate_project\.go|internal/app/upcase/state\.go|internal/app/upcase/workspace_project_conflict\.go)$'

# Match the deprecated entry points (both the package-level functions and
# the interface methods). The `[^a-zA-Z0-9_]` boundary prevents matching
# state.SaveLocalState / state.LoadGlobalState which are the modern API.
PATTERN='([^a-zA-Z0-9_])state\.(Save|Load|Exists)\(|StateManager\.(Save|Load|Exists)\('

hits=$(grep -rlnE "$PATTERN" --include='*.go' \
    --exclude='*_test.go' \
    --exclude-dir='.git' \
    . 2>/dev/null \
    | sed 's|^\./||' \
    || true)

bad=""
if [ -n "$hits" ]; then
    while IFS= read -r f; do
        [ -z "$f" ] && continue
        if ! echo "$f" | grep -Eq "$ALLOWED_RE"; then
            bad="${bad}${f}"$'\n'
        fi
    done <<< "$hits"
fi

if [ -n "$bad" ]; then
    echo "❌ New caller(s) of the deprecated state snapshot API:" >&2
    echo "" >&2
    printf '%s' "$bad" | sed 's/^/   /' >&2
    echo "" >&2
    echo "These functions are slated for removal (see ADR-011):" >&2
    echo "  state.Save, state.Load, state.Exists" >&2
    echo "  StateManager.Save, StateManager.Load, StateManager.Exists" >&2
    echo "" >&2
    echo "Use LocalState (internal/state/project_state.go) for runtime" >&2
    echo "state and re-read raioz.yaml + Docker labels for everything" >&2
    echo "else. If you genuinely need this API while 030/031 land," >&2
    echo "add the file to the whitelist in scripts/lint-state-legacy.sh" >&2
    echo "with a comment explaining the temporary need." >&2
    exit 1
fi

echo "✅ No new callers of the deprecated state snapshot API"
