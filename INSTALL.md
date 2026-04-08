# 🗡️ Mandau Installation Guide

## Quick Install

### Linux/macOS (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/install.sh | sudo bash
```

### Windows (PowerShell)

```powershell
# Download and run installer
Invoke-WebRequest -Uri "https://github.com/bhangun/mandau/releases/latest/download/install.sh" -OutFile "install.sh"
bash install.sh
```

---

## Platform-Specific Instructions

### Ubuntu/Debian Linux

```bash
# Install dependencies
sudo apt-get update
sudo apt-get install -y curl openssl

# Install Mandau
curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/install.sh | sudo bash

# Verify installation
mandau --help
mandau-core --version
mandau-agent --version
```

### RHEL/CentOS/Fedora Linux

```bash
# Install dependencies
sudo dnf install -y curl openssl

# Install Mandau
curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/install.sh | sudo bash
```

### macOS (Homebrew)

```bash
# Install dependencies
brew install curl openssl

# Install Mandau
curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/install.sh | bash

# Note: On macOS, you may need to run without sudo
# The script will install to /usr/local/bin by default
```

### macOS (Apple Silicon)

```bash
# The installer auto-detects ARM64 architecture
curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/install.sh | bash

# Verify architecture
file $(which mandau)
# Should show: Mach-O 64-bit executable arm64
```

### Windows (WSL2)

```bash
# Inside WSL2
curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/install.sh | sudo bash
```

### Windows (Native)

1. Download the latest release from [GitHub Releases](https://github.com/bhangun/mandau/releases)
2. Extract the `.zip` file
3. Add the extracted directory to your `PATH` environment variable

```powershell
# Example: Download and extract
$version = "v0.1.0"  # Replace with latest version
$url = "https://github.com/bhangun/mandau/releases/download/$version/mandau-windows-amd64-$version.zip"
Invoke-WebRequest -Uri $url -OutFile "mandau.zip"
Expand-Archive -Path "mandau.zip" -DestinationPath "C:\mandau"

# Add to PATH (current session only)
$env:Path += ";C:\mandau"

# Verify
.\mandau.exe --help
```

---

## Manual Installation

### From Release Binaries

1. Go to [GitHub Releases](https://github.com/bhangun/mandau/releases)
2. Download the archive for your platform:
   - `mandau-linux-amd64-vX.Y.Z.tar.gz` - Linux 64-bit
   - `mandau-linux-arm64-vX.Y.Z.tar.gz` - Linux ARM (Raspberry Pi, AWS Graviton)
   - `mandau-darwin-amd64-vX.Y.Z.tar.gz` - macOS Intel
   - `mandau-darwin-arm64-vX.Y.Z.tar.gz` - macOS Apple Silicon
   - `mandau-windows-amd64-vX.Y.Z.zip` - Windows 64-bit

3. Extract and install:

```bash
# Linux/macOS
VERSION="v0.1.0"  # Replace with actual version
curl -fsSL "https://github.com/bhangun/mandau/releases/download/${VERSION}/mandau-linux-amd64-${VERSION}.tar.gz" -o mandau.tar.gz
tar -xzf mandau.tar.gz
sudo mv mandau mandau-core mandau-agent /usr/local/bin/

# Verify checksums
curl -fsSL "https://github.com/bhangun/mandau/releases/download/${VERSION}/SHA256SUMS.txt" -o SHA256SUMS.txt
sha256sum -c SHA256SUMS.txt
```

### From Source

```bash
# Prerequisites
# - Go 1.24 or later
# - protoc (Protocol Buffers compiler)
# - Make

# Clone repository
git clone https://github.com/bhangun/mandau.git
cd mandau

# Build binaries
make build

# Install to /usr/local/bin
sudo make install

# Generate certificates
make certs
```

---

## Post-Installation Setup

### 1. Verify Installation

```bash
# Check binaries are installed
which mandau mandau-core mandau-agent

# Check versions
mandau --help
mandau-core --version
mandau-agent --version
```

### 2. Generate Certificates

The installation script automatically generates certificates in `~/mandau-certs/`. If you need to regenerate:

```bash
# Using make (if you have the source)
make certs

# Or using the script directly
curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/generate-certs.sh -o generate-certs.sh
chmod +x generate-certs.sh
./generate-certs.sh ~/mandau-certs
```

This creates:
- `ca.crt` / `ca.key` - Certificate Authority
- `core.crt` / `core.key` - Core server certificate
- `agent.crt` / `agent.key` - Agent certificate
- `client.crt` / `client.key` - CLI client certificate

### 3. Start Services

#### Option A: Using systemd (Linux)

The installation script creates systemd service files automatically:

```bash
# Enable and start services
sudo systemctl daemon-reload
sudo systemctl enable mandau-core
sudo systemctl enable mandau-agent
sudo systemctl start mandau-core
sudo systemctl start mandau-agent

