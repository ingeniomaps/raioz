#!/usr/bin/env bash
# Fail if a NEW file:line outside the baseline triggers errorlint.
# Baseline lives in scripts/errorlint-baseline.txt and shrinks as
# call sites migrate to %w / errors.Is / errors.As. New violations
# fail outright; baseline entries that no longer fire trip the
# script too (the dev must prune the baseline in the same PR).
#
# Pattern: ADR-027 / ADR-029 shrinking-baseline ratchet.

set -euo pipefail

cd "$(dirname "$0")/.."

baseline="scripts/errorlint-baseline.txt"
if [ ! -f "$baseline" ]; then
    echo "❌ missing baseline file: $baseline" >&2
    exit 1
fi

declare -A allowed
while IFS= read -r line; do
    case "$line" in
        ''|'#'*) continue ;;
    esac
    allowed["$line"]=1
done < "$baseline"

current=$(
    golangci-lint run --enable-only=errorlint --timeout=5m 2>&1 \
        | grep -E 'errorlint\)$' \
        | sed -E 's/^([^:]+:[0-9]+):.*$/\1/' \
        | sort \
        || true
)

violations=0
stale=""

declare -A currentSet
while IFS= read -r line; do
    [ -z "$line" ] && continue
    currentSet["$line"]=1
    if [ -z "${allowed[$line]:-}" ]; then
        echo "❌ $line is a new errorlint violation but not on the baseline." >&2
        echo "   Wrap the error with %w, replace == with errors.Is, or use errors.As" >&2
        echo "   for the type assertion. If this is a deliberate add (e.g. a baseline" >&2
        echo "   refactor), update scripts/errorlint-baseline.txt in the same commit." >&2
        violations=$(( violations + 1 ))
    fi
done <<< "$current"

for f in "${!allowed[@]}"; do
    if [ -z "${currentSet[$f]:-}" ]; then
        stale+="   $f"$'\n'
    fi
done

if [ "$violations" -gt 0 ]; then
    exit 1
fi

if [ -n "$stale" ]; then
    echo "ℹ Baseline entries no longer firing — prune them from scripts/errorlint-baseline.txt:"
    echo "$stale"
    exit 1
fi

echo "✅ errorlint held to baseline (${#allowed[@]} entries)"
