#!/bin/bash
# Three rules backing ADR-001 (container identity via com.raioz.* labels):
#
#   R0 — no hardcoded "com.raioz.*" literal outside internal/naming/.
#        A rename of the constants must be observable to the compiler;
#        scattered literals drift silently and cause sweep leaks.
#
#   R1 — a file that builds a `docker run` argv must stamp the labels via
#        naming.Labels(). Ephemeral runs (`--rm`) are exempt: a container
#        that deletes itself on exit never has to be found again.
#
#   R2 — a file that writes container_name into a compose spec must stamp
#        them too. Legacy generators that predate the rule live in
#        scripts/labels-stamp-baseline.txt and may only leave the list.
#
# R0 alone let DockerfileRunner ship for a year creating containers with no
# labels at all: `raioz down` could not see them, reported success, and left
# the service running. Prohibiting the wrong spelling says nothing about
# code that never spells it.

set -euo pipefail

cd "$(dirname "$0")/.."

fail=0

# --- R0: no hardcoded label literals ----------------------------------------

hits=$(grep -rn '"com\.raioz\.' --include='*.go' \
    --exclude='*_test.go' \
    --exclude-dir='.git' \
    . 2>/dev/null \
    | grep -v '^\./internal/naming/' \
    || true)

if [ -n "$hits" ]; then
    echo "❌ Hardcoded com.raioz.* labels found outside internal/naming/:" >&2
    echo "" >&2
    echo "$hits" >&2
    echo "" >&2
    echo "Use the constants from internal/naming/labels.go instead:" >&2
    echo "  naming.LabelManaged    (com.raioz.managed)" >&2
    echo "  naming.LabelWorkspace  (com.raioz.workspace)" >&2
    echo "  naming.LabelProject    (com.raioz.project)" >&2
    echo "  naming.LabelService    (com.raioz.service)" >&2
    echo "  naming.LabelKind       (com.raioz.kind)" >&2
    fail=1
fi

# --- shared: the baseline of known-unstamped creators ------------------------

baseline="scripts/labels-stamp-baseline.txt"
if [ ! -f "$baseline" ]; then
    echo "❌ missing baseline file: $baseline" >&2
    exit 1
fi

declare -A allowed
baselined=0
while IFS= read -r line; do
    case "$line" in
        ''|'#'*) continue ;;
    esac
    allowed["$line"]=1
    baselined=$(( baselined + 1 ))
done < "$baseline"

declare -A offenders

# stampsLabels reports whether a file calls naming.Labels().
stampsLabels() {
    grep -q 'naming\.Labels(' "$1"
}

# --- R1: `docker run` sites -------------------------------------------------

while IFS= read -r f; do
    [ -z "$f" ] && continue
    f="${f#./}"
    # `--rm` marks an ephemeral container (snapshot's alpine tar, etc.):
    # nothing survives the command, so nothing needs finding later.
    if grep -q '"--rm"' "$f"; then
        continue
    fi
    stampsLabels "$f" || offenders["$f"]="docker run without naming.Labels()"
done < <(
    grep -rlE 'runtime\.Binary\(\), "run"|\[\]string\{"run",' \
        --include='*.go' --exclude='*_test.go' internal/ 2>/dev/null || true
)

# --- R2: compose specs that name a container --------------------------------

while IFS= read -r f; do
    [ -z "$f" ] && continue
    f="${f#./}"
    stampsLabels "$f" || offenders["$f"]="compose container_name without naming.Labels()"
done < <(
    grep -rlE '"container_name"(:|\])' \
        --include='*.go' --exclude='*_test.go' internal/ 2>/dev/null || true
)

# --- report ------------------------------------------------------------------

new=0
for f in "${!offenders[@]}"; do
    if [ -z "${allowed[$f]:-}" ]; then
        echo "❌ $f creates containers without stamping the raioz labels (${offenders[$f]})." >&2
        new=1
    fi
done

if [ "$new" -eq 1 ]; then
    echo "" >&2
    echo "Every runner that owns a container MUST stamp naming.Labels() — ADR-001." >&2
    echo "Without them 'raioz down' cannot find the container and reports success" >&2
    echo "while the service keeps running." >&2
    echo "Ephemeral containers are exempt: add --rm and they clean up after themselves." >&2
    fail=1
fi

stale=""
for f in "${!allowed[@]}"; do
    if [ -z "${offenders[$f]:-}" ]; then
        stale+="   $f"$'\n'
    fi
done

if [ "$fail" -ne 0 ]; then
    exit 1
fi

if [ -n "$stale" ]; then
    echo "ℹ Baseline entries now stamp their labels — please prune:"
    echo "$stale"
fi

echo "✅ Labels: no stray literals, every container creator stamps them ($baselined baselined)"