# Check status
sudo systemctl status mandau-core
sudo systemctl status mandau-agent

# View logs
sudo journalctl -u mandau-core -f
sudo journalctl -u mandau-agent -f
```

#### Option B: Manual Start

```bash
# Start Core server
mandau-core \
  --listen :8443 \
  --cert ~/mandau-certs/core.crt \
  --key ~/mandau-certs/core.key \
  --ca ~/mandau-certs/ca.crt

# In another terminal, start Agent
mandau-agent \
  --server localhost:8443 \
  --cert ~/mandau-certs/agent.crt \
  --key ~/mandau-certs/agent.key \
  --ca ~/mandau-certs/ca.crt \
  --stack-root ~/mandau-stacks
```

### 4. Access Web Dashboard

Once Core is running, access the web dashboard:

```bash
# Open in browser
open http://localhost:8080  # macOS
xdg-open http://localhost:8080  # Linux

# Default credentials
# Username: admin
# Password: (shown in Core server logs during first start)
```

**⚠️ IMPORTANT**: On first start, Mandau generates a random admin password. Check the Core server logs:

```bash
# Look for this line in logs
log.Printf("Generated random admin password: %s", adminPass)
```

Or set your own via environment variables:

```bash
export MANDAU_ADMIN_USER=admin
export MANDAU_ADMIN_PASS=your-secure-password
mandau-core --listen :8443 ...
```

---

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `MANDAU_JWT_SECRET` | JWT signing secret (256-bit) | Auto-generated |
| `MANDAU_ADMIN_USER` | Admin username | `admin` |
| `MANDAU_ADMIN_PASS` | Admin password | Auto-generated |
| `MANDAU_CONFIG_PATH` | Custom config file path | `~/.mandau/config.yaml` |

### Configuration Files

After installation, configuration files are created in `~/.mandau/`:

```
~/.mandau/
├── config.yaml      # Default configuration
├── dev.yaml         # Development profile
└── local.yaml       # Local profile
```

Example `config.yaml`:

```yaml
server:
  listen_addr: ":8443"
  tls:
    cert_path: "~/mandau-certs/core.crt"
    key_path: "~/mandau-certs/core.key"
    ca_path: "~/mandau-certs/ca.crt"
    min_version: "TLS1.3"
    server_name: "mandau-core"
timeout: "30s"
```

---

## Uninstallation

### Linux/macOS

```bash
# Stop services
sudo systemctl stop mandau-core mandau-agent
sudo systemctl disable mandau-core mandau-agent

# Remove binaries
sudo rm /usr/local/bin/mandau /usr/local/bin/mandau-core /usr/local/bin/mandau-agent

# Remove service files
sudo rm /etc/systemd/system/mandau-core.service /etc/systemd/system/mandau-agent.service
sudo systemctl daemon-reload

# Remove configuration and data (optional)
rm -rf ~/.mandau ~/mandau-certs ~/mandau-stacks

# Reload systemd
sudo systemctl daemon-reload
```

### Windows

```powershell
# Remove from PATH (manual)
# Edit system environment variables and remove Mandau from PATH

# Remove binaries
Remove-Item -Recurse -Force "C:\mandau"
```

---

## Troubleshooting

### Installation Fails

```bash
# Check internet connection
curl -I https://github.com

# Check if curl is installed
which curl

# Manual download if curl fails
wget https://raw.githubusercontent.com/bhangun/mandau/main/scripts/install.sh
bash install.sh
```

### Certificate Errors

```bash
# Regenerate certificates
rm -rf ~/mandau-certs
curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/generate-certs.sh | bash -s ~/mandau-certs
```

### Port Already in Use

```bash
# Check what's using the port
sudo lsof -i :8443
sudo lsof -i :8080

# Kill the process
sudo kill -9 <PID>
```

### Permission Denied

```bash
# Fix binary permissions
sudo chmod +x /usr/local/bin/mandau /usr/local/bin/mandau-core /usr/local/bin/mandau-agent

# Fix certificate permissions
chmod 600 ~/mandau-certs/*.key
chmod 644 ~/mandau-certs/*.crt
```

---

## Next Steps

- [Read the README](README.md) for usage guide
- [View API Documentation](docs/API.md)
- [Join Discussions](https://github.com/bhangun/mandau/discussions)
- [Report Issues](https://github.com/bhangun/mandau/issues)
