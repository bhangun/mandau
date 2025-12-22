#!/bin/bash

# Mandau Production Runner Script
# This script builds and runs Mandau in production mode

set -e

echo "Building Mandau for production..."

# Navigate to the mandau directory
cd "$(dirname "$0")"

# Build the static binaries
make build-static

# Ensure certificates exist
if [ ! -d "certs" ]; then
    echo "Generating certificates..."
    make certs
fi

# Create necessary directories
mkdir -p /var/lib/mandau/stacks
mkdir -p /etc/mandau/config
mkdir -p /etc/mandau/certs

# Copy configs if they exist
if [ -d "config" ]; then
    cp -r config/* /etc/mandau/config/ 2>/dev/null || true
fi

# Copy certificates
cp certs/* /etc/mandau/certs/ 2>/dev/null || true

# Install binaries if not already installed
if [ ! -f "/usr/local/bin/mandau-agent" ]; then
    echo "Installing mandau binaries..."
    sudo make install
fi

# Create mandau user if it doesn't exist
if ! id -u mandau > /dev/null 2>&1; then
    echo "Creating mandau user..."
    sudo useradd --system --shell /bin/false --home /var/lib/mandau --create-home mandau
    sudo usermod -aG docker mandau
fi

# Install systemd services
echo "Installing systemd services..."
sudo cp script-deploy/mandau-core.service /etc/systemd/system/
sudo cp script-deploy/mandau-agent.service /etc/systemd/system/
sudo systemctl daemon-reload

echo "Starting Mandau services..."
sudo systemctl enable mandau-core
sudo systemctl enable mandau-agent
sudo systemctl start mandau-core
sudo systemctl start mandau-agent

echo "Mandau installation complete!"
echo "Services: mandau-core and mandau-agent"
echo "Check status with: sudo systemctl status mandau-core mandau-agent"