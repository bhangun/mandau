#!/bin/bash
set -e

# Mandau Local Installer Script
# Builds the binaries for the current platform and installs them to /usr/local/bin/

echo "🚀 Building Mandau for local system..."
./build-mandau.sh

echo "📦 Installing binaries to /usr/local/bin/ (this requires sudo)..."
sudo cp bin/mandau bin/mandau-core bin/mandau-agent /usr/local/bin/
sudo chmod +x /usr/local/bin/mandau /usr/local/bin/mandau-core /usr/local/bin/mandau-agent

if [ -d "$HOME/.local/bin" ]; then
    echo "📦 Copying to $HOME/.local/bin/ as well (often has higher path precedence)..."
    cp bin/mandau bin/mandau-core bin/mandau-agent "$HOME/.local/bin/"
    chmod +x "$HOME/.local/bin/mandau" "$HOME/.local/bin/mandau-core" "$HOME/.local/bin/mandau-agent"
fi

echo "✅ Installation complete! You can now run 'mandau' directly."
