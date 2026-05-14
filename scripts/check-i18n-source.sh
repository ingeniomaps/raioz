#!/usr/bin/env bash
# Fail when output.Print* takes a raw English string literal.
#
# raioz advertises bilingual (en/es) and `make check-i18n` validates
# the catalogs are synchronized. This script enforces the matching
# discipline at the source level: every user-facing message must go
# through i18n.T(). ADR-027.
#
# Pattern: output.Print<Anything>("<Uppercase first letter>…")
# matches calls like output.PrintInfo("Hello") but skips dynamic
# concatenation (output.PrintInfo("hello " + name)) and lowercase
# starts.
#
# Allowlist: scripts/i18n-source-baseline.txt caps each file's
# current violation count. New files fail outright; higher counts on
# existing files fail. Lower counts succeed but emit a hint to update
# the baseline so the ratchet stays monotonic.

set -euo pipefail

cd "$(dirname "$0")/.."

baseline="scripts/i18n-source-baseline.txt"
if [ ! -f "$baseline" ]; then
    echo "❌ missing baseline file: $baseline"
    exit 1
fi

declare -A allowed
while IFS=: read -r path count; do
    case "$path" in
        ''|'#'*) continue ;;
    esac
    allowed["$path"]="$count"
done < "$baseline"

declare -A current
while IFS= read -r line; do
    file="${line%%:*}"
    file="${file#./}"
    current["$file"]=$(( ${current["$file"]:-0} + 1 ))
done < <(
    grep -rEn 'output\.Print[A-Z][a-zA-Z]*\("[A-Z]' \
        internal --include="*.go" 2>/dev/null \
        | grep -v _test.go || true
)

violations=0
suggestions=""

for file in "${!current[@]}"; do
    got="${current[$file]}"
    cap="${allowed[$file]:-}"
    if [ -z "$cap" ]; then
        echo "❌ $file has $got raw English string(s); not on the baseline."
        echo "   New files must go through i18n.T() — no exceptions."
        violations=$(( violations + 1 ))
        continue
    fi
    if [ "$got" -gt "$cap" ]; then
        echo "❌ $file: $got raw strings (baseline allows $cap)."
        echo "   Wrap the new ones in i18n.T() or fix existing ones in this commit."
        violations=$(( violations + 1 ))
    elif [ "$got" -lt "$cap" ]; then
        suggestions+="   $file: $got (baseline says $cap — please tighten the baseline)"$'\n'
    fi
done

for file in "${!allowed[@]}"; do
    if [ -z "${current[$file]:-}" ]; then
        suggestions+="   $file: 0 (remove the entry; file is clean now)"$'\n'
    fi
done

if [ "$violations" -gt 0 ]; then
    echo ""
    echo "Add the key to internal/i18n/locales/{en,es}.json and wrap"
    echo "the string in i18n.T(\"key\", args…)."
    exit 1
fi

if [ -n "$suggestions" ]; then
    echo "ℹ Baseline is loose; please tighten scripts/i18n-source-baseline.txt:"
    echo "$suggestions"
fi

echo "✅ i18n source discipline holds within baseline"
