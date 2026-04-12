
## Prerequisites

- Docker 20.10+
- Go 1.21+ (for building from source)
- OpenSSL (for certificate generation)
- SSH access (for production deployments)

## Installation

### Option 1: Install from Binary Release (Recommended for Production)

Download the appropriate binary package for your platform from the [releases page](https://github.com/bhangun/mandau/releases). Each release includes pre-built static binaries for:

- Linux AMD64/ARM64
- macOS AMD64/ARM64
- Windows AMD64

**Quick Install Script:**
```bash
# Download and run the installation script (Linux/macOS)
curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/install.sh | bash
```

**Manual Installation for Linux/macOS:**
```bash
# Download and extract the archive for your platform
# Example for Linux AMD64:
VERSION=v1.0.0  # Replace with the latest version
wget https://github.com/bhangun/mandau/releases/download/${VERSION}/mandau-linux-amd64-${VERSION}.tar.gz
tar -xzf mandau-linux-amd64-${VERSION}.tar.gz

# Make binaries executable and move to PATH
sudo chmod +x mandau mandau-core mandau-agent
sudo mv mandau mandau-core mandau-agent /usr/local/bin/

# Generate certificates (development only)
./scripts/generate-certs.sh ./certs
```

**⚠️ Production Note:** For production deployments, use centralized CA management. See [Certificate Management Guide](CERTIFICATE_MANAGEMENT.md) for details.

**Quick Install with Production Certificates:**
```bash
# Set environment variables for production deployment
export MANDAU_CORE_HOSTNAME=mandau-core.example.com
export MANDAU_CORE_IP=192.168.1.100
export MANDAU_AGENT_HOSTNAME=agent-us-east-1
export MANDAU_AGENT_IP=192.168.1.101

# Install with custom hostnames
curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/install.sh | bash
```

**For Windows:**
```powershell
# Download the zip file for Windows AMD64 from the releases page
# Example using PowerShell:
$version = "v1.0.0"  # Replace with the latest version
$downloadUrl = "https://github.com/bhangun/mandau/releases/download/$version/mandau-windows-amd64-$version.zip"
$outputPath = "$env:TEMP\mandau-windows-amd64-$version.zip"
Invoke-WebRequest -Uri $downloadUrl -OutFile $outputPath

# Extract the zip file
Expand-Archive -Path $outputPath -DestinationPath "$env:TEMP\mandau"
# Copy mandau.exe, mandau-core.exe, and mandau-agent.exe to a directory in your PATH
```

### Option 2: Docker Compose (Development)

1. **Clone the repository:**
   ```bash
   git clone https://github.com/bhangun/mandau.git
   cd mandau
   ```

2. **Generate certificates:**
   ```bash
   make certs
   ```

3. **Start services:**
   ```bash
   docker-compose -f docker-compose-dev.yaml up -d
   ```

   Or run on host with enhanced reliability (recommended for development):
   ```bash
   # Clean up any stale processes
   ./run-dev.sh --clean

   # Start with automatic restarts and connection recovery (default behavior)
   ./run-dev.sh
   # or explicitly
   ./run-dev.sh --host
   ```

4. **Verify:**
   ```bash
   docker-compose ps
   ```

**Note:** Default port is now **3443** (not 8443). Update your CLI configuration accordingly.

### Option 3: Host Installation (Production)

**⚠️ Important:** Default port is now **3443** (not 8443). See [Certificate Management Guide](CERTIFICATE_MANAGEMENT.md) for production certificate setup.

1. **Clone the repository:**
   ```bash
   git clone https://github.com/bhangun/mandau.git
   cd mandau
   ```

2. **Run the production installer:**
   ```bash
   sudo ./run-prod.sh
   ```

   This will:
   - Build static binaries
   - Create the `mandau` user and add it to the `docker` group
   - Install binaries to `/usr/local/bin`
   - Install systemd services
   - Start the services

3. **Verify installation:**
   ```bash
   sudo systemctl status mandau-core mandau-agent
   ```

**Production Certificate Deployment:**

For multi-server deployments, use the certificate distribution script:

```bash
# Deploy to core server
./scripts/cert-distribute.sh \
  --type core \
  --host 192.168.1.100 \
  --agent-hostname mandau-core.example.com \
  --agent-ip 192.168.1.100

# Deploy to agent servers (generates unique certs per agent)
./scripts/cert-distribute.sh \
  --type agent \
  --host 192.168.1.101 \
  --agent-hostname agent-us-east-1 \
  --agent-ip 192.168.1.101
```

See [Certificate Management Guide](CERTIFICATE_MANAGEMENT.md) for complete production deployment procedures.

### Option 4: Manual Binary Installation (Production)

1. **Build static binaries:**
   ```bash
   make build-static
   ```

2. **Generate certificates:**
   ```bash
   ./scripts/generate-certs.sh ./certs
   ```

3. **Create mandau user:**
   ```bash
   sudo useradd --system --shell /bin/false --home /var/lib/mandau --create-home mandau
   sudo usermod -aG docker mandau
   ```

4. **Install binaries and configs:**
   ```bash
   sudo make install
   sudo mkdir -p /etc/mandau/{config,certs}
   sudo cp certs/* /etc/mandau/certs/
   # Copy config files as needed
   ```

5. **Install systemd services:**
   ```bash
   sudo cp mandau-*.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable mandau-core mandau-agent
   sudo systemctl start mandau-core mandau-agent
   ```

6. **Verify:**
   ```bash
   sudo systemctl status mandau-core mandau-agent
   ```

## Usage

### CLI Configuration

**Option 1: Auto-discovery (Recommended)**

The CLI automatically discovers certificates in `~/.mandau/certs/`:

```bash
# Generate certificates (if not already done)
mandau cert gen

# No additional config needed - just use the CLI!
mandau agent list
```

**Option 2: Using environment variables**
```bash
export MANDAU_SERVER=localhost:3443
export MANDAU_CERT=~/.mandau/certs/client.crt
export MANDAU_KEY=~/.mandau/certs/client.key
export MANDAU_CA=~/.mandau/certs/ca.crt
```

**Option 3: Using command-line flags**
```bash
mandau --server localhost:3443 \
  --cert ~/.mandau/certs/client.crt \
  --key ~/.mandau/certs/client.key \
  --ca ~/.mandau/certs/ca.crt \
  agent list
```

**⚠️ Port Change:** The default core server port has been changed from **8443** to **3443**. Update your configurations accordingly.

### List Agents

```bash
mandau agent list
```

### Deploy a Stack

```bash
# Create compose file
cat > mystack.yaml <<EOF
version: '3.8'
services:
  web:
    image: nginx:alpine
    ports:
      - "80:80"
EOF

# Apply to agent (Smart naming: uses directory name 'my-app')
mandau apply my-app/docker-compose.yaml

# Or specify a stack name explicitly (old way)
mandau stack apply agent-001 mystack mystack.yaml
```

### Stream Logs

```bash
mandau stack logs agent-001 mystack
```

### Execute Command

```bash
mandau container exec agent-001 mystack-web-1 /bin/sh
```

### Container Management

```bash
# List containers on an agent
mandau container list agent-001

# Get container logs
mandau container logs agent-001 mystack-web-1

# Start/stop containers
mandau container start agent-001 mystack-web-1
mandau container stop agent-001 mystack-web-1
```

### Filesystem Management

```bash
# List files on agent
mandau fs ls /var/log

# Copy from remote to local
mandau fs fetch /var/log/syslog ./local-syslog

# Copy from local to remote
mandau fs cp ./config.yaml /etc/myapp/config.yaml

# View text file
mandau fs cat /etc/hosts
```

### Interactive Host Shell

Open a fully interactive shell session on a remote agent — just like SSH, but secured through Mandau's mTLS infrastructure:

```bash
# Open shell on default agent
mandau shell

# Open shell on a specific agent
mandau shell agent-insanserver
```

Once connected, you have a full bash session. Use `exit` or `Ctrl+D` to disconnect.

> **Security Note:** Host shell access can be disabled per-agent by setting `disable_host_shell: true` in the agent's `security` config section.

### Stack Deployment

```bash
# Deploy a compose file (defaults to 'up -d')
mandau apply my-stack.yaml

# Deploy with explicit daemon mode
mandau apply my-stack.yaml up -d

# Bring a stack down
mandau apply my-stack.yaml down

# Deploy to a specific agent
mandau -a agent-002 apply my-stack.yaml
```

The `.env` file adjacent to your compose file is automatically uploaded to the agent.

### Service Management

```bash
# Nginx management
mandau services nginx list agent-001
mandau services nginx create-proxy agent-001 example.com http://localhost:3000 80
mandau services nginx reload agent-001

# Systemd service management
mandau services systemd start agent-001 myservice
mandau services systemd status agent-001 myservice
mandau services systemd restart agent-001 myservice

# SSL certificate management
mandau services ssl obtain agent-001 example.com admin@example.com
mandau services ssl renew-all agent-001

# Firewall management
mandau services firewall allow-port agent-001 80 tcp
mandau services firewall deny-port agent-001 8080 tcp
mandau services firewall list agent-001

# Cron job management
mandau services cron add agent-001 backup-job "0 2 * * *" "/usr/local/bin/backup.sh"
mandau services cron list agent-001

# Environment management
mandau services environment info agent-001
mandau services environment install agent-001 nginx

# DNS management (Pending)
# mandau services dns create-zone agent-001 example.com
```

### Plugin Management

```bash
# Authentication management
mandau plugins auth status
mandau plugins auth list-users

# Secrets management
mandau plugins secrets get my-secret
mandau plugins secrets set my-secret "my-value"
mandau plugins secrets delete my-secret

# Audit log management
mandau plugins audit list
mandau plugins audit query "agent:agent-001"
```

## Security Best Practices

1. **Never expose Docker socket directly**
2. **Always use mTLS for communication**
3. **Rotate certificates regularly**
4. **Enable audit logging**
5. **Use RBAC for access control**
6. **Store secrets in Vault or similar**

## Troubleshooting

Check agent logs:
```bash
sudo journalctl -u mandau-agent -f
```

Check core logs:
```bash
sudo journalctl -u mandau-core -f
```

Verify certificates:
```bash
openssl verify -CAfile /etc/mandau/certs/ca.crt \
  /etc/mandau/certs/agent.crt
```

### Common TLS Issues

**Error: "tls: bad certificate"**
- Ensure the CA certificate is provided to both client and server
- Use `--ca` flag or `MANDAU_CA` environment variable
- Verify certificates are properly signed by the same CA
- See [Certificate Management Guide](CERTIFICATE_MANAGEMENT.md) for troubleshooting

**Error: "certificate signed by unknown authority"**
- Check that the correct CA certificate is being used
- Regenerate certificates if needed: `make certs`
- For production, ensure all servers use the same centralized CA

### Common Connection and Process Issues

**Agent showing as "offline"**
- Check if both core and agent processes are running:
  ```bash
  ps aux | grep mandau
  ```
- Clean up stale processes and restart:
  ```bash
  ./run-dev.sh --clean
  ./run-dev.sh
  # or explicitly
  ./run-dev.sh --host
  ```
- Verify ports are available:
  ```bash
  lsof -i :8443  # Core server
  lsof -i :8444  # Agent server
  ```

**Port conflicts**
- If you see "address already in use" errors, clean up stale processes:
  ```bash
  ./run-dev.sh --clean
  ```
- Or use different ports:
  ```bash
  ./run-dev.sh --host-with-port 9445 9446
  ```
- Check what's using the port:
  ```bash
  lsof -i :3443  # Core server
  lsof -i :9444  # Agent server
  ```

**Connection refused errors**
- Verify both core and agent are running
- Check firewall settings if running across networks
- Ensure certificates are properly configured
