#!/usr/bin/env bash
# verify-stamp.sh — guard against unstamped release binaries.
#
# Called from .goreleaser.yml as a post-build hook on every build.
# Only attempts execution when the binary is runnable on the current
# runner (linux/amd64 ELF). Cross-compiled outputs for darwin/windows
# or non-native arch silently skip — they get covered transitively
# because all builds share the same ldflag template.
#
# Exit 1 if the binary reports `version dev`, lacks Commit/Build date,
# or does not match the expected semver tag — which kills goreleaser
# before the archive/publish phase.

set -euo pipefail

BIN="${1:?usage: verify-stamp.sh <binary> <expected-version>}"
EXPECTED="${2:?usage: verify-stamp.sh <binary> <expected-version>}"

if [ ! -x "$BIN" ]; then
    echo "[verify-stamp] $BIN: not executable — skipping"
    exit 0
fi

if ! file "$BIN" 2>/dev/null | grep -q "ELF 64-bit LSB.*x86-64"; then
    echo "[verify-stamp] $BIN: cross-compiled for another platform — skipping"
    exit 0
fi

out="$("$BIN" version 2>&1)"
echo "[verify-stamp] $BIN"
echo "$out"

if echo "$out" | grep -qE "version (dev|unknown)"; then
    echo "::error::Release binary is unstamped (reports 'version dev'). Aborting before publish."
    exit 1
fi

if ! echo "$out" | grep -qF "raioz version ${EXPECTED}"; then
    echo "::error::Version mismatch: expected '${EXPECTED}', got: $(echo "$out" | head -n1)"
    exit 1
fi

if ! echo "$out" | grep -q "^Commit: "; then
    echo "::error::Commit line missing — ldflags partially broken"
    exit 1
fi

if ! echo "$out" | grep -q "^Build date: "; then
    echo "::error::Build date line missing — ldflags partially broken"
    exit 1
fi

echo "[verify-stamp] OK"
