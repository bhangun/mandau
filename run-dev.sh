#!/bin/bash

# Mandau Development Runner Script
# This script runs Mandau in development mode using Docker Compose or on host

set -euo pipefail  # More comprehensive error handling

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
    if [ ! -x "./bin/$exe" ]; then
        print_error "Executable './bin/$exe' does not exist or is not executable."
        print_info "Run 'make build' to build the binaries first."
        exit 1
    fi
}

# Function to cleanup processes on exit
cleanup() {
    if [[ -n "${CORE_PID:-}" ]] && kill -0 $CORE_PID 2>/dev/null; then
        print_info "Stopping Mandau Core (PID: $CORE_PID)..."
        kill $CORE_PID 2>/dev/null
        wait $CORE_PID 2>/dev/null || true
    fi
}

# Function to show usage
show_usage() {
    echo "Usage: $0 [--host | --host-with-port PORT | --help]"
    echo "  --host              Run Mandau in development mode on host (port 8443)"
    echo "  --host-with-port    Run Mandau in development mode on host with custom port"
    echo "  --help              Show this help message"
}

# Trap signals to ensure cleanup happens
trap cleanup EXIT INT TERM

# Parse arguments
if [ "$#" -eq 0 ] || [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    show_usage
    exit 0
fi

if [ "$1" = "--host" ]; then
    print_info "Starting Mandau in development mode on host..."

    # Navigate to the mandau directory
    cd "$(dirname "$0")"

    # Check if required binaries exist
    check_executable "mandau-core"
    check_executable "mandau-agent"

    # Ensure certificates exist
    if [ ! -d "certs" ]; then
        print_info "Generating certificates..."
        make certs
    fi

    # Ensure stacks directory exists
    mkdir -p stacks

    # Check if default port is available
    CORE_PORT="8443"
    if ! check_port $CORE_PORT; then
        print_error "Port $CORE_PORT is already in use. Please stop the existing process or use a different port."
        print_info "Check what's using the port with: lsof -i :$CORE_PORT"
        exit 1
    fi

    # Check if Docker is available
    if ! docker info >/dev/null 2>&1; then
        print_warning "Docker is not accessible. Mandau Agent may not work properly."
        print_info "Please start Docker daemon before running this script."
        read -p "Continue anyway? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi

    # Run core in background
    print_info "Starting Mandau Core on port $CORE_PORT..."
    ./bin/mandau-core --cert certs/core.crt --key certs/core.key --ca certs/ca.crt &
    CORE_PID=$!

    # Wait a bit for core to start
    sleep 2

    # Check if core started successfully
    if ! kill -0 $CORE_PID 2>/dev/null; then
        print_error "Mandau Core failed to start"
        exit 1
    fi

    print_success "Mandau Core is running (PID: $CORE_PID)"

    # Run agent in foreground
    print_info "Starting Mandau Agent..."
    ./bin/mandau-agent --cert certs/agent.crt --key certs/agent.key --ca certs/ca.crt --stack-root ./stacks

    # When agent exits, kill core
    # This will be handled by the trap function
elif [ "$1" = "--host-with-port" ]; then
    # Allow specifying a custom port
    if [ -z "${2:-}" ]; then
        print_error "Please specify a port number with --host-with-port"
        show_usage
        exit 1
    fi

    CUSTOM_PORT=$2
    print_info "Starting Mandau in development mode on host with custom port $CUSTOM_PORT..."

    # Validate port number
    if ! [[ "$CUSTOM_PORT" =~ ^[0-9]+$ ]] || [ "$CUSTOM_PORT" -lt 1 ] || [ "$CUSTOM_PORT" -gt 65535 ]; then
        print_error "Invalid port number: $CUSTOM_PORT. Port must be between 1 and 65535."
        exit 1
    fi

    # Navigate to the mandau directory
    cd "$(dirname "$0")"

    # Check if required binaries exist
    check_executable "mandau-core"
    check_executable "mandau-agent"

    # Ensure certificates exist
    if [ ! -d "certs" ]; then
        print_info "Generating certificates..."
        make certs
    fi

    # Ensure stacks directory exists
    mkdir -p stacks

    # Check if custom port is available
    if ! check_port $CUSTOM_PORT; then
        print_error "Port $CUSTOM_PORT is already in use. Please stop the existing process or use a different port."
        print_info "Check what's using the port with: lsof -i :$CUSTOM_PORT"
        exit 1
    fi

    # Check if Docker is available
    if ! docker info >/dev/null 2>&1; then
        print_warning "Docker is not accessible. Mandau Agent may not work properly."
        print_info "Please start Docker daemon before running this script."
        read -p "Continue anyway? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi

    # Run core in background with custom port
    print_info "Starting Mandau Core on port $CUSTOM_PORT..."
    ./bin/mandau-core --listen ":$CUSTOM_PORT" --cert certs/core.crt --key certs/core.key --ca certs/ca.crt &
    CORE_PID=$!

    # Wait a bit for core to start
    sleep 2

    # Check if core started successfully
    if ! kill -0 $CORE_PID 2>/dev/null; then
        print_error "Mandau Core failed to start"
        exit 1
    fi

    print_success "Mandau Core is running (PID: $CORE_PID) on port $CUSTOM_PORT"

    # Run agent in foreground
    print_info "Starting Mandau Agent..."
    ./bin/mandau-agent --cert certs/agent.crt --key certs/agent.key --ca certs/ca.crt --stack-root ./stacks

    # When agent exits, kill core
    # This will be handled by the trap function
else
    print_error "Unknown option: $1"
    show_usage
    exit 1
fi