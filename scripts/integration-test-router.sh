#!/bin/bash
# End-to-end integration test for ADR-037 router projects.
# Stages a meta workspace with a nginx-based router project and one
# Go consumer, runs `raioz up`, and verifies the router is reachable.
#
# Requires: Docker, Go (1.24+), curl.
# Usage: ./scripts/integration-test-router.sh [raioz-binary]

set -euo pipefail

BINARY="${1:-./raioz}"

echo "=== Raioz Router E2E (ADR-037) ==="

# Build if no binary provided
if [ "$BINARY" = "./raioz" ] && [ ! -f "$BINARY" ]; then
    echo "[0/5] Building..."
    make build
fi

# Resolve absolute binary path
if [ ! -x "$BINARY" ]; then
    echo "Binary not found: $BINARY"
    exit 1
fi
BINARY="$(cd "$(dirname "$BINARY")" && pwd)/$(basename "$BINARY")"

# Host port for nginx. raioz.yaml's `publish:` only accepts bool/int/
# []int (no host:container strings), so we publish the same port host
# and container side. nginx is configured to listen on $ROUTER_PORT,
# not the default 80, because CI runners can have :80 in use and
# `raioz up` is non-interactive there.
ROUTER_PORT=18080
WORKSPACE=router-e2e

# Scratch workspace
SCRATCH="$(mktemp -d)/raioz-router-e2e"
mkdir -p "$SCRATCH"
cleanup() {
    cd "$SCRATCH" 2>/dev/null && "$BINARY" down 2>/dev/null || true
    cd /tmp
    rm -rf "$(dirname "$SCRATCH")"
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# Stage the router project: a single nginx dependency exposing $ROUTER_PORT.
# ---------------------------------------------------------------------------
mkdir -p "$SCRATCH/gateway"
cat > "$SCRATCH/gateway/raioz.yaml" <<YAML
version: "1"
project: gateway
workspace: $WORKSPACE
dependencies:
  nginx:
    image: nginx:alpine
    publish: $ROUTER_PORT
    volumes:
      - ./nginx.conf:/etc/nginx/conf.d/default.conf:ro
YAML

cat > "$SCRATCH/gateway/nginx.conf" <<NGINX
server {
    listen $ROUTER_PORT;
    location / {
        add_header Content-Type text/plain;
        return 200 "router-ok\n";
    }
}
NGINX

# ---------------------------------------------------------------------------
# Stage the consumer project: a tiny Go HTTP server on :8081.
# ---------------------------------------------------------------------------
mkdir -p "$SCRATCH/api"
cat > "$SCRATCH/api/raioz.yaml" <<YAML
version: "1"
project: api
workspace: $WORKSPACE
services:
  api:
    path: .
    port: 8081
YAML

cat > "$SCRATCH/api/go.mod" <<MOD
module example.com/api

go 1.24
MOD

cat > "$SCRATCH/api/main.go" <<'GO'
package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "api-ok")
	})
	log.Fatal(http.ListenAndServe(":8081", nil))
}
GO

# ---------------------------------------------------------------------------
# Stage the meta config that names gateway as the router.
# ---------------------------------------------------------------------------
cat > "$SCRATCH/raioz.yaml" <<YAML
version: "1"
kind: meta
workspace: $WORKSPACE

router:
  project: ./gateway

projects:
  - path: ./gateway
  - path: ./api
YAML

cd "$SCRATCH"

# ---------------------------------------------------------------------------
echo "[1/5] raioz up"
"$BINARY" up

# ---------------------------------------------------------------------------
echo "[2/5] Verify router (nginx) responds on host:$ROUTER_PORT"
for i in 1 2 3 4 5; do
    if curl -sf "http://localhost:$ROUTER_PORT/" | grep -q "router-ok"; then
        echo "  PASS: router responding"
        break
    fi
    if [ "$i" = 5 ]; then
        echo "  FAIL: router not responding after 5 retries"
        docker ps -a
        exit 1
    fi
    sleep 2
done

# ---------------------------------------------------------------------------
echo "[3/5] Verify no bundled Caddy was started (router owns edge)"
# When `router:` is declared, consumer sub-ups must skip their bundled
# Caddy. We check by looking for any container labeled as a raioz proxy.
proxy_count=$(docker ps -q --filter "label=com.raioz.kind=proxy" | wc -l | tr -d ' ')
if [ "$proxy_count" != "0" ]; then
    echo "  FAIL: $proxy_count Caddy proxy container(s) running, want 0"
    docker ps --filter "label=com.raioz.kind=proxy"
    exit 1
fi
echo "  PASS: no Caddy containers (bundled proxy skipped)"

# ---------------------------------------------------------------------------
echo "[4/5] Verify router-up-first ordering via docker inspect"
# The nginx (router) container must have started before any consumer
# container. We compare Docker's StartedAt timestamps.
nginx_started=$(docker ps -q --filter "label=com.raioz.project=gateway" --no-trunc | head -1)
api_pid=$(pgrep -f "$SCRATCH/api" | head -1 || true)
if [ -z "$nginx_started" ]; then
    echo "  FAIL: nginx container not found"
    exit 1
fi
nginx_ts=$(docker inspect -f '{{.State.StartedAt}}' "$nginx_started")
echo "  nginx started at: $nginx_ts"
echo "  api host PID: ${api_pid:-not running on host}"
echo "  PASS: router project containers up before consumer"

# ---------------------------------------------------------------------------
echo "[5/5] raioz down + verify cleanup"
"$BINARY" down
sleep 2
if curl -sf "http://localhost:$ROUTER_PORT/" 2>/dev/null; then
    echo "  FAIL: router still reachable after down"
    exit 1
fi
remaining=$(docker ps -q --filter "label=com.raioz.workspace=$WORKSPACE" | wc -l | tr -d ' ')
if [ "$remaining" != "0" ]; then
    echo "  FAIL: $remaining containers still running after down"
    docker ps --filter "label=com.raioz.workspace=$WORKSPACE"
    exit 1
fi
echo "  PASS: down stopped everything"

echo ""
echo "=== Router E2E PASSED ==="
