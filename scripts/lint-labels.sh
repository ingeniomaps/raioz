#!/bin/bash
# Fail if any Go production file outside internal/naming/ hardcodes a
# "com.raioz.*" label key as a string literal.
#
# The Docker label keys are the cross-package contract that `raioz down`
# uses to sweep its containers. Any rename of the constants in
# internal/naming/labels.go must be observable to the compiler — string
# literals scattered across the codebase silently drift and cause leaks.
#
# See: docs/issues/019-labels-lint.md, ADR-001 (container identity via labels).

set -euo pipefail

# Match `"com.raioz.<anything>` — the quote prefix ensures comments and
# docs that happen to mention the label name don't trigger the linter.
PATTERN='"com\.raioz\.'

hits=$(grep -rn "$PATTERN" --include='*.go' \
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
    exit 1
fi

echo "✅ No hardcoded com.raioz.* labels outside internal/naming/"
