#!/bin/bash

# Mandau Development Setup Script
# This script helps set up development configurations and profiles

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

# Setup development environment
setup_dev_environment() {
    print_dev "Setting up development environment..."
    
    # Get the original user and home directory
    if [ -n "$SUDO_USER" ]; then
        ORIGINAL_USER="$SUDO_USER"
    else
        ORIGINAL_USER="$(whoami)"
    fi
    ORIGINAL_HOME=$(eval echo ~$ORIGINAL_USER)
    
    # Create development-specific directories
    mkdir -p "$ORIGINAL_HOME/mandau-stacks-dev"
    mkdir -p "$ORIGINAL_HOME/mandau-stacks-test"
    
    # Set proper permissions
    chown -R "$ORIGINAL_USER:$ORIGINAL_USER" "$ORIGINAL_HOME/mandau-stacks-dev"
    chown -R "$ORIGINAL_USER:$ORIGINAL_USER" "$ORIGINAL_HOME/mandau-stacks-test"
    
    print_dev "Created development directories:"
    print_dev "  - ~/mandau-stacks-dev"
    print_dev "  - ~/mandau-stacks-test"
    
    # Create development configuration profiles
    CONFIG_DIR="$ORIGINAL_HOME/.mandau"
    
    # Create or update dev profile with development-specific settings
    cat > "$CONFIG_DIR/dev.yaml" << EOF
# Development Profile Configuration
server:
  listen_addr: "localhost:8443"
  tls:
    cert_path: "$ORIGINAL_HOME/mandau-certs/client.crt"
    key_path: "$ORIGINAL_HOME/mandau-certs/client.key"
    ca_path: "$ORIGINAL_HOME/mandau-certs/ca.crt"
    min_version: "TLS1.3"
    server_name: "mandau-core"
timeout: "10s"  # Shorter timeout for development
debug: true     # Enable debug mode
# Development-specific settings
EOF

    # Create test profile
    cat > "$CONFIG_DIR/test.yaml" << EOF
# Test Profile Configuration
server:
  listen_addr: "localhost:8444"  # Different port for testing
  tls:
    cert_path: "$ORIGINAL_HOME/mandau-certs/client.crt"
    key_path: "$ORIGINAL_HOME/mandau-certs/client.key"
    ca_path: "$ORIGINAL_HOME/mandau-certs/ca.crt"
    min_version: "TLS1.3"
    server_name: "mandau-core"
timeout: "5s"
debug: true
# Test-specific settings
EOF

    # Set permissions
    chmod 600 "$CONFIG_DIR/dev.yaml"
    chmod 600 "$CONFIG_DIR/test.yaml"
    
    print_dev "Updated development profiles with development-specific settings"
}

# Create a script for easy service management
create_service_scripts() {
    print_dev "Creating service management scripts..."
    
    if [ -n "$SUDO_USER" ]; then
        ORIGINAL_USER="$SUDO_USER"
    else
        ORIGINAL_USER="$(whoami)"
    fi
    ORIGINAL_HOME=$(eval echo ~$ORIGINAL_USER)
    
    # Create a script to start development services
    cat > "$ORIGINAL_HOME/mandau-dev-start.sh" << EOF
#!/bin/bash
# Development start script for Mandau services

echo "Starting Mandau development services..."

# Start Core service
echo "Starting Mandau Core..."
/usr/local/bin/mandau-core \\
  --listen :8443 \\
  --cert $ORIGINAL_HOME/mandau-certs/core.crt \\
  --key $ORIGINAL_HOME/mandau-certs/core.key \\
  --ca $ORIGINAL_HOME/mandau-certs/ca.crt &

CORE_PID=\$!
echo "Core started with PID: \$CORE_PID"

# Wait a moment for core to start
sleep 2

# Start Agent service
echo "Starting Mandau Agent..."
/usr/local/bin/mandau-agent \\
  --server localhost:8443 \\
  --cert $ORIGINAL_HOME/mandau-certs/agent.crt \\
  --key $ORIGINAL_HOME/mandau-certs/agent.key \\
  --ca $ORIGINAL_HOME/mandau-certs/ca.crt \\
  --stack-root $ORIGINAL_HOME/mandau-stacks &

AGENT_PID=\$!
echo "Agent started with PID: \$AGENT_PID"

echo "Mandau services started!"
echo "Core PID: \$CORE_PID"
echo "Agent PID: \$AGENT_PID"

# Save PIDs for later use
echo \$CORE_PID > /tmp/mandau-core.pid
echo \$AGENT_PID > /tmp/mandau-agent.pid

echo "To stop services, run: pkill -f 'mandau-(core|agent)'"
EOF

    chmod +x "$ORIGINAL_HOME/mandau-dev-start.sh"
    
    print_dev "Created development start script: ~/mandau-dev-start.sh"
}

