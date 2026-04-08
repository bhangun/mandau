#!/bin/bash

# Mandau Development Runner Script
# Automatically builds, installs, and runs Mandau in development mode
# With enhanced reliability features: cleanup, monitoring, and recovery

set -euo pipefail

# Function to print colored output
print_info() {
    echo -e "\033[1;34m[i]\033[0m $1"  # Blue
}

print_success() {
    echo -e "\033[1;32m[v]\033[0m $1"  # Green
}

print_warning() {
    echo -e "\033[1;33m[!]\033[0m $1"  # Yellow
}

print_error() {
    echo -e "\033[1;31m[x]\033[0m $1"  # Red
}

# Configuration
CORE_PORT=${MANDAU_CORE_PORT:-9443}
AGENT_PORT=${MANDAU_AGENT_PORT:-9444}
CORE_PID_FILE="/tmp/mandau-core.pid"
AGENT_PID_FILE="/tmp/mandau-agent.pid"
HEALTH_CHECK_INTERVAL=30  # seconds
MAX_RESTART_ATTEMPTS=5
RESTART_DELAY=5  # seconds

# Admin credentials (can be overridden via environment variables)
MANDAU_ADMIN_USER=${MANDAU_ADMIN_USER:-admin}
MANDAU_ADMIN_PASS=${MANDAU_ADMIN_PASS:-}

# Binary paths (will be set during build)
MANDAU_CORE_BIN=""
MANDAU_AGENT_BIN=""
MANDAU_CLI_BIN=""

# Function to build Mandau binaries
build_mandau() {
    print_info "Building Mandau binaries..."

    # Navigate to project root
    cd "$(dirname "$0")"

    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed or not in PATH"
        print_info "Install Go from https://golang.org/dl/"
        exit 1
    fi

    # Create bin directory
    mkdir -p bin

    # Build mandau-core
    print_info "  Building mandau-core..."
    if ! go build -o bin/mandau-core ./cmd/mandau-core 2>&1; then
        print_error "Failed to build mandau-core"
        exit 1
    fi

    # Build mandau-agent
    print_info "  Building mandau-agent..."
    if ! go build -o bin/mandau-agent ./cmd/mandau-agent 2>&1; then
        print_error "Failed to build mandau-agent"
        exit 1
    fi

    # Build mandau CLI
    print_info "  Building mandau CLI..."
    if ! go build -o bin/mandau ./cmd/mandau-cli 2>&1; then
        print_error "Failed to build mandau CLI"
        exit 1
    fi

    # Make binaries executable
    chmod +x bin/mandau-core bin/mandau-agent bin/mandau

    # Set binary paths
    MANDAU_CORE_BIN="$(pwd)/bin/mandau-core"
    MANDAU_AGENT_BIN="$(pwd)/bin/mandau-agent"
    MANDAU_CLI_BIN="$(pwd)/bin/mandau"

    print_success "Mandau binaries built successfully"
}

# Function to install Mandau binaries to local environment
install_mandau() {
    print_info "Installing Mandau binaries to local environment..."

    # Install to ~/.local/bin (user-level) or /usr/local/bin (system-level)
    local install_dir=""
    if [[ -d "$HOME/.local/bin" ]] && echo "$PATH" | grep -q "$HOME/.local/bin"; then
        install_dir="$HOME/.local/bin"
        print_info "  Installing to user directory: $install_dir"
    elif command -v sudo &> /dev/null && [[ -w "/usr/local/bin" || $EUID -eq 0 ]]; then
        install_dir="/usr/local/bin"
        print_info "  Installing to system directory: $install_dir"
    else
        install_dir="$HOME/.local/bin"
        mkdir -p "$install_dir"
        print_info "  Installing to user directory: $install_dir"
    fi

    # Copy binaries
    cp bin/mandau-core "$install_dir/"
    cp bin/mandau-agent "$install_dir/"
    cp bin/mandau "$install_dir/"

    # Make executable
    chmod +x "$install_dir/mandau-core"
    chmod +x "$install_dir/mandau-agent"
    chmod +x "$install_dir/mandau"

    print_success "Mandau installed to: $install_dir"
    print_info "  Ensure '$install_dir' is in your PATH"
}

