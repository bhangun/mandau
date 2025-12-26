#!/bin/bash

# Mandau Installation Script
# This script automatically detects your platform and installs the appropriate Mandau binaries

# Don't exit immediately on error - we'll handle errors explicitly
# set -e is too aggressive for this script

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
    # The version from GitHub API includes 'v' prefix (e.g., v0.0.7) but release assets use version without 'v'
    VERSION_NO_V="${version#v}"  # Strip the 'v' prefix for the filename
    if [ "$os" = "windows" ]; then
        # Windows uses .zip format
        FILENAME="mandau-windows-${arch}-${VERSION_NO_V}.zip"
        URL="https://github.com/bhangun/mandau/releases/download/${version}/${FILENAME}"
    else
        # Linux/macOS use .tar.gz format
        FILENAME="mandau-${os}-${arch}-${VERSION_NO_V}.tar.gz"
        URL="https://github.com/bhangun/mandau/releases/download/${version}/${FILENAME}"
    fi

    print_status "Downloading Mandau binaries from: $URL"
    
    # Create temporary directory
    TEMP_DIR=$(mktemp -d)
    cd "$TEMP_DIR"
    
    # Download the file - temporarily disable set -e for this operation
    set +e
    if command -v wget >/dev/null 2>&1; then
        print_status "Using wget to download $URL"
        wget -q "$URL" -O "$FILENAME"
        DOWNLOAD_STATUS=$?
    elif command -v curl >/dev/null 2>&1; then
        print_status "Using curl to download $URL"
        curl -fsSL -o "$FILENAME" "$URL"
        DOWNLOAD_STATUS=$?
    else
        print_error "Neither wget nor curl is available"
        rm -rf "$TEMP_DIR"
        exit 1
    fi
    set -e

    if [ $DOWNLOAD_STATUS -ne 0 ]; then
        print_error "Failed to download $FILENAME (exit code: $DOWNLOAD_STATUS)"
        print_error "URL attempted: $URL"
        ls -la "$TEMP_DIR"  # Show what's in the temp directory
        rm -rf "$TEMP_DIR"
        exit 1
    fi

    print_status "Downloaded $FILENAME successfully"
    print_status "File size: $(ls -lah $FILENAME | awk '{print $5}')"

    # Extract the archive
    print_status "Extracting $FILENAME..."
    set +e
    if [[ "$FILENAME" == *.tar.gz ]]; then
        tar -xzf "$FILENAME"
        EXTRACT_STATUS=$?
    elif [[ "$FILENAME" == *.zip ]]; then
        unzip -q "$FILENAME"
        EXTRACT_STATUS=$?
    else
        print_error "Unknown archive format: $FILENAME"
        EXTRACT_STATUS=1
    fi
    set -e

    if [ $EXTRACT_STATUS -ne 0 ]; then
        print_error "Failed to extract $FILENAME (exit code: $EXTRACT_STATUS)"
        ls -la  # Show contents after failed extraction
        rm -rf "$TEMP_DIR"
        exit 1
    fi

    print_status "Extraction completed successfully"
    print_status "Contents of extraction directory:"
    ls -la

    # Make binaries executable (not needed for Windows)
    if [ "$os" != "windows" ]; then
        print_status "Making binaries executable..."
        set +e
        chmod +x mandau mandau-core mandau-agent
        CHMOD_STATUS=$?
        set -e
        if [ $CHMOD_STATUS -ne 0 ]; then
            print_error "Failed to make binaries executable (exit code: $CHMOD_STATUS)"
            ls -la
            rm -rf "$TEMP_DIR"
            exit 1
        fi
        print_status "Binaries made executable successfully"
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
            set +e
            $SUDO install -m 755 mandau mandau-core mandau-agent /usr/local/bin/
            INSTALL_STATUS=$?
            set -e
            if [ $INSTALL_STATUS -ne 0 ]; then
                # Fallback to cp if install command fails
                print_warning "install command failed, trying cp as fallback..."
                set +e
                $SUDO cp mandau mandau-core mandau-agent /usr/local/bin/
                $SUDO chmod 755 /usr/local/bin/mandau /usr/local/bin/mandau-core /usr/local/bin/mandau-agent
                CP_STATUS=$?
                set -e
                if [ $CP_STATUS -ne 0 ]; then
                    print_error "Both install and cp commands failed"
                    cd - >/dev/null
                    rm -rf "$TEMP_DIR"
                    exit 1
                fi
            fi
        elif [ "$EUID" -eq 0 ]; then
            # Running as root (e.g., via curl | sudo bash), install directly
            set +e
            install -m 755 mandau mandau-core mandau-agent /usr/local/bin/
            INSTALL_STATUS=$?
            set -e
            if [ $INSTALL_STATUS -ne 0 ]; then
                # Fallback to cp if install command fails
                print_warning "install command failed, trying cp as fallback..."
                set +e
                cp mandau mandau-core mandau-agent /usr/local/bin/
                chmod 755 /usr/local/bin/mandau /usr/local/bin/mandau-core /usr/local/bin/mandau-agent
                CP_STATUS=$?
                set -e
                if [ $CP_STATUS -ne 0 ]; then
                    print_error "Both install and cp commands failed"
                    cd - >/dev/null
                    rm -rf "$TEMP_DIR"
                    exit 1
                fi
            fi
        else
            # Try to install without sudo (might fail)
            set +e
            install -m 755 mandau mandau-core mandau-agent /usr/local/bin/ 2>/dev/null
            INSTALL_STATUS=$?
            set -e
            if [ $INSTALL_STATUS -ne 0 ]; then
                # Fallback to cp if install command fails
                print_warning "install command failed, trying cp as fallback..."
                if [ -n "$SUDO" ]; then
                    set +e
                    $SUDO cp mandau mandau-core mandau-agent /usr/local/bin/
                    $SUDO chmod 755 /usr/local/bin/mandau /usr/local/bin/mandau-core /usr/local/bin/mandau-agent
                    CP_STATUS=$?
                    set -e
                    if [ $CP_STATUS -ne 0 ]; then
                        print_error "Both install and cp commands failed"
                        cd - >/dev/null
                        rm -rf "$TEMP_DIR"
                        exit 1
                    fi
                else
                    set +e
                    cp mandau mandau-core mandau-agent /usr/local/bin/ 2>/dev/null
                    CP_STATUS=$?
                    set -e
                    if [ $CP_STATUS -ne 0 ]; then
                        print_error "Installation to /usr/local/bin requires sudo. Please run with sudo or install manually."
                        cd - >/dev/null
                        rm -rf "$TEMP_DIR"
                        exit 1
                    fi
                fi
            fi
        fi
    else
        # Windows: Copy to a user-accessible location
        # This is a simplified approach - Windows installation would need more work
        print_warning "Windows installation requires manual copying of .exe files to PATH"
        print_status "Binaries extracted to: $TEMP_DIR"
    fi

    print_success "Mandau binaries installed successfully!"

    # Create default configuration directory and file
    print_status "Creating default configuration..."

    # Determine the original user (in case running with sudo)
    if [ -n "$SUDO_USER" ]; then
        ORIGINAL_USER="$SUDO_USER"
    else
        ORIGINAL_USER="$(whoami)"
    fi

    # Get the home directory for the original user
    ORIGINAL_HOME=$(eval echo ~$ORIGINAL_USER)
    CONFIG_DIR="$ORIGINAL_HOME/.mandau"

    # Create config directory with appropriate permissions
    mkdir -p "$CONFIG_DIR"

    # Create default config file
    cat > "$CONFIG_DIR/config.yaml" << EOF
