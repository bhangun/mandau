
## Prerequisites

- Docker 20.10+
- Go 1.21+ (for building from source)
- OpenSSL (for certificate generation)

## Installation

### Option 1: Docker Compose (Development)

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

### Option 2: Host Installation (Production)

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

### Option 3: Manual Binary Installation (Production)

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

**Option 1: Using environment variables**
```bash
export MANDAU_SERVER=localhost:8443
export MANDAU_CERT=/etc/mandau/certs/client.crt
export MANDAU_KEY=/etc/mandau/certs/client.key
export MANDAU_CA=/etc/mandau/certs/ca.crt
```

**Option 2: Using command-line flags**
```bash
mandau --server localhost:8443 --cert /etc/mandau/certs/client.crt --key /etc/mandau/certs/client.key --ca /etc/mandau/certs/ca.crt agent list
```

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

# Apply to agent
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

### Service Management

```bash
# Nginx management
mandau services nginx create-proxy agent-001 example.com http://localhost:3000 80
mandau services nginx list agent-001

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

# DNS management
mandau services dns create-zone agent-001 example.com
mandau services dns add-a agent-001 example.com www 192.168.1.100
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

**Error: "certificate signed by unknown authority"**
- Check that the correct CA certificate is being used
- Regenerate certificates if needed: `make certs`

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
  ./run-dev.sh --host-with-port 8445 8446
  ```

**Connection refused errors**
- Verify both core and agent are running
- Check firewall settings if running across networks
- Ensure certificates are properly configured
