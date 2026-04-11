#!/bin/bash
# =============================================================================
# Mandau Installation Script
# =============================================================================
# Automates platform detection, binary installation, certificate generation,
# and systemd service configuration for Mandau Core and Agent.
# =============================================================================

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

# Print functions
print_info() {
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

print_step() {
    echo ""
    echo -e "${PURPLE}[STEP]${NC} $1"
    echo "─────────────────────────────────────────────────────"
}

# Detection Variables
OS=""
ARCH=""
LATEST_VERSION=""
ORIGINAL_USER=""
ORIGINAL_HOME=""

# Detect platform
detect_platform() {
    print_step "Detecting Platform"
    
    case "$(uname -s)" in
        Linux*)  OS="linux" ;;
        Darwin*) OS="darwin" ;;
        *)       print_error "Unsupported OS: $(uname -s)"; exit 1 ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *)             print_error "Unsupported architecture: $(uname -m)"; exit 1 ;;
    esac

    print_success "Detected: ${OS}/${ARCH}"
}

# Pre-flight checks
preflight_checks() {
    print_step "Pre-flight Checks"

    # Check for required tools
    local tools=("curl" "openssl")
    for tool in "${tools[@]}"; do
        if ! command -v "$tool" &> /dev/null; then
            print_error "'$tool' is required but not installed."
            exit 1
        fi
    done

    # Check for systemd on Linux
    if [ "$OS" == "linux" ]; then
        if ! pidof systemd &>/dev/null && [ ! -d /run/systemd/system ]; then
            print_warning "systemd not detected. Services will not be configured."
            HAS_SYSTEMD=false
        else
            HAS_SYSTEMD=true
        fi
        
        if ! command -v docker &> /dev/null; then
            print_warning "Docker not found. Mandau Core/Agent may require Docker to function properly."
        fi
    else
        HAS_SYSTEMD=false
    fi

    # Determine original user (for sudo context)
    if [ "${SUDO_USER:-}" != "" ]; then
        ORIGINAL_USER="$SUDO_USER"
    else
        ORIGINAL_USER="$(whoami)"
    fi
    ORIGINAL_HOME=$(eval echo "~$ORIGINAL_USER")

    print_success "Pre-flight checks passed (User: $ORIGINAL_USER, Home: $ORIGINAL_HOME)"
}

