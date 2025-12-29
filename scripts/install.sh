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
PURPLE='\033[0;35m'
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

print_dev() {
    echo -e "${PURPLE}[DEV]${NC} $1"
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

# Generate certificates if they don't exist
generate_certificates() {
    local cert_dir="$1"
    local original_user="$2"
    
    if [ -d "$cert_dir" ] && [ -f "$cert_dir/ca.crt" ]; then
        print_status "Certificates already exist in $cert_dir, skipping generation"
        return 0
    fi

    print_status "Generating certificates in $cert_dir..."
    
    # Create the certificates directory with proper permissions
    mkdir -p "$cert_dir"
    chown "$original_user:$original_user" "$cert_dir"
    chmod 700 "$cert_dir"  # Restrict access to owner only

    # Generate CA certificate
    openssl genrsa -out "$cert_dir/ca.key" 4096
    openssl req -new -x509 -days 3650 -key "$cert_dir/ca.key" \
        -out "$cert_dir/ca.crt" \
        -subj "/CN=Mandau CA/O=Mandau/C=US" -nodes

    # Generate Core certificate
    openssl genrsa -out "$cert_dir/core.key" 4096
    openssl req -new -key "$cert_dir/core.key" \
        -out "$cert_dir/core.csr" \
        -subj "/CN=mandau-core/O=Mandau/C=US" -nodes

    cat > "$cert_dir/core.ext" <<EOF
subjectAltName = DNS:mandau-core,DNS:localhost,IP:127.0.0.1
extendedKeyUsage = serverAuth,clientAuth
EOF

    openssl x509 -req -in "$cert_dir/core.csr" \
        -CA "$cert_dir/ca.crt" -CAkey "$cert_dir/ca.key" \
        -CAcreateserial -out "$cert_dir/core.crt" \
        -days 365 -extfile "$cert_dir/core.ext"

    # Generate Agent certificate
    openssl genrsa -out "$cert_dir/agent.key" 4096
    openssl req -new -key "$cert_dir/agent.key" \
        -out "$cert_dir/agent.csr" \
        -subj "/CN=mandau-agent/O=Mandau/C=US" -nodes

    cat > "$cert_dir/agent.ext" <<EOF
subjectAltName = DNS:mandau-agent,DNS:localhost,IP:127.0.0.1
extendedKeyUsage = serverAuth,clientAuth
EOF

    openssl x509 -req -in "$cert_dir/agent.csr" \
        -CA "$cert_dir/ca.crt" -CAkey "$cert_dir/ca.key" \
        -CAcreateserial -out "$cert_dir/agent.crt" \
        -days 365 -extfile "$cert_dir/agent.ext"

    # Generate CLI client certificate
    openssl genrsa -out "$cert_dir/client.key" 4096
    openssl req -new -key "$cert_dir/client.key" \
        -out "$cert_dir/client.csr" \
        -subj "/CN=mandau-cli/O=Mandau/C=US" -nodes

    cat > "$cert_dir/client.ext" <<EOF
extendedKeyUsage = clientAuth
EOF

    openssl x509 -req -in "$cert_dir/client.csr" \
        -CA "$cert_dir/ca.crt" -CAkey "$cert_dir/ca.key" \
        -CAcreateserial -out "$cert_dir/client.crt" \
        -days 365 -extfile "$cert_dir/client.ext"

    # Set proper permissions
    chmod 600 "$cert_dir"/*.key
    chmod 644 "$cert_dir"/*.crt "$cert_dir"/*.ext "$cert_dir"/ca.srl

    # Set ownership to the original user
    chown "$original_user:$original_user" "$cert_dir"/*
    
    print_success "Certificates generated in $cert_dir"
}

# Create systemd service files with absolute paths
create_systemd_services() {
    local original_user="$1"
    local original_home="$2"
    
    print_status "Creating systemd service files with absolute paths..."
    
    # Create mandau-core.service
    cat > "/tmp/mandau-core.service" << EOF
[Unit]
Description=Mandau Core Service
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User=$original_user
Group=$original_user
ExecStart=/usr/local/bin/mandau-core --listen :8443 --cert $original_home/mandau-certs/core.crt --key $original_home/mandau-certs/core.key --ca $original_home/mandau-certs/ca.crt
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$original_home/mandau-certs $original_home/mandau-stacks /tmp

[Install]
WantedBy=multi-user.target
EOF

    # Create mandau-agent.service
    cat > "/tmp/mandau-agent.service" << EOF
[Unit]
Description=Mandau Agent Service
After=network.target mandau-core.service
Wants=mandau-core.service

[Service]
Type=simple
User=$original_user
Group=$original_user
ExecStart=/usr/local/bin/mandau-agent --server localhost:8443 --cert $original_home/mandau-certs/agent.crt --key $original_home/mandau-certs/agent.key --ca $original_home/mandau-certs/ca.crt --stack-root $original_home/mandau-stacks
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$original_home/mandau-certs $original_home/mandau-stacks /var/run/docker.sock /tmp

[Install]
WantedBy=multi-user.target
EOF

    # Copy service files to system location with sudo
    if command -v sudo >/dev/null 2>&1; then
        SUDO="sudo"
    else
        SUDO=""
    fi

    if [ -n "$SUDO" ]; then
        $SUDO cp /tmp/mandau-core.service /etc/systemd/system/
        $SUDO cp /tmp/mandau-agent.service /etc/systemd/system/
        $SUDO systemctl daemon-reload
        print_success "Systemd service files created and loaded"
    else
        print_warning "Cannot create systemd service files without sudo access"
        print_status "You'll need to manually create the service files with absolute paths"
    fi
}

# Create default configuration with profile support
create_default_config() {
    local original_user="$1"
    local original_home="$2"
    
    print_status "Creating default configuration with profile support..."
    
    CONFIG_DIR="$original_home/.mandau"
    mkdir -p "$CONFIG_DIR"

    # Create main config file with profile support
    cat > "$CONFIG_DIR/config.yaml" << EOF
# Mandau Unified Configuration with Profile Support
# This is the default profile (production)
server:
  listen_addr: "localhost:8443"  # For core server: address to listen on; For client: remote server address
  tls:
    cert_path: "$original_home/mandau-certs/client.crt"  # Client certificate
    key_path: "$original_home/mandau-certs/client.key"   # Client key
    ca_path: "$original_home/mandau-certs/ca.crt"        # CA certificate
    min_version: "TLS1.3"
    server_name: "mandau-core"
timeout: "30s"
# For remote server usage, update server.listen_addr to your remote server (e.g., "myserver.com:8443")
EOF

    # Create development profile
    cat > "$CONFIG_DIR/dev.yaml" << EOF
# Development Profile Configuration
server:
  listen_addr: "localhost:8443"
  tls:
    cert_path: "$original_home/mandau-certs/client.crt"
    key_path: "$original_home/mandau-certs/client.key"
    ca_path: "$original_home/mandau-certs/ca.crt"
    min_version: "TLS1.3"
    server_name: "mandau-core"
timeout: "30s"
# Development-specific settings can go here
EOF

    # Create local development profile for testing
    cat > "$CONFIG_DIR/local.yaml" << EOF
# Local Development Profile Configuration
server:
  listen_addr: "localhost:8443"
  tls:
    cert_path: "$original_home/mandau-certs/client.crt"
    key_path: "$original_home/mandau-certs/client.key"
    ca_path: "$original_home/mandau-certs/ca.crt"
    min_version: "TLS1.3"
    server_name: "mandau-core"
timeout: "10s"
# Local development settings
EOF

    # Set appropriate permissions for the config directory and files
    chown -R "$original_user:$original_user" "$CONFIG_DIR"
    chmod 700 "$CONFIG_DIR"
    chmod 600 "$CONFIG_DIR/config.yaml"
    chmod 600 "$CONFIG_DIR/dev.yaml"
    chmod 600 "$CONFIG_DIR/local.yaml"

    print_success "Default configuration created at $CONFIG_DIR/"
    print_dev "Development profiles created: dev.yaml, local.yaml"
    print_status "Use MANDAU_PROFILE=dev to use development configuration"
    print_status "Certificates are located in ~/mandau-certs/ (first-class location)"
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

    # Determine the original user (in case running with sudo)
    if [ -n "$SUDO_USER" ]; then
        ORIGINAL_USER="$SUDO_USER"
    else
        ORIGINAL_USER="$(whoami)"
    fi

    # Get the home directory for the original user
    ORIGINAL_HOME=$(eval echo ~$ORIGINAL_USER)
    
    # Generate certificates in the first-class location
    generate_certificates "$ORIGINAL_HOME/mandau-certs" "$ORIGINAL_USER"

    # Create default configuration with profile support
    create_default_config "$ORIGINAL_USER" "$ORIGINAL_HOME"

    # Create systemd service files with absolute paths
    create_systemd_services "$ORIGINAL_USER" "$ORIGINAL_HOME"

    # Create stacks directory for agent
    mkdir -p "$ORIGINAL_HOME/mandau-stacks"
    chown "$ORIGINAL_USER:$ORIGINAL_USER" "$ORIGINAL_HOME/mandau-stacks"
    chmod 755 "$ORIGINAL_HOME/mandau-stacks"

    print_success "Mandau installation completed successfully!"
    print_status "Certificates are in ~/mandau-certs/ (first-class location)"
    print_status "Configuration is in ~/.mandau/ with profile support"
    print_status "Stacks directory created at ~/mandau-stacks/"
    print_status "Systemd services created with absolute paths"

    # Return to original directory and cleanup
    cd - >/dev/null
    rm -rf "$TEMP_DIR"
}

# Main execution
main() {
    print_status "Starting Mandau installation with enhanced configuration..."

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
    print_status "For development: Use MANDAU_PROFILE=dev to use development configuration"
    print_status "For systemd services: Run 'sudo systemctl enable mandau-core mandau-agent' and 'sudo systemctl start mandau-core'"
}

# Run main function
main "$@"