# Create profile management functions
create_profile_management() {
    print_dev "Creating profile management utilities..."
    
    if [ -n "$SUDO_USER" ]; then
        ORIGINAL_USER="$SUDO_USER"
    else
        ORIGINAL_USER="$(whoami)"
    fi
    ORIGINAL_HOME=$(eval echo ~$ORIGINAL_USER)
    
    # Create a profile switcher script
    cat > "$ORIGINAL_HOME/mandau-profile.sh" << EOF
#!/bin/bash
# Mandau Profile Management Script

CONFIG_DIR="$ORIGINAL_HOME/.mandau"

case "\$1" in
    "list"|"ls")
        echo "Available Mandau profiles:"
        for profile in \$(ls \$CONFIG_DIR/*.yaml 2>/dev/null | xargs -n 1 basename | sed 's/\.yaml$//'); do
            if [ "\$profile" = "\$(basename \$(readlink -f \$CONFIG_DIR/config.yaml) .yaml 2>/dev/null)" ]; then
                echo "  * \$profile (active)"
            else
                echo "  - \$profile"
            fi
        done
        ;;
    "use"|"switch")
        if [ -z "\$2" ]; then
            echo "Usage: \$0 use <profile>"
            echo "Available profiles: \$(ls \$CONFIG_DIR/*.yaml 2>/dev/null | xargs -n 1 basename | sed 's/\.yaml$//' | tr '\n' ' ')"
            exit 1
        fi
        
        PROFILE_FILE="\$CONFIG_DIR/\$2.yaml"
        if [ ! -f "\$PROFILE_FILE" ]; then
            echo "Profile '\$2' not found!"
            echo "Available profiles: \$(ls \$CONFIG_DIR/*.yaml 2>/dev/null | xargs -n 1 basename | sed 's/\.yaml$//' | tr '\n' ' ')"
            exit 1
        fi
        
        # Create a symlink to the selected profile
        ln -sf "\$PROFILE_FILE" "\$CONFIG_DIR/config.yaml"
        echo "Switched to profile: \$2"
        ;;
    "show"|"current")
        CURRENT_CONFIG="\$(readlink -f \$CONFIG_DIR/config.yaml 2>/dev/null)"
        if [ -n "\$CURRENT_CONFIG" ]; then
            PROFILE_NAME="\$(basename "\$CURRENT_CONFIG" .yaml)"
            echo "Current profile: \$PROFILE_NAME"
        else
            echo "Current profile: default"
        fi
        ;;
    *)
        echo "Usage: \$0 {list|use|show}"
        echo "  list/ls  - List available profiles"
        echo "  use      - Switch to a profile: \$0 use <profile>"
        echo "  show     - Show current profile"
        exit 1
        ;;
esac
EOF

    chmod +x "$ORIGINAL_HOME/mandau-profile.sh"
    
    print_dev "Created profile management script: ~/mandau-profile.sh"
    print_dev "Usage examples:"
    print_dev "  ~/mandau-profile.sh list          # List available profiles"
    print_dev "  ~/mandau-profile.sh use dev       # Switch to dev profile"
    print_dev "  ~/mandau-profile.sh show          # Show current profile"
}

# Main execution
main() {
    print_dev "Setting up Mandau development environment..."
    
    setup_dev_environment
    create_service_scripts
    create_profile_management
    
    print_success "Development environment setup complete!"
    print_dev "Next steps:"
    print_dev "  1. Use ~/mandau-profile.sh to manage configuration profiles"
    print_dev "  2. Use ~/mandau-dev-start.sh to start development services"
    print_dev "  3. Set MANDAU_PROFILE=dev environment variable for development"
    print_dev "  4. Use ~/mandau-stacks-dev for development stack files"
}

main "$@"