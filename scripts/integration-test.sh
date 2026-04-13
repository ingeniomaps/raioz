#!/bin/bash
# End-to-end integration test for Raioz.
# Requires: Docker, Go, make
# Usage: ./scripts/integration-test.sh [raioz-binary]

set -euo pipefail

BINARY="${1:-./raioz}"

echo "=== Raioz Integration Test ==="

# Build if no binary provided
if [ "$BINARY" = "./raioz" ] && [ ! -f "$BINARY" ]; then
    echo "[0/7] Building..."
    make build
fi

# Verify binary exists
if [ ! -x "$BINARY" ]; then
    echo "Binary not found: $BINARY"
    exit 1
fi

# Use absolute path for the binary
BINARY="$(cd "$(dirname "$BINARY")" && pwd)/$(basename "$BINARY")"

# Setup test project in a Docker-safe directory name
TESTDIR="$(mktemp -d)/raioz-integration-test"
mkdir -p "$TESTDIR"
trap 'rm -rf "$(dirname "$TESTDIR")"; cd /tmp; "$BINARY" down --project raioz-integration-test 2>/dev/null || true' EXIT

mkdir -p "$TESTDIR/api"
cat > "$TESTDIR/api/go.mod" << 'GOMOD'
module example.com/api
go 1.24
GOMOD

cat > "$TESTDIR/api/main.go" << 'GO'
package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	})
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "healthy")
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
GO

cd "$TESTDIR"

# Test init
echo "[1/7] Testing init..."
"$BINARY" init
test -f raioz.yaml && echo "  PASS: raioz.yaml generated"

# Test check
echo "[2/7] Testing check..."
"$BINARY" check && echo "  PASS: config valid"

# Test up
echo "[3/7] Testing up..."
"$BINARY" up
sleep 3
curl -sf http://localhost:8080/ > /dev/null && echo "  PASS: API responding"

# Test status
echo "[4/7] Testing status..."
"$BINARY" status && echo "  PASS: status works"

# Test logs
echo "[5/7] Testing logs..."
"$BINARY" logs api --tail 3 && echo "  PASS: logs works"

# Test down + verify cleanup
echo "[6/7] Testing down..."
"$BINARY" down
sleep 1
if curl -sf http://localhost:8080/ 2>/dev/null; then
    echo "  FAIL: API still running after down"
    exit 1
fi
echo "  PASS: down stopped everything"

# Test doctor
echo "[7/7] Testing doctor..."
"$BINARY" doctor && echo "  PASS: doctor works"

echo ""
echo "=== ALL TESTS PASSED ==="
