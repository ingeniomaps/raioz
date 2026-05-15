#!/usr/bin/env bash
# Unit-style tests for install.sh::verify_sha256 (issue 081).
#
# install.sh runs `main` at the bottom; we source it with
# RAIOZ_INSTALL_TEST=1 to skip that call and exercise the
# verification function directly against synthesized
# fixtures. The fixtures live in a tempdir we clean up at exit.
#
# Exit code: 0 on all assertions passing, non-zero on first failure.

set -u

cd "$(dirname "$0")/.."

# Suppress the trap-installed cleanup_install_tmp until we have sourced
# the script — RAIOZ_INSTALL_TMP starts empty so the trap is a noop.
export RAIOZ_INSTALL_TEST=1
# shellcheck source=../install.sh
. ./install.sh

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

archive="raioz_0.7.0_linux_amd64.tar.gz"
content="$tmp/$archive"
echo "fixture-payload" > "$content"

if command -v sha256sum >/dev/null 2>&1; then
    real_sha=$(sha256sum "$content" | awk '{print $1}')
else
    real_sha=$(shasum -a 256 "$content" | awk '{print $1}')
fi

fail_count=0

# Case 1: matching checksum → exits success, prints "Checksum verified".
checksums="$tmp/checksums.txt"
{
    echo "$real_sha  $archive"
    echo "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef  other_archive.zip"
} > "$checksums"

if output=$(verify_sha256 "$content" "$checksums" "$archive" 2>&1); then
    case "$output" in
        *"Checksum verified"*) echo "PASS: match accepted" ;;
        *)                     echo "FAIL: match accepted but output unexpected: $output"; fail_count=$((fail_count+1)) ;;
    esac
else
    echo "FAIL: match should have succeeded, got exit code $?, output: $output"
    fail_count=$((fail_count+1))
fi

# Case 2: missing entry → fail with "No checksum entry".
{
    echo "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef  other_archive.zip"
} > "$checksums"

if output=$(verify_sha256 "$content" "$checksums" "$archive" 2>&1); then
    echo "FAIL: missing entry should have failed but didn't; output: $output"
    fail_count=$((fail_count+1))
else
    case "$output" in
        *"No checksum entry"*) echo "PASS: missing entry rejected" ;;
        *)                     echo "FAIL: missing entry rejected but wrong message: $output"; fail_count=$((fail_count+1)) ;;
    esac
fi

# Case 3: mismatched checksum → fail with "Checksum mismatch".
{
    echo "0000000000000000000000000000000000000000000000000000000000000000  $archive"
} > "$checksums"

if output=$(verify_sha256 "$content" "$checksums" "$archive" 2>&1); then
    echo "FAIL: mismatch should have failed but didn't; output: $output"
    fail_count=$((fail_count+1))
else
    case "$output" in
        *"Checksum mismatch"*) echo "PASS: mismatch rejected" ;;
        *)                     echo "FAIL: mismatch rejected but wrong message: $output"; fail_count=$((fail_count+1)) ;;
    esac
fi

# Case 4: similar-but-not-equal filename in checksums.txt MUST NOT match
# (awk uses $2 == name, exact). Guards against substring poisoning.
echo "$real_sha  raioz_0.7.0_linux_amd64.tar.gz.sig" > "$checksums"
if output=$(verify_sha256 "$content" "$checksums" "$archive" 2>&1); then
    echo "FAIL: substring filename should not match; output: $output"
    fail_count=$((fail_count+1))
else
    case "$output" in
        *"No checksum entry"*) echo "PASS: substring filename rejected" ;;
        *)                     echo "FAIL: substring filename rejected but wrong message: $output"; fail_count=$((fail_count+1)) ;;
    esac
fi

if [ "$fail_count" -gt 0 ]; then
    echo "❌ $fail_count assertion(s) failed"
    exit 1
fi
echo "✅ install.sh verify_sha256 all assertions passed"
