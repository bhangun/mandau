#!/bin/bash
set -e

# Ensure Homebrew path is included for Go
export PATH=$PATH:/opt/homebrew/bin:/usr/local/bin

BIN_DIR="./bin"
mkdir -p $BIN_DIR

# Check if we should cross-compile for Linux
if [ "$1" == "--linux" ]; then
    echo "🌍 Cross-compiling for Linux AMD64..."
    export GOOS=linux
    export GOARCH=amd64
    OS="linux"
    ARCH="amd64"
else
    # Build for current platform
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    if [ "$ARCH" = "x86_64" ]; then ARCH="amd64"; fi
    if [ "$ARCH" = "arm64" ] || [ "$ARCH" = "aarch64" ]; then ARCH="arm64"; fi
    echo "🚀 Building Mandau for local ($OS/$ARCH)..."
fi

# Ensure proto files are generated
make proto

echo "Building Mandau Core..."
go build -o $BIN_DIR/mandau-core ./cmd/mandau-core

echo "Building Mandau Agent..."
go build -o $BIN_DIR/mandau-agent ./cmd/mandau-agent

echo "Building Mandau CLI..."
go build -o $BIN_DIR/mandau ./cmd/mandau-cli

echo "✅ Build complete! Binaries located in: $BIN_DIR/"
