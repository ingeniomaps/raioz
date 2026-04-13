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

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="raioz"
GITHUB_REPO="${GITHUB_REPO:-ingeniomaps/raioz}"
GITHUB_URL="https://github.com/${GITHUB_REPO}"

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

    # Development mode: local binary exists next to install.sh or in cwd
    local script_dir
    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

    if [ -f "${script_dir}/raioz" ] && [ -x "${script_dir}/raioz" ]; then
        binary_path="${script_dir}/raioz"
        info "Development mode: using local binary"
    elif [ -f "./raioz" ] && [ -x "./raioz" ]; then
        binary_path="./raioz"
        info "Development mode: using local binary"
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

    # Verify
    echo ""
    if command -v "$BINARY_NAME" >/dev/null 2>&1; then
        local installed_version
        installed_version=$($BINARY_NAME version 2>/dev/null \
            | head -n 1 || echo "installed")
        ok "raioz installed successfully!"
        echo ""
        echo "  Version:  ${installed_version}"
        echo "  Location: $(command -v "$BINARY_NAME")"
        echo ""
        info "Run 'raioz --help' to get started"
    else
        warn "raioz installed to ${INSTALL_DIR} but not found in PATH"
        echo ""
        echo "  Add to your PATH:"
        echo "    export PATH=\"${INSTALL_DIR}:\$PATH\""
    fi
}

main
