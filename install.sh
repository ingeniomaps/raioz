#!/bin/bash

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="raioz"
GITHUB_REPO="${GITHUB_REPO:-raioz/raioz}"  # Update with actual GitHub repo
GITHUB_URL="https://github.com/${GITHUB_REPO}"

# Print colored message
print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

print_success() {
    echo -e "${GREEN}✔${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1" >&2
}

print_error() {
    echo -e "${RED}✗${NC} $1" >&2
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Detect OS
detect_os() {
    local os
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"

    case "$os" in
        linux*)
            echo "linux"
            ;;
        darwin*)
            echo "darwin"
            ;;
        *)
            print_error "Unsupported OS: $os"
            echo "Supported OS: Linux, macOS"
            exit 1
            ;;
    esac
}

# Detect architecture
detect_arch() {
    local arch
    arch="$(uname -m)"

    case "$arch" in
        x86_64|amd64)
            echo "amd64"
            ;;
        arm64|aarch64)
            echo "arm64"
            ;;
        armv7l|armv6l)
            echo "armv7"
            ;;
        *)
            print_error "Unsupported architecture: $arch"
            echo "Supported architectures: amd64, arm64, armv7"
            exit 1
            ;;
    esac
}

# Get latest release version
get_latest_version() {
    local version
    if command_exists curl; then
        version=$(curl -sL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' || echo "")
    elif command_exists wget; then
        version=$(wget -qO- "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' || echo "")
    fi

    if [ -z "$version" ]; then
        print_warning "Could not fetch latest version, using 'latest'"
        echo "latest"
    else
        echo "$version"
    fi
}

# Download binary
download_binary() {
    local version=$1
    local os=$2
    local arch=$3
    local output=$4

    local url
    if [ "$version" = "latest" ]; then
        url="${GITHUB_URL}/releases/latest/download/raioz-${os}-${arch}"
    else
        url="${GITHUB_URL}/releases/download/${version}/raioz-${os}-${arch}"
    fi

    print_info "Downloading raioz from GitHub releases..."
    print_info "URL: $url"

    if command_exists curl; then
        if ! curl -fL -o "$output" "$url"; then
            print_error "Failed to download binary"
            print_info "If you're building from source, use: make build && sudo make install"
            exit 1
        fi
    elif command_exists wget; then
        if ! wget -qO "$output" "$url"; then
            print_error "Failed to download binary"
            print_info "If you're building from source, use: make build && sudo make install"
            exit 1
        fi
    else
        print_error "Neither curl nor wget is available"
        exit 1
    fi
}

# Verify dependencies
check_dependencies() {
    local missing=0

    print_info "Checking dependencies..."

    if ! command_exists docker; then
        print_warning "Docker is not installed"
        print_info "Install Docker: https://docs.docker.com/get-docker/"
        missing=1
    else
        if ! docker info >/dev/null 2>&1; then
            print_warning "Docker is installed but not running"
            print_info "Start Docker daemon and try again"
        else
            print_success "Docker is installed and running"
        fi
    fi

    if ! command_exists git; then
        print_warning "Git is not installed"
        print_info "Install Git: https://git-scm.com/downloads"
        missing=1
    else
        print_success "Git is installed"
    fi

    if [ $missing -eq 1 ]; then
        print_warning "Some dependencies are missing, but installation will continue"
        echo ""
    fi
}

# Check if running from development directory (has local binary)
is_development_mode() {
    # Check if we're in the raioz repo and there's a local binary
    local script_dir
    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

    # If there's a 'raioz' binary in the same directory as install.sh, use it
    if [ -f "$script_dir/raioz" ] && [ -x "$script_dir/raioz" ]; then
        return 0
    fi

    # Also check current directory (for make install)
    if [ -f "./raioz" ] && [ -x "./raioz" ]; then
        return 0
    fi

    return 1
}

# Main installation function
main() {
    print_info "Installing raioz..."
    echo ""

    # Check if we're in development mode (local binary exists)
    local binary_path=""
    local use_local=false

    if is_development_mode; then
        # Find local binary
        local script_dir
        script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

        if [ -f "$script_dir/raioz" ] && [ -x "$script_dir/raioz" ]; then
            binary_path="$script_dir/raioz"
            use_local=true
        elif [ -f "./raioz" ] && [ -x "./raioz" ]; then
            binary_path="./raioz"
            use_local=true
        fi
    fi

    if [ "$use_local" = true ] && [ -n "$binary_path" ]; then
        # Development mode: use local binary
        print_info "Development mode detected: using local binary"
        print_info "Binary path: $binary_path"

        # Get version from local binary
        local installed_version
        if "$binary_path" version >/dev/null 2>&1; then
            installed_version=$("$binary_path" version 2>/dev/null | head -n 1 || echo "local build")
        else
            installed_version="local build"
        fi
        print_info "Version: $installed_version"
        echo ""

        # Check dependencies
        check_dependencies
        echo ""

        # Install binary
        print_info "Installing to $INSTALL_DIR..."

        if [ ! -w "$INSTALL_DIR" ]; then
            print_info "Using sudo to install (required for $INSTALL_DIR)"
            sudo mkdir -p "$INSTALL_DIR"
            sudo cp "$binary_path" "$INSTALL_DIR/${BINARY_NAME}"
            sudo chmod +x "$INSTALL_DIR/${BINARY_NAME}"
        else
            mkdir -p "$INSTALL_DIR"
            cp "$binary_path" "$INSTALL_DIR/${BINARY_NAME}"
            chmod +x "$INSTALL_DIR/${BINARY_NAME}"
        fi
    else
        # Production mode: download from GitHub
        # Detect OS and architecture
        local os
        local arch
        os=$(detect_os)
        arch=$(detect_arch)

        print_info "Detected OS: $os"
        print_info "Detected architecture: $arch"
        echo ""

        # Get version
        local version
        version=$(get_latest_version)
        print_info "Installing version: $version"
        echo ""

        # Check dependencies
        check_dependencies
        echo ""

        # Create temp directory
        local tmp_dir
        tmp_dir=$(mktemp -d)
        trap "rm -rf $tmp_dir" EXIT

        binary_path="${tmp_dir}/${BINARY_NAME}"

        # Download binary
        download_binary "$version" "$os" "$arch" "$binary_path"

        # Make binary executable
        chmod +x "$binary_path"

        # Verify binary (optional: check if it runs)
        print_info "Verifying binary..."
        if "$binary_path" version >/dev/null 2>&1; then
            print_success "Binary is valid"
        else
            print_warning "Could not verify binary (this may be normal)"
        fi
        echo ""

        # Install binary
        print_info "Installing to $INSTALL_DIR..."

        if [ ! -w "$INSTALL_DIR" ]; then
            print_info "Using sudo to install (required for $INSTALL_DIR)"
            sudo mkdir -p "$INSTALL_DIR"
            sudo mv "$binary_path" "$INSTALL_DIR/${BINARY_NAME}"
            sudo chmod +x "$INSTALL_DIR/${BINARY_NAME}"
        else
            mkdir -p "$INSTALL_DIR"
            mv "$binary_path" "$INSTALL_DIR/${BINARY_NAME}"
            chmod +x "$INSTALL_DIR/${BINARY_NAME}"
        fi
    fi

    echo ""

    # Verify installation
    if command_exists "$BINARY_NAME"; then
        local installed_version
        installed_version=$($BINARY_NAME version 2>/dev/null | head -n 1 || echo "installed")
        print_success "raioz installed successfully!"
        echo ""
        echo "  Version: $installed_version"
        echo "  Location: $(command -v $BINARY_NAME)"
        echo ""
        print_info "Run 'raioz --help' to get started"
        echo ""
    else
        print_warning "raioz may not be in your PATH"
        echo ""
        echo "  Installed to: $INSTALL_DIR/${BINARY_NAME}"
        echo "  Add $INSTALL_DIR to your PATH or use: $INSTALL_DIR/${BINARY_NAME}"
        echo ""

        # Check common shell config files
        local shell_rc=""
        if [ -n "${ZSH_VERSION:-}" ]; then
            shell_rc="$HOME/.zshrc"
        elif [ -n "${BASH_VERSION:-}" ]; then
            shell_rc="$HOME/.bashrc"
        fi

        if [ -n "$shell_rc" ] && [ -f "$shell_rc" ]; then
            if ! grep -q "$INSTALL_DIR" "$shell_rc" 2>/dev/null; then
                print_info "To add $INSTALL_DIR to your PATH, run:"
                echo "  echo 'export PATH=\"$INSTALL_DIR:\$PATH\"' >> $shell_rc"
                echo "  source $shell_rc"
                echo ""
            fi
        fi
    fi
}

# Run main function
main
