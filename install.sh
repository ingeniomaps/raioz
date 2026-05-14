#!/bin/bash
# Raioz installer — downloads from GitHub releases or installs local binary
#
# Usage (Linux/macOS):
#   curl -fsSL https://raw.githubusercontent.com/ingeniomaps/raioz/main/install.sh | bash
#   INSTALL_DIR=~/.local/bin bash install.sh
#
# Windows:
#   go install github.com/ingeniomaps/raioz/cmd/raioz@latest
#   Or download the .zip from https://github.com/ingeniomaps/raioz/releases

if [ -z "${BASH_VERSION:-}" ]; then
    echo "Error: This script requires bash." >&2
    echo "Use: curl -fsSL ... | bash (not sh)" >&2
    exit 1
fi

set -euo pipefail

# --- Configuration -----------------------------------------------------------

BINARY_NAME="raioz"
GITHUB_REPO="${GITHUB_REPO:-ingeniomaps/raioz}"
GITHUB_URL="https://github.com/${GITHUB_REPO}"

# pick_install_dir chooses where to install. Honors INSTALL_DIR if set,
# otherwise picks the first preferred bin directory that already lives
# on the user's PATH — so the freshly installed binary actually wins
# `command -v raioz`. Falls back to ~/.local/bin (the binary still
# installs, just with a follow-up tip to add it to PATH).
pick_install_dir() {
    if [ -n "${INSTALL_DIR:-}" ]; then
        echo "$INSTALL_DIR"
        return
    fi
    local IFS=':' dir
    for dir in $PATH; do
        case "$dir" in
            "$HOME/.local/bin"|"$HOME/bin"|/usr/local/bin)
                echo "$dir"
                return
                ;;
        esac
    done
    echo "$HOME/.local/bin"
}

INSTALL_DIR="$(pick_install_dir)"

# --- Output helpers -----------------------------------------------------------

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info()    { echo -e "${BLUE}i${NC} $1"; }
ok()      { echo -e "${GREEN}✔${NC} $1"; }
warn()    { echo -e "${YELLOW}⚠${NC} $1" >&2; }
fail()    { echo -e "${RED}✗${NC} $1" >&2; exit 1; }

# --- Detection ----------------------------------------------------------------

detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux"  ;;
        Darwin*) echo "darwin" ;;
        *)       fail "Unsupported OS: $(uname -s). Supported: Linux, macOS" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)   echo "amd64" ;;
        arm64|aarch64)  echo "arm64" ;;
        *) fail "Unsupported architecture: $(uname -m). Supported: amd64, arm64" ;;
    esac
}

# --- HTTP helpers (curl or wget) ----------------------------------------------

http_get() {
    local url="$1" output="${2:-}"
    if command -v curl >/dev/null 2>&1; then
        if [ -n "$output" ]; then
            curl -fSL -o "$output" "$url"
        else
            curl -fsSL "$url"
        fi
    elif command -v wget >/dev/null 2>&1; then
        if [ -n "$output" ]; then
            wget -qO "$output" "$url"
        else
            wget -qO- "$url"
        fi
    else
        fail "curl or wget is required"
    fi
}

# --- Version & download -------------------------------------------------------

get_latest_version() {
    local api_url="https://api.github.com/repos/${GITHUB_REPO}/releases/latest"
    local version
    version=$(http_get "$api_url" "" 2>/dev/null \
        | grep '"tag_name":' \
        | sed -E 's/.*"([^"]+)".*/\1/' || echo "")

    if [ -z "$version" ]; then
        fail "Could not fetch latest version from GitHub"
    fi
    echo "$version"
}

download_release() {
    local version="$1" os="$2" arch="$3" output="$4"

    # Strip 'v' prefix for archive name (goreleaser convention)
    local ver_no_v="${version#v}"
    local archive="raioz_${ver_no_v}_${os}_${arch}.tar.gz"
    local url="${GITHUB_URL}/releases/download/${version}/${archive}"

    info "Downloading ${archive}..."
    local tmp_dir
    tmp_dir=$(mktemp -d)
    trap 'rm -rf "$tmp_dir"' EXIT

    http_get "$url" "${tmp_dir}/${archive}" \
        || fail "Download failed: ${url}\nBuild from source: make build && make install"

    tar -xzf "${tmp_dir}/${archive}" -C "$tmp_dir" \
        || fail "Failed to extract archive"

    local binary
    binary=$(find "$tmp_dir" -name "raioz" -type f | head -n 1)
    [ -n "$binary" ] || fail "Binary not found in archive"

    mv "$binary" "$output"
    chmod +x "$output"
}

# --- Install to target directory ----------------------------------------------

