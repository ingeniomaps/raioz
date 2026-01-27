#!/bin/bash
# This script requires bash (not sh/dash)
# If executed with sh, show error message
if [ -z "${BASH_VERSION:-}" ]; then
    echo "Error: This script requires bash." >&2
    echo "" >&2
    echo "Please use:" >&2
    echo "  curl -fsSL https://raw.githubusercontent.com/ingeniomaps/raioz/develop/install.sh | bash" >&2
    echo "" >&2
    echo "Instead of:" >&2
    echo "  curl -fsSL ... | sh" >&2
    exit 1
fi

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
GITHUB_REPO="${GITHUB_REPO:-ingeniomaps/raioz}"  # Update with actual GitHub repo
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

    local tmp_dir
    tmp_dir=$(dirname "$output")
    local download_success=false

    # Try direct binary first (for manual releases)
    local binary_url
    if [ "$version" = "latest" ]; then
        binary_url="${GITHUB_URL}/releases/latest/download/raioz-${os}-${arch}"
    else
        binary_url="${GITHUB_URL}/releases/download/${version}/raioz-${os}-${arch}"
    fi

    print_info "Downloading raioz from GitHub releases..."
    print_info "Trying direct binary: $binary_url"

    if command_exists curl; then
        if curl -fL -o "$output" "$binary_url" 2>/dev/null; then
            download_success=true
        fi
    elif command_exists wget; then
        if wget -qO "$output" "$binary_url" 2>/dev/null; then
            download_success=true
        fi
    fi

    # If direct binary failed, try compressed archive (GoReleaser format)
    if [ "$download_success" = false ]; then
        local archive_ext="tar.gz"
        if [ "$os" = "windows" ]; then
            archive_ext="zip"
        fi

        # Get actual version if "latest"
        local actual_version="$version"
        if [ "$version" = "latest" ]; then
            actual_version=$(get_latest_version)
            # If we still can't get version, try to fetch from API
            if [ "$actual_version" = "latest" ] || [ -z "$actual_version" ]; then
                print_info "Fetching latest release info from GitHub API..."
                if command_exists curl; then
                    actual_version=$(curl -sL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' | sed 's/^v//' || echo "")
                elif command_exists wget; then
                    actual_version=$(wget -qO- "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' | sed 's/^v//' || echo "")
                fi
            fi
            # Remove 'v' prefix if present
            actual_version=$(echo "$actual_version" | sed 's/^v//')
        else
            # Remove 'v' prefix if present
            actual_version=$(echo "$version" | sed 's/^v//')
        fi

        local archive_name="raioz_${actual_version}_${os}_${arch}.${archive_ext}"

        local archive_url
        if [ "$version" = "latest" ]; then
            archive_url="${GITHUB_URL}/releases/latest/download/${archive_name}"
        else
            archive_url="${GITHUB_URL}/releases/download/${version}/${archive_name}"
        fi

        print_info "Direct binary not found, trying archive: $archive_url"

        local archive_path="${tmp_dir}/${archive_name}"

        if command_exists curl; then
            if curl -fL -o "$archive_path" "$archive_url" 2>/dev/null; then
                download_success=true
            fi
        elif command_exists wget; then
            if wget -qO "$archive_path" "$archive_url" 2>/dev/null; then
                download_success=true
            fi
        fi

        if [ "$download_success" = true ]; then
            # Extract the binary from archive
            print_info "Extracting binary from archive..."
            if [ "$archive_ext" = "tar.gz" ]; then
                if command_exists tar; then
                    # Extract all files first
                    tar -xzf "$archive_path" -C "$tmp_dir" 2>/dev/null || {
                        print_error "Failed to extract archive"
                        download_success=false
                    }

                    if [ "$download_success" = true ]; then
                        # Find the binary (could be in root or subdirectory)
                        local found_binary
                        found_binary=$(find "$tmp_dir" -name "raioz" -type f | head -n 1)
                        if [ -n "$found_binary" ] && [ -f "$found_binary" ]; then
                            mv "$found_binary" "$output" 2>/dev/null || cp "$found_binary" "$output" 2>/dev/null
                            chmod +x "$output" 2>/dev/null
                        else
                            print_error "Binary 'raioz' not found in archive"
                            download_success=false
                        fi
                    fi
                else
                    print_error "tar is required to extract the archive"
                    exit 1
                fi
            elif [ "$archive_ext" = "zip" ]; then
                if command_exists unzip; then
                    unzip -q "$archive_path" -d "$tmp_dir" 2>/dev/null || {
                        print_error "Failed to extract archive"
                        download_success=false
                    }

                    if [ "$download_success" = true ]; then
                        # Find the binary (could be .exe on Windows or just raioz)
                        local found_binary
                        found_binary=$(find "$tmp_dir" \( -name "raioz.exe" -o -name "raioz" \) -type f | head -n 1)
                        if [ -n "$found_binary" ] && [ -f "$found_binary" ]; then
                            mv "$found_binary" "$output" 2>/dev/null || cp "$found_binary" "$output" 2>/dev/null
                            chmod +x "$output" 2>/dev/null
                        else
                            print_error "Binary 'raioz' not found in archive"
                            download_success=false
                        fi
                    fi
                else
                    print_error "unzip is required to extract the archive"
            exit 1
        fi
            fi

            # Clean up archive and any extracted directories
            rm -f "$archive_path" 2>/dev/null
            # Clean up any extracted directories (but keep the binary we moved)
            find "$tmp_dir" -type d -mindepth 1 -maxdepth 1 -exec rm -rf {} + 2>/dev/null || true

            # Verify we got the binary
            if [ "$download_success" = true ] && ([ ! -f "$output" ] || [ ! -x "$output" ]); then
                download_success=false
            fi
        fi
    fi

    if [ "$download_success" = false ]; then
        print_error "Failed to download binary"
        print_info "Tried both direct binary and compressed archive formats"
        print_info "If you're building from source, use: make build && sudo make install"
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
