#!/bin/bash
# Fail if a PR changes the YAML schema (internal/config/yaml_types.go
# or internal/config/yaml_aux_types.go) without also adding or
# modifying at least one fixture under internal/config/testdata/configs/.
#
# Rationale: the YAML schema is the public user contract. Changing it
# without exercising the new shape in a fixture means CI can't tell us
# whether old configs still parse. See docs/issues/027-config-corpus.md
# and internal/config/testdata/configs/README.md.
#
# Behavior:
#   - On CI (BASE_REF set): diff HEAD against the merge-base with BASE_REF.
#   - Locally: diff against origin/main; falls back to no-op if origin
#     isn't reachable (offline contributors aren't blocked).
#
# The intent is "loud on PRs, helpful locally", not "block every
# developer working without network".

set -euo pipefail

# Files that, if changed, require a fixture diff.
SCHEMA_FILES=(
    "internal/config/yaml_types.go"
    "internal/config/yaml_aux_types.go"
)

FIXTURE_DIR="internal/config/testdata/configs"

# Pick a base commit to diff against.
base=""
if [ -n "${BASE_REF:-}" ]; then
    base="$BASE_REF"
elif git rev-parse --verify --quiet origin/main >/dev/null 2>&1; then
    base="origin/main"
elif git rev-parse --verify --quiet main >/dev/null 2>&1; then
    base="main"
else
    # No reasonable base; opt out rather than block.
    echo "ℹ️  No base ref found (no origin/main or main); " \
         "skipping fixture-diff check."
    exit 0
fi

# Use merge-base so we compare against the branch point, not the tip.
mb=$(git merge-base "$base" HEAD 2>/dev/null || echo "")
if [ -z "$mb" ]; then
    echo "ℹ️  No merge-base with $base; skipping fixture-diff check."
    exit 0
fi

changed=$(git diff --name-only "$mb" HEAD)

# A schema file is considered "really changed" only when the diff
# adds or removes a non-comment, non-blank line. Pure comment edits
# (typo fixes, reformatting, documentation cleanup) don't alter the
# user contract and don't require a fixture refresh.
has_substantive_change() {
    local file="$1"
    local diff
    diff=$(git diff -U0 "$mb" HEAD -- "$file" 2>/dev/null || true)
    [ -z "$diff" ] && return 1

    echo "$diff" | awk '
        /^(diff |index |--- |\+\+\+ |@@ )/ { next }
        /^[+-]/ {
            line = substr($0, 2)
            sub(/^[[:space:]]+/, "", line)
            if (line == "" || line ~ /^\/\//) next
            found = 1
            exit
        }
        END { exit (found ? 0 : 1) }
    '
}

schema_changed=false
for f in "${SCHEMA_FILES[@]}"; do
    if echo "$changed" | grep -Fxq "$f"; then
        if has_substantive_change "$f"; then
            schema_changed=true
            break
        fi
    fi
done

if [ "$schema_changed" = "false" ]; then
    echo "✅ No schema files changed; corpus diff not required."
    exit 0
fi

fixture_changed=$(echo "$changed" | grep -E "^${FIXTURE_DIR}/.+\.ya?ml$" || true)

if [ -z "$fixture_changed" ]; then
    echo "❌ YAML schema changed but no fixtures were modified." >&2
    echo "" >&2
    echo "When you change one of:" >&2
    for f in "${SCHEMA_FILES[@]}"; do
        echo "  $f" >&2
    done
    echo "" >&2
    echo "you must add or update a fixture in $FIXTURE_DIR/ that exercises" >&2
    echo "the new or changed field. See $FIXTURE_DIR/README.md for the" >&2
    echo "corpus charter and conventions." >&2
    exit 1
fi

echo "✅ Schema change is accompanied by fixture changes:"
echo "$fixture_changed" | sed 's/^/   /'
