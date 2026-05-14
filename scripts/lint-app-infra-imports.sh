#!/usr/bin/env bash
# Fail if a NEW file under internal/app/ or internal/cli/ imports
# raioz/internal/{docker,proxy,orchestrate} directly. Files already
# doing so are tracked in scripts/app-infra-imports-baseline.txt —
# the ratchet that backs ADR-012's segregation strategy.
#
# ADR-029. The aggregate DockerRunner keeps embedding
# the 6 segregated ports for backwards compat, but new code is
# expected to depend on the narrow port that fits its need. This
# lint catches drift: existing files can leave the list (file
# removed when its imports go through the port), but new files
# are not allowed on.
#
# Test files are exempt — they may bring in concrete packages for
# fixtures, since tests aren't the API surface ADR-012 protects.

set -euo pipefail

cd "$(dirname "$0")/.."

baseline="scripts/app-infra-imports-baseline.txt"
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

declare -a current
while IFS= read -r f; do
    current+=("${f#./}")
done < <(
    grep -rln \
        -e '"raioz/internal/docker"' \
        -e '"raioz/internal/proxy"' \
        -e '"raioz/internal/orchestrate"' \
        internal/app internal/cli 2>/dev/null \
        | grep -v _test.go || true
)

violations=0
stale=""

for f in "${current[@]}"; do
    if [ -z "${allowed[$f]:-}" ]; then
        echo "❌ $f imports raioz/internal/{docker,proxy,orchestrate} but is not on the baseline." >&2
        echo "   Use the domain port (interfaces.ContainerManager, ProxyManager, etc.) instead," >&2
        echo "   or — if this is genuinely a Plan-B exception — add it to the baseline AND update" >&2
        echo "   ADR-029 referencing the new entry." >&2
        violations=$(( violations + 1 ))
    fi
done

declare -A currentSet
for f in "${current[@]}"; do
    currentSet["$f"]=1
done
for f in "${!allowed[@]}"; do
    if [ -z "${currentSet[$f]:-}" ]; then
        stale+="   $f"$'\n'
    fi
done

if [ "$violations" -gt 0 ]; then
    exit 1
fi

if [ -n "$stale" ]; then
    echo "ℹ Baseline entries no longer detected — please prune:"
    echo "$stale"
fi

echo "✅ app-layer infra imports held to baseline (${#current[@]} entries)"
