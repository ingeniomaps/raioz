#!/bin/bash
# Check code coverage against a threshold.
# Usage: ./scripts/check-coverage.sh [threshold]
# Default threshold: 80.0

set -euo pipefail

THRESHOLD=${1:-80.0}
COVERAGE_FILE=${COVERAGE_FILE:-coverage.out}

if [ ! -f "$COVERAGE_FILE" ]; then
    echo "Coverage file '$COVERAGE_FILE' not found."
    echo "Run: make test-coverage"
    exit 1
fi

# Strip test-only packages (mocks, testing helpers) from the profile
# before computing the total. They exist solely to support the test
# suite, so reporting them as "uncovered production code" is misleading.
FILTERED=$(mktemp)
trap 'rm -f "$FILTERED"' EXIT
grep -v -E "raioz/internal/(mocks|testing)" "$COVERAGE_FILE" > "$FILTERED"

TOTAL_COVERAGE=$(go tool cover -func="$FILTERED" \
    | grep -E "^total:" | awk '{print $3}' | sed 's/%//')

if [ -z "$TOTAL_COVERAGE" ]; then
    echo "Failed to calculate coverage"
    exit 1
fi

echo "Total coverage: ${TOTAL_COVERAGE}%"
echo "Threshold: ${THRESHOLD}%"

if awk "BEGIN {exit !($TOTAL_COVERAGE >= $THRESHOLD)}"; then
    echo "Coverage meets threshold (${TOTAL_COVERAGE}% >= ${THRESHOLD}%)"
    exit 0
else
    echo "Coverage below threshold (${TOTAL_COVERAGE}% < ${THRESHOLD}%)"
    echo ""
    echo "Coverage by package:"
    go tool cover -func="$FILTERED" \
        | grep -E "^raioz" | sort -k3 -n
    exit 1
fi
