#!/usr/bin/env bash
# Integration test suite for raioz
# Runs real commands against Docker using the examples/ directory
#
# Usage: ./scripts/integration-test.sh [binary-path]
# Default binary: ./raioz (from make build)

set -euo pipefail

RAIOZ="${1:-./raioz}"
PASS=0
FAIL=0
EXAMPLES_DIR="$(cd "$(dirname "$0")/../examples" && pwd)"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m'

log_pass() {
    PASS=$((PASS + 1))
    echo -e "  ${GREEN}✔${NC} $1"
}

log_fail() {
    FAIL=$((FAIL + 1))
    echo -e "  ${RED}✘${NC} $1: $2"
}

log_section() {
    echo ""
    echo -e "${YELLOW}━━━ $1 ━━━${NC}"
}

assert_ok() {
    local name="$1"
    shift
    if "$@" >/dev/null 2>&1; then
        log_pass "$name"
    else
        log_fail "$name" "exit code $?"
    fi
}

assert_fail() {
    local name="$1"
    shift
    if "$@" >/dev/null 2>&1; then
        log_fail "$name" "expected failure but succeeded"
    else
        log_pass "$name"
    fi
}

assert_output_contains() {
    local name="$1"
    local pattern="$2"
    shift 2
    local output
    output=$("$@" 2>&1) || true
    if echo "$output" | grep -qi "$pattern"; then
        log_pass "$name"
    else
        log_fail "$name" "output missing '$pattern'"
    fi
}

cleanup_project() {
    local dir="$1"
    cd "$dir"
    $RAIOZ down >/dev/null 2>&1 || true
    # Force stop any remaining containers from generated compose
    local ws_name
    ws_name=$(grep -o '"name"[[:space:]]*:[[:space:]]*"[^"]*"' .raioz.json 2>/dev/null | head -1 | sed 's/.*"name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
    if [ -n "$ws_name" ]; then
        docker ps -a --filter "name=raioz-${ws_name}" --format "{{.Names}}" 2>/dev/null | xargs -r docker rm -f 2>/dev/null || true
    fi
    # Also force down any project compose
    docker compose down >/dev/null 2>&1 || true
    cd - >/dev/null
}

cleanup_docker() {
    # Stop and remove any leftover containers from previous runs
    docker ps -a --filter "name=raioz-" --format "{{.Names}}" 2>/dev/null | xargs -r docker rm -f 2>/dev/null || true
    # Also clean containers from project-compose example
    docker ps -a --filter "name=13-project-compose" --format "{{.Names}}" 2>/dev/null | xargs -r docker rm -f 2>/dev/null || true
}

# Ensure binary exists
if [ ! -f "$RAIOZ" ]; then
    echo "Binary not found: $RAIOZ"
    echo "Run 'make build' first or pass the binary path"
    exit 1
fi

RAIOZ="$(cd "$(dirname "$RAIOZ")" && pwd)/$(basename "$RAIOZ")"
echo "Using binary: $RAIOZ"
echo "Examples dir: $EXAMPLES_DIR"

# Pre-cleanup
cleanup_docker

# ============================================
# Test 1: Doctor
# ============================================
log_section "Test: raioz doctor"
assert_ok "doctor passes all checks" $RAIOZ doctor

# ============================================
# Test 2: Image-only project (05)
# ============================================
log_section "Test: image-only project (05)"

docker network create image-only-network 2>/dev/null || true
cd "$EXAMPLES_DIR/05-image-only"

assert_output_contains "raioz up" "exitosamente\|successfully" $RAIOZ up
assert_output_contains "raioz status shows services" "web\|api\|redis" $RAIOZ status
assert_output_contains "raioz exec redis ping" "PONG" $RAIOZ exec redis redis-cli ping
assert_output_contains "raioz exec web ls" "nginx.conf" $RAIOZ exec web ls /etc/nginx/nginx.conf
assert_fail "raioz exec nonexistent → error" $RAIOZ exec noexiste sh
assert_output_contains "raioz exec non-interactive" "OK" $RAIOZ exec -i=false redis redis-cli SET integration-test ok
assert_ok "raioz volumes list" $RAIOZ volumes list

cleanup_project "$EXAMPLES_DIR/05-image-only"
log_pass "raioz down"

# ============================================
# Test 3: Infra-only project (07)
# ============================================
log_section "Test: infra-only project (07)"

docker network create infra-network 2>/dev/null || true
cd "$EXAMPLES_DIR/07-infra-only"

assert_output_contains "raioz up" "exitosamente\|successfully" $RAIOZ up
assert_output_contains "raioz exec postgres psql" "1" $RAIOZ exec postgres psql -U postgres -c "SELECT 1"
assert_output_contains "raioz exec redis ping" "PONG" $RAIOZ exec redis redis-cli ping
assert_output_contains "raioz exec mongo" "ok" $RAIOZ exec mongo mongosh --eval "db.runCommand({ping:1})"

cleanup_project "$EXAMPLES_DIR/07-infra-only"
log_pass "raioz down"

# ============================================
# Test 4: Host service (11)
# ============================================
log_section "Test: host+docker service (11)"

docker network create host-test-network 2>/dev/null || true
mkdir -p /tmp/host-worker-test
cd "$EXAMPLES_DIR/11-host-service"

assert_output_contains "raioz up" "exitosamente\|successfully" $RAIOZ up
assert_fail "raioz exec host-worker → error" $RAIOZ exec host-worker sh

cleanup_project "$EXAMPLES_DIR/11-host-service"
log_pass "raioz down"

# ============================================
# Test 5: Project compose (13)
# ============================================
log_section "Test: project-compose (13)"

docker network create project-compose-network 2>/dev/null || true
cd "$EXAMPLES_DIR/13-project-compose"

assert_output_contains "raioz up" "exitosamente\|successfully" $RAIOZ up
assert_output_contains "raioz exec redis (generated)" "PONG" $RAIOZ exec redis redis-cli ping
assert_output_contains "raioz exec nginx (project compose)" "nginx.conf" $RAIOZ exec nginx ls /etc/nginx/nginx.conf
assert_output_contains "raioz exec busybox (project compose)" "integration-test" $RAIOZ exec busybox echo "integration-test"

cleanup_project "$EXAMPLES_DIR/13-project-compose"
log_pass "raioz down"

# ============================================
# Test 6: Cross-project operations
# ============================================
log_section "Test: cross-project operations"

assert_ok "raioz list" $RAIOZ list
assert_output_contains "raioz --help shows new commands" "exec\|restart\|volumes\|doctor" $RAIOZ --help

# ============================================
# Cleanup
# ============================================
docker network rm image-only-network 2>/dev/null || true
docker network rm infra-network 2>/dev/null || true
docker network rm host-test-network 2>/dev/null || true
docker network rm project-compose-network 2>/dev/null || true

# ============================================
# Summary
# ============================================
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
TOTAL=$((PASS + FAIL))
echo -e "Results: ${GREEN}${PASS} passed${NC}, ${RED}${FAIL} failed${NC}, ${TOTAL} total"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ $FAIL -gt 0 ]; then
    exit 1
fi