# Fetch latest version
get_latest_version() {
    print_step "Fetching Version Info"
    
    if [ "${MANDAU_VERSION:-}" != "" ]; then
        LATEST_VERSION="$MANDAU_VERSION"
        print_info "Using manually specified version: $LATEST_VERSION"
        return 0
    fi

    LATEST_VERSION=$(curl -sL "https://api.github.com/repos/bhangun/mandau/releases/latest" \
        -H "Accept: application/vnd.github.v3+json" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

    if [ -z "$LATEST_VERSION" ]; then
        print_error "Failed to fetch latest version from GitHub."
        exit 1
    fi

    print_success "Latest version: $LATEST_VERSION"
}

# Download and Extract
download_binaries() {
    print_step "Downloading Binaries"
    
    local version_no_v="${LATEST_VERSION#v}"
    local filename="mandau-${OS}-${ARCH}-${version_no_v}.tar.gz"
    local url="https://github.com/bhangun/mandau/releases/download/${LATEST_VERSION}/${filename}"
    
    # Special case for Windows would go here if needed, keeping it Unix-centric for now
    
    TEMP_DIR=$(mktemp -d)
    print_info "Downloading $url..."
    
    if ! curl -fsSL -o "$TEMP_DIR/$filename" "$url"; then
        print_error "Download failed."
        rm -rf "$TEMP_DIR"
        exit 1
    fi

    print_info "Extracting..."
    tar -xzf "$TEMP_DIR/$filename" -C "$TEMP_DIR"
    
    # Ensure binaries are executable
    chmod +x "$TEMP_DIR"/mandau* 2>/dev/null || true
    
    # Return temp dir path
    echo "$TEMP_DIR"
}

# Install Binaries
install_binaries() {
    local source_dir="$1"
    print_step "Installing Binaries"

    local install_dir="/usr/local/bin"
    local binaries=("mandau" "mandau-core" "mandau-agent")
    
    SUDO_CMD=""
    if [ "$EUID" -ne 0 ] && command -v sudo &>/dev/null; then
        SUDO_CMD="sudo"
    fi

    for bin in "${binaries[@]}"; do
        if [ -f "$source_dir/$bin" ]; then
            print_info "Installing $bin to $install_dir..."
            $SUDO_CMD install -m 755 "$source_dir/$bin" "$install_dir/$bin"
        else
            print_warning "Binary $bin not found in package, skipping."
        fi
    done

    print_success "Binaries installed to $install_dir"
}

# Setup Directories and Certificates
setup_environment() {
    print_step "Setting up Environment"
    
    local config_dir="$ORIGINAL_HOME/.mandau"
    local cert_dir="$config_dir/certs"
    local stacks_dir="$ORIGINAL_HOME/mandau-stacks"

    # Create directories with proper ownership
    mkdir -p "$config_dir" "$cert_dir" "$stacks_dir"
    chown -R "$ORIGINAL_USER:$ORIGINAL_USER" "$config_dir" "$stacks_dir"
    chmod 700 "$config_dir" "$cert_dir"

    # Generate certificates if they don't exist
    if [ ! -f "$cert_dir/ca.crt" ]; then
        print_info "Generating self-signed certificates in $cert_dir..."
        
        # CA
        openssl genrsa -out "$cert_dir/ca.key" 4096
        openssl req -new -x509 -days 3650 -key "$cert_dir/ca.key" -out "$cert_dir/ca.crt" \
            -subj "/CN=Mandau Root CA/O=Mandau/C=ID" -nodes

        # Helper for cert generation
        generate_cert() {
            local name=$1
            local cn=$2
            local ext_content=$3
            
            openssl genrsa -out "$cert_dir/$name.key" 4096
            openssl req -new -key "$cert_dir/$name.key" -out "$cert_dir/$name.csr" \
                -subj "/CN=$cn/O=Mandau/C=ID" -nodes
            
            echo "$ext_content" > "$cert_dir/$name.ext"
            
            openssl x509 -req -in "$cert_dir/$name.csr" -CA "$cert_dir/ca.crt" -CAkey "$cert_dir/ca.key" \
                -CAcreateserial -out "$cert_dir/$name.crt" -days 365 -extfile "$cert_dir/$name.ext"
            
            rm "$cert_dir/$name.csr" "$cert_dir/$name.ext"
        }

        generate_cert "core" "mandau-core" "subjectAltName=DNS:localhost,IP:127.0.0.1"
        generate_cert "agent" "mandau-agent" "subjectAltName=DNS:localhost,IP:127.0.0.1"
        generate_cert "client" "mandau-cli" "extendedKeyUsage=clientAuth"

        # Final permissions
        chmod 600 "$cert_dir"/*.key
        chmod 644 "$cert_dir"/*.crt
        chown -R "$ORIGINAL_USER:$ORIGINAL_USER" "$cert_dir"
        print_success "Certificates generated successfully."
    else
        print_info "Certificates already exist in $cert_dir, skipping generation."
    fi

    # Create default config
    local core_port="${MANDAU_CORE_PORT:-3443}"
    if [ ! -f "$config_dir/config.yaml" ]; then
        cat > "$config_dir/config.yaml" <<EOF
server:
  listen_addr: "localhost:${core_port}"
  tls:
    cert_path: "$cert_dir/client.crt"
    key_path: "$cert_dir/client.key"
    ca_path: "$cert_dir/ca.crt"
    server_name: "mandau-core"
EOF
        chown "$ORIGINAL_USER:$ORIGINAL_USER" "$config_dir/config.yaml"
        chmod 600 "$config_dir/config.yaml"
        print_success "Default configuration created."
    fi
}

# Create Systemd Services
configure_services() {
    if [ "$HAS_SYSTEMD" != "true" ]; then return 0; fi
    
    print_step "Configuring Systemd Services"
    local core_port="${MANDAU_CORE_PORT:-3443}"
    local SUDO_CMD=""
    if [ "$EUID" -ne 0 ]; then SUDO_CMD="sudo"; fi

    # Mandau Core Service
    cat <<EOF | $SUDO_CMD tee /etc/systemd/system/mandau-core.service >/dev/null
[Unit]
Description=Mandau Core Service
After=network.target docker.service
Wants=docker.service

[Service]
Type=simple
User=$ORIGINAL_USER
Group=$ORIGINAL_USER
WorkingDirectory=$ORIGINAL_HOME
ExecStart=/usr/local/bin/mandau-core \\
    -listen :${core_port} \\
    -cert $ORIGINAL_HOME/.mandau/certs/core.crt \\
    -key $ORIGINAL_HOME/.mandau/certs/core.key \\
    -ca $ORIGINAL_HOME/.mandau/certs/ca.crt
Restart=always
RestartSec=5

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=$ORIGINAL_HOME/.mandau $ORIGINAL_HOME/mandau-stacks /tmp

[Install]
WantedBy=multi-user.target
EOF

    # Mandau Agent Service
    cat <<EOF | $SUDO_CMD tee /etc/systemd/system/mandau-agent.service >/dev/null
[Unit]
Description=Mandau Agent Service
After=network.target mandau-core.service docker.service
Wants=mandau-core.service docker.service

[Service]
Type=simple
User=$ORIGINAL_USER
Group=$ORIGINAL_USER
WorkingDirectory=$ORIGINAL_HOME
ExecStart=/usr/local/bin/mandau-agent \\
    -server localhost:${core_port} \\
    -cert $ORIGINAL_HOME/.mandau/certs/agent.crt \\
    -key $ORIGINAL_HOME/.mandau/certs/agent.key \\
    -ca $ORIGINAL_HOME/.mandau/certs/ca.crt \\
    -stack-root $ORIGINAL_HOME/mandau-stacks
Restart=always
RestartSec=5

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=$ORIGINAL_HOME/.mandau $ORIGINAL_HOME/mandau-stacks /var/run/docker.sock /tmp

[Install]
WantedBy=multi-user.target
EOF

    # Reload and enable
    $SUDO_CMD chmod 644 /etc/systemd/system/mandau-*.service
    $SUDO_CMD systemctl daemon-reload
    print_success "Systemd services configured."
    
    print_info "Starting services..."
    $SUDO_CMD systemctl enable mandau-core mandau-agent 2>/dev/null || true
    $SUDO_CMD systemctl restart mandau-core
    $SUDO_CMD systemctl restart mandau-agent
}

# Main function
main() {
    local client_only=false
    while [[ $# -gt 0 ]]; do
        case $1 in
            --client) client_only=true; shift ;;
            *) shift ;;
        esac
    done

    echo -e "${BLUE}╔═══════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║${NC}  ${GREEN}Mandau Installation Script${NC}                          ${BLUE}║${NC}"
    echo -e "${BLUE}╚═══════════════════════════════════════════════════════╝${NC}"

    detect_platform
    preflight_checks
    
    if [ "$OS" == "linux" ]; then
        if ! command -v docker &> /dev/null; then
            print_warning "Docker is NOT installed. Mandau Agent requires Docker to manage containers."
            print_info "Install Docker: https://docs.docker.com/engine/install/"
        else
            print_success "Docker detected ($(docker --version))"
        fi
    fi

    get_latest_version
    
    TEMP_DIR=$(download_binaries)
    
    if [ "$client_only" == "true" ]; then
        print_info "Client-only mode requested. Installing only the Mandau CLI."
        local install_dir="/usr/local/bin"
        local SUDO_CMD=""
        if [ "$EUID" -ne 0 ] && command -v sudo &>/dev/null; then SUDO_CMD="sudo"; fi
        $SUDO_CMD install -m 755 "$TEMP_DIR/mandau" "$install_dir/mandau"
        print_success "Mandau CLI installed."
    else
        install_binaries "$TEMP_DIR"
        setup_environment
        configure_services
        
        # Service Health Check
        print_step "Verifying Services"
        if [ "$HAS_SYSTEMD" == "true" ]; then
            sleep 2
            if systemctl is-active --quiet mandau-core; then
                print_success "Mandau Core is running."
            else
                print_error "Mandau Core failed to start. Check: journalctl -u mandau-core"
            fi
            
            if systemctl is-active --quiet mandau-agent; then
                print_success "Mandau Agent is running."
            else
                print_error "Mandau Agent failed to start. Check: journalctl -u mandau-agent"
            fi
        fi
    fi
    
    rm -rf "$TEMP_DIR"

    print_step "Installation Summary"
    if [ "$client_only" == "true" ]; then
        print_success "Mandau CLI $LATEST_VERSION has been installed!"
        echo -e "  - ${BLUE}CLI:${NC}         /usr/local/bin/mandau"
        echo ""
        print_info "Next steps for client setup:"
        print_info "1. Run 'mandau connect <server-ip>'"
        print_info "2. Sync certificates as instructed by the connect command."
    else
        print_success "Mandau $LATEST_VERSION (Server & Agent) has been installed!"
        echo ""
        echo -e "  - ${BLUE}CLI:${NC}         /usr/local/bin/mandau"
        echo -e "  - ${BLUE}Config:${NC}      $ORIGINAL_HOME/.mandau/config.yaml"
        echo -e "  - ${BLUE}Stacks:${NC}      $ORIGINAL_HOME/mandau-stacks/"
        echo ""
        if [ "$HAS_SYSTEMD" == "true" ]; then
            print_info "To check status: sudo systemctl status mandau-core mandau-agent"
            print_info "To view logs:   journalctl -u mandau-agent -f"
        fi
    fi
    echo ""
    print_info "Run 'mandau --help' to get started."
    print_success "Done!"
}

main "$@"