# Function to generate certificates if needed
ensure_certificates() {
    local cert_dir="$HOME/.mandau/certs"
    local ca_dir="$HOME/.mandau/ca"

    # Check if certificates already exist
    if [[ -f "$cert_dir/ca.crt" ]] && [[ -f "$cert_dir/core.crt" ]] && [[ -f "$cert_dir/agent.crt" ]]; then
        print_success "Certificates found in $cert_dir"
        return 0
    fi

    print_info "Generating certificates..."

    # Try using mandau CLI first
    if [[ -n "$MANDAU_CLI_BIN" ]] && [[ -x "$MANDAU_CLI_BIN" ]]; then
        print_info "  Using mandau cert gen..."
        if "$MANDAU_CLI_BIN" cert gen; then
            print_success "Certificates generated via mandau CLI"
            return 0
        fi
    fi

    # Fallback to make certs
    print_info "  Using make certs..."
    if command -v make &> /dev/null; then
        make certs 2>/dev/null || {
            print_warning "make certs failed, trying script..."
            # Try generate-certs.sh
            if [[ -x "./scripts/generate-certs.sh" ]]; then
                ./scripts/generate-certs.sh "$cert_dir" || {
                    print_error "Failed to generate certificates"
                    exit 1
                }
            else
                print_error "No certificate generation method available"
                exit 1
            fi
        }
        
        # Copy generated certs to ~/.mandau/certs if they were generated elsewhere
        if [[ -d "./certs" ]] && [[ -f "./certs/ca.crt" ]]; then
            mkdir -p "$cert_dir" "$ca_dir"
            cp ./certs/* "$cert_dir/" 2>/dev/null || true
            cp ./certs/ca.crt ./certs/ca.key "$ca_dir/" 2>/dev/null || true
        fi
    fi

    print_success "Certificates generated"
}

# Function to check if port is available
check_port() {
    local port=$1
    if command -v lsof >/dev/null 2>&1; then
        if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; then
            return 1  # Port is in use
        else
            return 0  # Port is free
        fi
    else
        # Fallback for systems without lsof
        if nc -z localhost $port 2>/dev/null; then
            return 1  # Port is in use
        else
            return 0  # Port is free
        fi
    fi
}

# Function to check if executable exists
check_executable() {
    local exe=$1
    if [[ ! -x "$exe" ]]; then
        print_error "Executable '$exe' does not exist or is not executable."
        exit 1
    fi
}

# Function to cleanup processes and files on exit
cleanup() {
    print_info "Cleaning up Mandau processes..."

    # Kill core process if running
    if [[ -f "$CORE_PID_FILE" ]] && [[ -n "$(cat $CORE_PID_FILE 2>/dev/null || true)" ]]; then
        CORE_PID=$(cat $CORE_PID_FILE)
        if kill -0 $CORE_PID 2>/dev/null; then
            print_info "Stopping Mandau Core (PID: $CORE_PID)..."
            kill $CORE_PID 2>/dev/null
            # Wait for graceful shutdown, then force kill if needed
            sleep 3
            if kill -0 $CORE_PID 2>/dev/null; then
                kill -9 $CORE_PID 2>/dev/null || true
            fi
        fi
        rm -f $CORE_PID_FILE
    fi

    # Kill agent process if running
    if [[ -f "$AGENT_PID_FILE" ]] && [[ -n "$(cat $AGENT_PID_FILE 2>/dev/null || true)" ]]; then
        AGENT_PID=$(cat $AGENT_PID_FILE)
        if kill -0 $AGENT_PID 2>/dev/null; then
            print_info "Stopping Mandau Agent (PID: $AGENT_PID)..."
            kill $AGENT_PID 2>/dev/null
            # Wait for graceful shutdown, then force kill if needed
            sleep 3
            if kill -0 $AGENT_PID 2>/dev/null; then
                kill -9 $AGENT_PID 2>/dev/null || true
            fi
        fi
        rm -f $AGENT_PID_FILE
    fi

    # Kill any remaining mandau processes
    pkill -f "mandau-core" 2>/dev/null || true
    pkill -f "mandau-agent" 2>/dev/null || true

    print_success "Cleanup completed"
}

# Function to kill stale processes
cleanup_stale_processes() {
    print_info "Cleaning up stale Mandau processes..."

    # Kill any existing mandau processes
    pkill -f "mandau-core" 2>/dev/null || true
    pkill -f "mandau-agent" 2>/dev/null || true

    # Wait a moment for processes to terminate
    sleep 2

    # Remove PID files
    rm -f $CORE_PID_FILE $AGENT_PID_FILE

    print_success "Stale processes cleaned up"
}

# Function to start core server
start_core() {
    local attempt=1
    local max_attempts=3

    while [ $attempt -le $max_attempts ]; do
        print_info "Starting Mandau Core (attempt $attempt/$max_attempts)..."

        # Check if port is available
        if ! check_port $CORE_PORT; then
            print_error "Port $CORE_PORT is still in use. Attempting to kill any remaining processes..."
            cleanup_stale_processes
            sleep 2
        fi

        # Start core server in background
        MANDAU_ADMIN_USER="$MANDAU_ADMIN_USER" \
        MANDAU_ADMIN_PASS="$MANDAU_ADMIN_PASS" \
        "$MANDAU_CORE_BIN" \
            --listen ":$CORE_PORT" \
            --cert "$HOME/.mandau/certs/core.crt" \
            --key "$HOME/.mandau/certs/core.key" \
            --ca "$HOME/.mandau/certs/ca.crt" &

        CORE_PID=$!

        # Save PID to file
        echo $CORE_PID > $CORE_PID_FILE

        # Wait a moment for core to start
        sleep 3

        # Check if core is running
        if kill -0 $CORE_PID 2>/dev/null; then
            print_success "Mandau Core started successfully (PID: $CORE_PID) on port $CORE_PORT"
            return 0
        else
            print_error "Mandau Core failed to start on attempt $attempt"
            attempt=$((attempt + 1))
            if [ $attempt -le $max_attempts ]; then
                sleep 5
            fi
        fi
    done

    print_error "Mandau Core failed to start after $max_attempts attempts"
    return 1
}

# Function to start agent
start_agent() {
    local attempt=1
    local max_attempts=3

    while [ $attempt -le $max_attempts ]; do
        print_info "Starting Mandau Agent (attempt $attempt/$max_attempts)..."

        # Check if agent port is available
        if ! check_port $AGENT_PORT; then
            print_warning "Agent port $AGENT_PORT is in use, trying to start anyway..."
        fi

        # Start agent in background
        "$MANDAU_AGENT_BIN" \
            --server "localhost:$CORE_PORT" \
            --cert "$HOME/.mandau/certs/agent.crt" \
            --key "$HOME/.mandau/certs/agent.key" \
            --ca "$HOME/.mandau/certs/ca.crt" \
            --stack-root ./stacks &

        AGENT_PID=$!

        # Save PID to file
        echo $AGENT_PID > $AGENT_PID_FILE

        # Wait a moment for agent to start and register
        sleep 5

        # Check if agent is running
        if kill -0 $AGENT_PID 2>/dev/null; then
            print_success "Mandau Agent started successfully (PID: $AGENT_PID)"
            return 0
        else
            print_error "Mandau Agent failed to start on attempt $attempt"
            attempt=$((attempt + 1))
            if [ $attempt -le $max_attempts ]; then
                sleep 5
            fi
        fi
    done

    print_error "Mandau Agent failed to start after $max_attempts attempts"
    return 1
}

# Function to monitor processes and restart if needed
monitor_processes() {
    print_info "Starting process monitoring..."

    while true; do
        # Check if core is running
        if [[ -f "$CORE_PID_FILE" ]]; then
            CORE_PID=$(cat $CORE_PID_FILE)
            if ! kill -0 $CORE_PID 2>/dev/null; then
                print_warning "Mandau Core process died (PID: $CORE_PID). Attempting restart..."
                rm -f $CORE_PID_FILE
                if start_core; then
                    print_success "Mandau Core restarted successfully"
                else
                    print_error "Failed to restart Mandau Core"
                    exit 1
                fi
            fi
        fi

        # Check if agent is running
        if [[ -f "$AGENT_PID_FILE" ]]; then
            AGENT_PID=$(cat $AGENT_PID_FILE)
            if ! kill -0 $AGENT_PID 2>/dev/null; then
                print_warning "Mandau Agent process died (PID: $AGENT_PID). Attempting restart..."
                rm -f $AGENT_PID_FILE
                if start_agent; then
                    print_success "Mandau Agent restarted successfully"
                else
                    print_error "Failed to restart Mandau Agent"
                    exit 1
                fi
            fi
        fi

        # Health check - verify both processes are communicating
        if [[ -f "$CORE_PID_FILE" ]] && [[ -f "$AGENT_PID_FILE" ]]; then
            CORE_PID=$(cat $CORE_PID_FILE)
            AGENT_PID=$(cat $AGENT_PID_FILE)

            if kill -0 $CORE_PID 2>/dev/null && kill -0 $AGENT_PID 2>/dev/null; then
                print_info "Health check: Both processes running"
            fi
        fi

        # Wait before next check
        sleep $HEALTH_CHECK_INTERVAL
    done
}

# Function to show agent status
show_agent_status() {
    print_info "Current agent status:"

    # Try using mandau CLI with auto-discovered certs
    if [[ -n "$MANDAU_CLI_BIN" ]] && [[ -x "$MANDAU_CLI_BIN" ]]; then
        timeout 10s "$MANDAU_CLI_BIN" agent list || {
            print_warning "Unable to retrieve agent list (core may still be starting)"
        }
    elif [[ -f "$HOME/.mandau/certs/client.crt" ]] && [[ -f "$HOME/.mandau/certs/client.key" ]]; then
        # Fallback to explicit cert paths
        timeout 10s ./bin/mandau agent list || {
            print_warning "Unable to retrieve agent list (core may still be starting)"
        }
    else
        print_warning "Client certificates not found. Cannot show agent status."
    fi
}

# Function to show usage
show_usage() {
    echo "Usage: $0 [--host | --host-with-port CORE_PORT AGENT_PORT | --clean | --no-build | --help]"
    echo "  (no args)           Build, install, and run Mandau (default, ports 9443/9444)"
    echo "  --host              Build, install, and run Mandau (default port 9443/9444)"
    echo "  --host-with-port    Run Mandau with custom ports for core and agent"
    echo "  --clean             Clean up any stale processes and exit"
    echo "  --no-build          Skip build step (use existing binaries)"
    echo "  --help              Show this help message"
}

# Main execution - build and setup
build_and_setup() {
    local skip_build=${1:-false}

    # Navigate to the mandau directory
    cd "$(dirname "$0")"

    if [[ "$skip_build" == true ]]; then
        print_info "Skipping build (using existing binaries)..."
        MANDAU_CORE_BIN="$(pwd)/bin/mandau-core"
        MANDAU_AGENT_BIN="$(pwd)/bin/mandau-agent"
        MANDAU_CLI_BIN="$(pwd)/bin/mandau"

        # Check if binaries exist
        check_executable "$MANDAU_CORE_BIN"
        check_executable "$MANDAU_AGENT_BIN"
    else
        # Build Mandau
        build_mandau

        # Install to local environment
        install_mandau

        # Ensure certificates exist
        ensure_certificates
    fi

    # Ensure stacks directory exists
    mkdir -p stacks
}

# Parse arguments
if [ "$#" -eq 0 ]; then
    # Default to host mode if no arguments provided
    set -- "--host"
fi

if [ "$#" -eq 0 ] || [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    show_usage
    exit 0
fi

if [ "$1" = "--clean" ]; then
    cleanup_stale_processes
    print_success "Stale processes cleaned up"
    exit 0
elif [ "$1" = "--host" ]; then
    # Check for --no-build flag
    skip_build=false
    if [[ "${2:-}" == "--no-build" ]]; then
        skip_build=true
    fi

    print_info "Starting Mandau with enhanced reliability features..."

    # Build and setup
    build_and_setup "$skip_build"

    # Trap signals to ensure cleanup happens
    trap cleanup EXIT INT TERM

    # Clean up any stale processes first
    cleanup_stale_processes

    # Check if ports are available
    if ! check_port $CORE_PORT; then
        print_warning "Core port $CORE_PORT might be in use. Will attempt to clean up and proceed."
    fi

    if ! check_port $AGENT_PORT; then
        print_warning "Agent port $AGENT_PORT might be in use. Will attempt to clean up and proceed."
    fi

    # Check if Docker is available
    if ! docker info >/dev/null 2>&1; then
        print_warning "Docker is not accessible. Mandau Agent may not work properly."
        print_info "Please start Docker daemon before continuing."
        read -p "Continue anyway? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi

    # Start core server
    if ! start_core; then
        print_error "Failed to start Mandau Core"
        exit 1
    fi

    # Display admin credentials information
    if [ -n "$MANDAU_ADMIN_PASS" ]; then
        print_info "Admin credentials: user=$MANDAU_ADMIN_USER, password=$MANDAU_ADMIN_PASS"
    else
        print_info "Admin user: $MANDAU_ADMIN_USER (password auto-generated, check ~/.mandau/admin.password)"
        if [ -f "$HOME/.mandau/admin.password" ]; then
            print_info "Current admin password: $(cat $HOME/.mandau/admin.password)"
        fi
    fi

    # Wait a bit before starting agent
    sleep 3

    # Start agent
    if ! start_agent; then
        print_error "Failed to start Mandau Agent"
        exit 1
    fi

    # Wait for agent to register
    sleep 5

    # Show agent status
    show_agent_status

    # Start monitoring
    monitor_processes

elif [ "$1" = "--host-with-port" ]; then
    # Allow specifying custom ports
    if [ -z "${2:-}" ] || [ -z "${3:-}" ]; then
        print_error "Please specify both core and agent port numbers with --host-with-port"
        show_usage
        exit 1
    fi

    CUSTOM_CORE_PORT=$2
    CUSTOM_AGENT_PORT=$3

    # Validate port numbers
    if ! [[ "$CUSTOM_CORE_PORT" =~ ^[0-9]+$ ]] || [ "$CUSTOM_CORE_PORT" -lt 1 ] || [ "$CUSTOM_CORE_PORT" -gt 65535 ]; then
        print_error "Invalid core port number: $CUSTOM_CORE_PORT. Port must be between 1 and 65535."
        exit 1
    fi

    if ! [[ "$CUSTOM_AGENT_PORT" =~ ^[0-9]+$ ]] || [ "$CUSTOM_AGENT_PORT" -lt 1 ] || [ "$CUSTOM_AGENT_PORT" -gt 65535 ]; then
        print_error "Invalid agent port number: $CUSTOM_AGENT_PORT. Port must be between 1 and 65535."
        exit 1
    fi

    MANDAU_CORE_PORT=$CUSTOM_CORE_PORT
    MANDAU_AGENT_PORT=$CUSTOM_AGENT_PORT

    # Check for --no-build flag
    skip_build=false
    if [[ "${4:-}" == "--no-build" ]]; then
        skip_build=true
    fi

    print_info "Starting Mandau with custom ports: Core=$CUSTOM_CORE_PORT, Agent=$CUSTOM_AGENT_PORT..."

    # Build and setup
    build_and_setup "$skip_build"

    # Trap signals to ensure cleanup happens
    trap cleanup EXIT INT TERM

    # Clean up any stale processes first
    cleanup_stale_processes

    # Check if ports are available
    if ! check_port $CUSTOM_CORE_PORT; then
        print_error "Port $CUSTOM_CORE_PORT is already in use."
        exit 1
    fi

    if ! check_port $CUSTOM_AGENT_PORT; then
        print_error "Port $CUSTOM_AGENT_PORT is already in use."
        exit 1
    fi

    # Check if Docker is available
    if ! docker info >/dev/null 2>&1; then
        print_warning "Docker is not accessible. Mandau Agent may not work properly."
        print_info "Please start Docker daemon before continuing."
        read -p "Continue anyway? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi

    # Start core server
    if ! start_core; then
        print_error "Failed to start Mandau Core"
        exit 1
    fi

    # Display admin credentials information
    if [ -n "$MANDAU_ADMIN_PASS" ]; then
        print_info "Admin credentials: user=$MANDAU_ADMIN_USER, password=$MANDAU_ADMIN_PASS"
    else
        print_info "Admin user: $MANDAU_ADMIN_USER (password auto-generated, check ~/.mandau/admin.password)"
        if [ -f "$HOME/.mandau/admin.password" ]; then
            print_info "Current admin password: $(cat $HOME/.mandau/admin.password)"
        fi
    fi

    # Wait a bit before starting agent
    sleep 3

    # Start agent
    if ! start_agent; then
        print_error "Failed to start Mandau Agent"
        exit 1
    fi

    # Wait for agent to register
    sleep 5

    # Show agent status
    show_agent_status

    # Start monitoring
    monitor_processes

else
    print_error "Unknown option: $1"
    show_usage
    exit 1
fi