# Mandau CLI Configuration
server: "localhost:8443"
cert: "$ORIGINAL_HOME/mandau-certs/client.crt"
key: "$ORIGINAL_HOME/mandau-certs/client.key"
ca: "$ORIGINAL_HOME/mandau-certs/ca.crt"
timeout: "30s"
# Note: Certificates need to be generated separately using the generate-certs.sh script
EOF

    # Set appropriate permissions for the config directory and file
    chown -R "$ORIGINAL_USER:$ORIGINAL_USER" "$CONFIG_DIR"
    chmod 700 "$CONFIG_DIR"
    chmod 600 "$CONFIG_DIR/config.yaml"

    print_success "Default configuration created at $CONFIG_DIR/config.yaml"
    print_status "Note: You need to generate certificates in ~/mandau-certs/ directory for full functionality."

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
        print_status "Checking if binaries exist in /usr/local/bin/:"
        if [ -f "/usr/local/bin/mandau" ]; then
            print_status "  - mandau binary exists in /usr/local/bin/"
            ls -la /usr/local/bin/mandau 2>/dev/null || echo "  - Cannot access /usr/local/bin/mandau"
        else
            print_status "  - mandau binary does NOT exist in /usr/local/bin/"
        fi
        if [ -f "/usr/local/bin/mandau-core" ]; then
            print_status "  - mandau-core binary exists in /usr/local/bin/"
        else
            print_status "  - mandau-core binary does NOT exist in /usr/local/bin/"
        fi
        if [ -f "/usr/local/bin/mandau-agent" ]; then
            print_status "  - mandau-agent binary exists in /usr/local/bin/"
        else
            print_status "  - mandau-agent binary does NOT exist in /usr/local/bin/"
        fi
    fi

    print_status "Installation complete! Run 'mandau --help' to get started."
}

# Run main function
main "$@"