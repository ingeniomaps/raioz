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

TOTAL_COVERAGE=$(go tool cover -func="$COVERAGE_FILE" \
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
    go tool cover -func="$COVERAGE_FILE" \
        | grep -E "^raioz" | sort -k3 -n
    exit 1
fi
