#!/bin/bash
# Fail if a file under internal/cli/ bypasses internal/app/ without
# being on the explicit exemption list. The exemption is documented
# in docs/ARCHITECTURE.md and ADR-017.
#
# A CLI file is OK when:
#   - it imports "raioz/internal/app" (it routes through a use case), or
#   - its basename is on EXEMPT below.
#
# Adding to EXEMPT requires updating the ARCHITECTURE.md table and
# ADR-017 in the same PR — reviewers should refuse silent expansions.

set -euo pipefail

# Files allowed to skip the use-case layer.
EXEMPT='^(root|config_path|zzz_i18n_descriptions|wiring|version|lang|migrate|yaml|migrate_yaml|yaml_lint|graph)\.go$'

bad=""
for f in internal/cli/*.go; do
    case "$f" in *_test.go) continue ;; esac
    base=$(basename "$f")
    if [[ "$base" =~ $EXEMPT ]]; then
        continue
    fi
    if grep -q '"raioz/internal/app' "$f"; then
        continue
    fi
    bad="${bad}${f}"$'\n'
done

if [ -n "$bad" ]; then
    echo "❌ CLI file(s) bypass internal/app/ without being on the exempt list:" >&2
    echo "" >&2
    printf '%s' "$bad" | sed 's/^/   /' >&2
    echo "" >&2
    echo "Either:" >&2
    echo "  - route the command through a use case in internal/app/ (preferred)," >&2
    echo "  - or, if this is genuinely a pure-visualization / structural file," >&2
    echo "    add it to EXEMPT in scripts/lint-cli-layering.sh AND update" >&2
    echo "    docs/ARCHITECTURE.md (CLI thin-viz exception) AND ADR-017." >&2
    exit 1
fi

echo "✅ Every CLI command routes through internal/app/ or is on the exempt list."