install_binary() {
    local src="$1"

    local check_dir="$INSTALL_DIR"
    [ -d "$check_dir" ] || check_dir="$(dirname "$check_dir")"
    if [ ! -w "$check_dir" ]; then
        info "Using sudo to install to ${INSTALL_DIR}"
        sudo mkdir -p "$INSTALL_DIR"
        sudo cp "$src" "${INSTALL_DIR}/${BINARY_NAME}"
        sudo chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    else
        mkdir -p "$INSTALL_DIR"
        cp "$src" "${INSTALL_DIR}/${BINARY_NAME}"
        chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    fi
}

# --- Dependency check ---------------------------------------------------------

check_dependencies() {
    info "Checking dependencies..."

    if command -v docker >/dev/null 2>&1; then
        if docker info >/dev/null 2>&1; then
            ok "Docker is installed and running"
        else
            warn "Docker is installed but not running"
        fi
    else
        warn "Docker is not installed — https://docs.docker.com/get-docker/"
    fi

    if command -v git >/dev/null 2>&1; then
        ok "Git is installed"
    else
        warn "Git is not installed — https://git-scm.com/downloads"
    fi
    echo ""
}

# --- Main ---------------------------------------------------------------------

main() {
    info "Installing raioz..."
    echo ""

    local binary_path=""

    # Development mode: only when the script was loaded from a file on
    # disk AND a sibling `raioz` binary lives next to it. When invoked
    # via `curl … | bash`, BASH_SOURCE[0] is empty (or "bash"), the
    # `dirname` falls back to cwd, and we used to pick up stray binaries
    # — for example, the `./raioz` artifact left over from `make build`
    # in the repo root would silently override the downloaded release.
    # The cwd fallback (`./raioz`) is intentionally dropped: there is
    # no legitimate use case it covers that the script_dir check does
    # not, and it was the source of the bug.
    local script_dir=""
    if [ -n "${BASH_SOURCE[0]:-}" ] && [ -f "${BASH_SOURCE[0]}" ]; then
        script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    fi

    if [ -n "$script_dir" ] \
        && [ -f "${script_dir}/raioz" ] \
        && [ -x "${script_dir}/raioz" ]; then
        binary_path="${script_dir}/raioz"
        info "Development mode: using local binary at ${binary_path}"
    fi

    if [ -z "$binary_path" ]; then
        # Production mode: download from GitHub releases
        local os arch version
        os=$(detect_os)
        arch=$(detect_arch)
        info "Platform: ${os}/${arch}"

        version=$(get_latest_version)
        info "Version: ${version}"
        echo ""

        check_dependencies

        binary_path=$(mktemp)
        download_release "$version" "$os" "$arch" "$binary_path"
    else
        echo ""
        check_dependencies
    fi

    install_binary "$binary_path"

    verify_install
}

# verify_install reports whether `command -v raioz` resolves to the
# binary we just installed, or whether an older copy elsewhere on PATH
# is shadowing it. Output is informational; we don't fail the install
# because the user can still invoke the new binary by full path while
# they clean up.
verify_install() {
    echo ""
    local target="${INSTALL_DIR}/${BINARY_NAME}"
    local resolved
    resolved="$(command -v "$BINARY_NAME" 2>/dev/null || true)"

    if [ -z "$resolved" ]; then
        warn "raioz installed to ${target} but is not on your \$PATH"
        echo ""
        echo "  Add to your PATH:"
        echo "    export PATH=\"${INSTALL_DIR}:\$PATH\""
        return 0
    fi

    local resolved_real target_real
    resolved_real="$(readlink -f "$resolved" 2>/dev/null || echo "$resolved")"
    target_real="$(readlink -f "$target" 2>/dev/null || echo "$target")"

    if [ "$resolved_real" != "$target_real" ]; then
        warn "raioz installed to ${target} but 'command -v raioz' resolves to: ${resolved}"
        warn "The freshly installed binary is shadowed by an older copy."
        echo ""
        echo "  Remove the shadowing copy:"
        echo "    rm '${resolved}'"
        echo ""
        echo "  Or re-install over the shadowing copy:"
        echo "    INSTALL_DIR='$(dirname "$resolved")' bash install.sh"
        echo ""
        echo "  Until then, invoke the new binary explicitly:"
        echo "    ${target} version"
        return 0
    fi

    local installed_version
    installed_version=$("$BINARY_NAME" version 2>/dev/null \
        | head -n 1 || echo "installed")
    ok "raioz installed successfully!"
    echo ""
    echo "  Version:  ${installed_version}"
    echo "  Location: ${resolved}"
    echo ""
    info "Run 'raioz --help' to get started"
}

main
