#!/bin/bash
set -euo pipefail

echo "=== Raioz Integration Test ==="

# Build
echo "[1/7] Building..."
make build
cp raioz /usr/local/bin/ 2>/dev/null || true

# Setup test project
TESTDIR=$(mktemp -d)
trap "rm -rf $TESTDIR; raioz down 2>/dev/null || true" EXIT

mkdir -p "$TESTDIR/api"
printf 'module example.com/api\ngo 1.22\n' > "$TESTDIR/api/go.mod"
cat > "$TESTDIR/api/main.go" << 'GO'
package main
import ("fmt";"log";"net/http")
func main() {
  http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") })
  http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "healthy") })
  log.Fatal(http.ListenAndServe(":8080", nil))
}
GO

cd "$TESTDIR"

# Test init
echo "[2/7] Testing init..."
raioz init
test -f raioz.yaml && echo "  PASS: raioz.yaml generated"

# Test check
echo "[3/7] Testing check..."
raioz check && echo "  PASS: config valid"

# Test up
echo "[4/7] Testing up..."
raioz up
sleep 3
curl -sf http://localhost:8080/ > /dev/null && echo "  PASS: API responding"

# Test status
echo "[5/7] Testing status..."
raioz status && echo "  PASS: status works"

# Test logs
echo "[6/7] Testing logs..."
raioz logs api --tail 3 && echo "  PASS: logs works"

# Test down + verify cleanup
echo "[7/7] Testing down..."
raioz down
sleep 1
if curl -sf http://localhost:8080/ 2>/dev/null; then
  echo "  FAIL: API still running after down"
  exit 1
fi
echo "  PASS: down stopped everything"

echo ""
echo "=== ALL TESTS PASSED ==="
