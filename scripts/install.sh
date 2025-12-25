#!/bin/bash

# Mandau Installation Script
# This script automatically detects your platform and installs the appropriate Mandau binaries

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

# Detect OS and architecture
detect_platform() {
    OS=""
    ARCH=""

    # Detect OS
    case "$(uname -s)" in
        Linux*)
            OS="linux"
            ;;
        Darwin*)
            OS="darwin"
            ;;
        *)
            print_error "Unsupported operating system: $(uname -s)"
            exit 1
            ;;
    esac

    # Detect architecture
    case "$(uname -m)" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            print_error "Unsupported architecture: $(uname -m)"
            exit 1
            ;;
    esac

    print_status "Detected platform: ${OS}/${ARCH}"
}

# Get the latest release version from GitHub API
get_latest_version() {
    print_status "Fetching latest release version..."
    
    # Use GitHub API to get the latest release
    LATEST_VERSION=$(curl -s "https://api.github.com/repos/bhangun/mandau/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    
    if [ -z "$LATEST_VERSION" ]; then
        print_error "Failed to fetch latest version from GitHub API"
        # Fallback to a default version
        LATEST_VERSION="v1.0.0"
        print_warning "Using fallback version: $LATEST_VERSION"
    else
        print_status "Latest version: $LATEST_VERSION"
    fi
}

# Download and install binaries
download_and_install() {
    local version=$1
    local os=$2
    local arch=$3

    # Construct download URL based on platform
    if [ "$os" = "windows" ]; then
        # Windows uses .zip format
        FILENAME="mandau-windows-${arch}-${version}.zip"
        URL="https://github.com/bhangun/mandau/releases/download/${version}/${FILENAME}"
    else
        # Linux/macOS use .tar.gz format
        FILENAME="mandau-${os}-${arch}-${version}.tar.gz"
        URL="https://github.com/bhangun/mandau/releases/download/${version}/${FILENAME}"
    fi

    print_status "Downloading Mandau binaries from: $URL"
    
    # Create temporary directory
    TEMP_DIR=$(mktemp -d)
    cd "$TEMP_DIR"
    
    # Download the file
    if command -v wget >/dev/null 2>&1; then
        wget -q "$URL" -O "$FILENAME"
    elif command -v curl >/dev/null 2>&1; then
        curl -fsSL -o "$FILENAME" "$URL"
    else
        print_error "Neither wget nor curl is available"
        exit 1
    fi

    if [ $? -ne 0 ]; then
        print_error "Failed to download $FILENAME"
        rm -rf "$TEMP_DIR"
        exit 1
    fi

    print_status "Downloaded $FILENAME successfully"

    # Extract the archive
    if [[ "$FILENAME" == *.tar.gz ]]; then
        tar -xzf "$FILENAME"
    elif [[ "$FILENAME" == *.zip ]]; then
        unzip -q "$FILENAME"
    fi

    # Make binaries executable (not needed for Windows)
    if [ "$os" != "windows" ]; then
        chmod +x mandau mandau-core mandau-agent
    fi

    # Install to system location
    if [ "$os" != "windows" ]; then
        # Check if we have sudo access
        if command -v sudo >/dev/null 2>&1; then
            SUDO="sudo"
        else
            SUDO=""
        fi

        # Verify binaries exist before installing
        if [ ! -f "mandau" ] || [ ! -f "mandau-core" ] || [ ! -f "mandau-agent" ]; then
            print_error "Required binaries not found in extracted archive. Available files:"
            ls -la
            cd - >/dev/null
            rm -rf "$TEMP_DIR"
            exit 1
        fi

        # Install binaries
        if [ -n "$SUDO" ] && [ "$EUID" -ne 0 ]; then
            # Use sudo if available and not running as root
            $SUDO install -m 755 mandau mandau-core mandau-agent /usr/local/bin/
        elif [ "$EUID" -eq 0 ]; then
            # Running as root (e.g., via curl | sudo bash), install directly
            install -m 755 mandau mandau-core mandau-agent /usr/local/bin/
        else
            # Try to install without sudo (might fail)
            install -m 755 mandau mandau-core mandau-agent /usr/local/bin/ 2>/dev/null || {
                print_error "Installation to /usr/local/bin requires sudo. Please run with sudo or install manually."
                cd - >/dev/null
                rm -rf "$TEMP_DIR"
                exit 1
            }
        fi
    else
        # Windows: Copy to a user-accessible location
        # This is a simplified approach - Windows installation would need more work
        print_warning "Windows installation requires manual copying of .exe files to PATH"
        print_status "Binaries extracted to: $TEMP_DIR"
    fi

    print_success "Mandau binaries installed successfully!"
    
    # Return to original directory and cleanup
    cd - >/dev/null
    rm -rf "$TEMP_DIR"
}

# Main execution
main() {
    print_status "Starting Mandau installation..."

    # Detect platform
    detect_platform

    # Get latest version
    get_latest_version

    # Download and install
    download_and_install "$LATEST_VERSION" "$OS" "$ARCH"

    # Verify installation
    if command -v mandau >/dev/null 2>&1; then
        print_success "Mandau CLI is available: $(mandau --version 2>/dev/null || echo "version info not available")"
    else
        print_warning "Mandau CLI not found in PATH after installation"
    fi

    print_status "Installation complete! Run 'mandau --help' to get started."
}

# Run main function
main "$@"