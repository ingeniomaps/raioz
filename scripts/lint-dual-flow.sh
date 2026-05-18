#!/usr/bin/env bash
# Shrinking-baseline ratchet for the legacy SchemaVersion literal
# comparisons. The canonical path is internal/app.SelectFlow
# (ADR-039); existing inline readers are listed in
# scripts/dual-flow-baseline.txt and migrate opportunistically.
# New readers outside the baseline fail outright. Test files exempt.

set -euo pipefail

cd "$(dirname "$0")/.."

baseline="scripts/dual-flow-baseline.txt"
if [ ! -f "$baseline" ]; then
    echo "❌ missing baseline file: $baseline" >&2
    exit 1
fi

declare -A allowed=()
while IFS= read -r line; do
    case "$line" in
        ''|'#'*) continue ;;
    esac
    allowed["$line"]=1
done < "$baseline"

declare -a current=()
while IFS= read -r f; do
    current+=("${f#./}")
done < <(
    grep -rln \
        -e 'SchemaVersion == "2.0"' \
        -e 'SchemaVersion != "2.0"' \
        -e 'SchemaVersion == "1.0"' \
        -e 'SchemaVersion != "1.0"' \
        internal 2>/dev/null \
        | grep -v _test.go \
        | grep -v 'internal/domain/models/' \
        | grep -v 'internal/app/flow.go' \
        | sort \
        || true
)

violations=0
stale=""

for f in "${current[@]}"; do
    if [ -z "${allowed[$f]:-}" ]; then
        echo "❌ $f reads the legacy SchemaVersion literal but is not on the baseline." >&2
        echo "   Route the check through internal/app.SelectFlow or read" >&2
        echo "   deps.SourceFormat directly (ADR-039). If you're migrating an existing" >&2
        echo "   reader off the baseline, remove its entry in the same PR." >&2
        violations=$(( violations + 1 ))
    fi
done

declare -A currentSet=()
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
    exit 1
fi

count=${#current[@]}
echo "✅ dual-flow readers held to baseline (${count} entries)"
