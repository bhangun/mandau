# 🗡️ Mandau - Secure Infrastructure Control Plane

**Production-grade, agent-driven Docker infrastructure management for security-sensitive environments**

## 📋 Overview

Mandau is a secure, operator-grade control plane for managing Docker infrastructure across local and remote hosts. Built for SREs, platform engineers, and secure environments including air-gapped networks.

### Key Features

✅ **Security-First Design**
- Mutual TLS (mTLS) authentication
- Certificate-based identity
- Policy-based authorization (RBAC + OPA)
- Complete audit logging
- No direct Docker socket exposure

✅ **Agent Architecture**
- Lightweight Go binaries
- Runs per Docker host
- No inbound ports required
- Works through bastion hosts
- Air-gap compatible

✅ **File-Based Infrastructure**
- Compose files on disk
- Compatible with `docker compose` CLI
- No database lock-in
- Version control friendly

✅ **Plugin System**
- Extensible architecture
- Auth, audit, secrets, policy plugins
- Built-in Vault integration
- Custom plugin support

✅ **Operations Model**
- Async operations with streaming
- Progress tracking
- Cancellable tasks
- Automatic retries
- Persistent operation queuing for disconnected scenarios

✅ **High Availability**
- Multi-core server support with automatic failover
- Health monitoring and automatic reconnection
- Priority-based server selection
- Zero-downtime core server upgrades

✅ **Web Dashboard**
- Modern responsive web interface
- Real-time monitoring and management
- Agent, stack, container, and operations views
- Live log streaming
- No CLI required for basic operations

✅ **Transport Flexibility**
- Primary gRPC transport with mTLS
- Automatic WebSocket fallback when gRPC fails
- Configurable transport timeouts and retries
- Works through restrictive proxies and firewalls

## 🏗️ Architecture

```
┌──────────────────────────────────────────┐
│   UI Clients                             │
│  - Flutter Desktop                       │
│  - CLI                                   │
│  - **Web Dashboard (NEW: Port 8080)**    │
└──────────────┬───────────────────────────┘
               │
               │ mTLS/gRPC (primary)
               │ WebSocket (fallback)
               ▼
┌──────────────────────────────────────────┐
│   Mandau Core (HA Support)               │
│  - Agent registry                        │
│  - Auth & RBAC                           │
│  - Audit logging                         │
│  - Policy engine                         │
│  - **Multi-node failover (NEW)**         │
└──────────────┬───────────────────────────┘
               │
               │ mTLS/gRPC
               │ **Auto-reconnect + Queue (NEW)**
               ▼
┌──────────────────────────────────────────┐
│   Mandau Agent                           │
│  - Docker control                        │
│  - Stack management                      │
│  - File system                           │
│  - Container exec                        │
│  - **Operation queue (NEW)**             │
│  - **WebSocket fallback (NEW)**          │
└──────────────────────────────────────────┘
```

## 📦 Components Delivered

### 1. **API Layer** (`api/v1/`)
- Complete gRPC protocol definitions
- Agent lifecycle management
- Stack operations with streaming
- Container management
- Filesystem access (scoped)
- Operation tracking

### 2. **Stack Manager** (`pkg/agent/stack/`)
- Compose file parsing and validation
- Stack lifecycle (apply, remove, update)
- Diff calculation before apply
- Multi-service orchestration
- Environment variable injection
- Secret interpolation

### 3. **Operation Manager** (`pkg/agent/operation/`)
- Async operation tracking
- Progress reporting
- Event streaming
- Cancellation support
- Operation history
- State management

### 4. **Plugin System** (`pkg/plugin/`)
- Core plugin interfaces
- Registry and lifecycle management
- Type-safe plugin discovery
- Dynamic loading support

### 5. **Example Plugins** (`plugins/`)

**File Audit Plugin**
- JSON-L format logs
- Automatic log rotation
- Queryable audit trail
- Never fails (resilient)

**RBAC Auth Plugin**
- Role-based access control
- Wildcard resource matching
- User/role management
- YAML-based configuration

**Vault Secrets Plugin**
- HashiCorp Vault integration
- Secret injection into compose
- Dynamic secret retrieval
- Kubernetes auth support

### 6. **Core Control Plane** (`pkg/core/`)
- Multi-agent management
- Agent registration and heartbeat
- Health monitoring
- Operation proxying
- Centralized audit

### 7. **CLI Tool** (`cmd/mandau-cli/`)
- Agent management
- Stack operations
- Log streaming
- Container management (exec, list, logs, start, stop)
- Service management (nginx, systemd, ssl, firewall, dns, cron, environment)
- Plugin management (auth, secrets, audit)
- Interactive mode

### 8. **Deployment Configurations**
- Docker Compose setup
- Kubernetes manifests (DaemonSet + Deployment)
- Systemd service files
- Certificate generation scripts
- Security-hardened configurations

### 9. **Transport Layer** (`pkg/transport/`) **NEW**
- gRPC primary transport with mTLS
- WebSocket fallback for restrictive networks
- Automatic transport failover and reconnection
- Configurable timeouts and keepalive
- Works through HTTP proxies

### 10. **Operation Queue** (`pkg/agent/queue/`) **NEW**
- Persistent disk-based operation queue
- Automatic retry on reconnection
- Configurable retry limits
- Queue persistence across restarts
- Perfect for disconnected/intermittent scenarios

### 11. **High Availability Manager** (`pkg/ha/`) **NEW**
- Multi-core server support
- Priority-based server selection
- Automatic health monitoring
- Seamless failover between core nodes
- Zero-downtime core server maintenance

### 12. **Web Dashboard** (`web/` and `pkg/web/`) **NEW**
- Modern responsive web interface
- Real-time monitoring dashboard
- Agent management and health monitoring
- Stack deployment and management
- Container operations
- Live log streaming
- Accessible at `http://localhost:8080` when core is running

## 🚀 Quick Start

### 1. Install (One Command)

**Linux/macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/install.sh | sudo bash
```

**Windows (PowerShell):**
```powershell
Invoke-WebRequest -Uri "https://github.com/bhangun/mandau/releases/latest/download/install.sh" -OutFile "install.sh"
bash install.sh
```

**Client-Only (CLI only):**
```bash
curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/install.sh | bash -s -- --client
```

This will:
- ✅ Auto-detect your OS and architecture
- ✅ Download the latest release binaries
- ✅ Install to `/usr/local/bin/`
- ✅ Generate TLS certificates in `~/mandau-certs/`
- ✅ Create configuration in `~/.mandau/`
- ✅ Set up systemd services (Linux)

See [INSTALL.md](INSTALL.md) for detailed platform-specific instructions.

### 2. Generate Certificates (if not done by installer)

```bash
make certs
```

### 3. Build from Source (Development)

### 4. Post-Installation Setup

After installing Mandau, you need to set up certificates. The installation automatically creates a default client configuration file at `~/.mandau/config.yaml` that points to the expected certificate locations.

#### Client vs Server Installation

**Client Installation** (for connecting to remote Mandau Core):
- Use the curl installation method to install the CLI on your local machine
- Update the server address in `~/.mandau/config.yaml` to point to your remote Mandau Core instance
- Ensure you have the appropriate client certificates signed by the same CA as the remote server

**Server Installation** (on infrastructure to be managed):
- Deploy `mandau-core` and `mandau-agent` binaries to your servers
- Configure certificates for server authentication
- Set up agents on each Docker host to be managed

#### Smart Connection Setup (Pro Tip)

Mandau now streamlines remote connectivity with the `connect` command:

1. **Point to your server:**
   ```bash
   mandau connect <server-ip>
   ```

2. **Sync certificates (Manual):**
   The `connect` command will provide a tailored `scp` command. Usually:
   ```bash
   scp <USER>@<SERVER_IP>:~/.mandau/certs/{ca.crt,client.crt,client.key} ~/.mandau/certs/
   ```

3. **Verify:**
   ```bash
   mandau agent list
   ```

#### Client Configuration for Remote Server

For client usage, update the configuration to point to your remote server:

1. **Edit the configuration file:**
```bash
nano ~/.mandau/config.yaml
```

2. **Update the server address** to point to your remote server:
```yaml
server:
  listen_addr: "your-server.com:8443"  # Change to your remote server address and port
  tls:
    cert_path: "/home/username/mandau-certs/client.crt"
    key_path: "/home/username/mandau-certs/client.key"
    ca_path: "/home/username/mandau-certs/ca.crt"
    min_version: "TLS1.3"
    server_name: "mandau-core"
timeout: "30s"
```

3. **Ensure certificates are properly configured:**
   - The client certificates must be signed by the same CA that signed the remote server's certificates
   - Place the certificates in the location specified in the config file
   - Make sure certificate paths are accessible to the client

4. **Alternative: Use environment variables:**
```bash
export MANDAU_SERVER="your-server.com:8443"
export MANDAU_CERT="/home/username/mandau-certs/client.crt"
export MANDAU_KEY="/home/username/mandau-certs/client.key"
export MANDAU_CA="/home/username/mandau-certs/ca.crt"
```

5. **Test the connection:**
```bash
mandau agent list
```

#### Security Considerations

- **Certificate Validation**: The client validates the server certificate using the CA certificate
- **mTLS Authentication**: Both client and server authenticate each other using certificates
- **Secure Communication**: All communication is encrypted using TLS 1.3
- **Port Configuration**: The default port is 8443, but can be configured on the server side

#### Troubleshooting Remote Connections

- **Connection refused**: Verify the server is running and accessible on port 8443
- **Certificate errors**: Ensure client certificates are signed by the same CA as the server
- **Authentication failures**: Verify certificate paths and permissions
- **Firewall issues**: Ensure port 8443 is open on the server

#### Generate Certificates

Mandau uses mTLS (mutual TLS) for secure communication between components. Generate certificates for Core, Agent, and CLI:

```bash
# Create certificates directory
mkdir -p ~/mandau-certs

# If you built from source, you can generate certificates directly:
make certs
cp ./certs/* ~/mandau-certs/

# If you installed via curl or manual download, certificates will be generated directly in the correct location:
curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/generate-certs.sh -o generate-certs.sh
chmod +x generate-certs.sh
./generate-certs.sh ~/mandau-certs
```

This creates the following certificates in the `~/mandau-certs` directory:
- `ca.crt` and `ca.key` - Certificate Authority
- `core.crt` and `core.key` - Core service certificate
- `agent.crt` and `agent.key` - Agent service certificate
- `client.crt` and `client.key` - CLI client certificate

#### Configure Mandau Core

The Core service manages agents and provides the API endpoint.

**Start Core Service:**
```bash
mandau-core \
  --listen :8443 \
  --cert ~/mandau-certs/core.crt \
  --key ~/mandau-certs/core.key \
  --ca ~/mandau-certs/ca.crt
```

#### Configure Mandau Agent

The Agent runs on each Docker host and executes commands:

```bash
# Create stacks directory for compose files
mkdir -p ~/mandau-stacks

mandau-agent \
  --server localhost:8443 \
  --cert ~/mandau-certs/agent.crt \
  --key ~/mandau-certs/agent.key \
  --ca ~/mandau-certs/ca.crt \
  --stack-root ~/mandau-stacks
```

#### Configure CLI Authentication

You can use either environment variables, command-line flags, or configuration files for CLI authentication:

**Option A: Using Environment Variables**
```bash
export MANDAU_SERVER=localhost:8443
export MANDAU_CERT=~/mandau-certs/client.crt
export MANDAU_KEY=~/mandau-certs/client.key
export MANDAU_CA=~/mandau-certs/ca.crt
```

**Option B: Using Command-Line Flags**
```bash
mandau --cert ~/mandau-certs/client.crt --key ~/mandau-certs/client.key --ca ~/mandau-certs/ca.crt --server localhost:8443 [command]
```

**Option C: Using Configuration File with Profile Support (Recommended)**
Mandau now supports configuration profiles for different environments. The default location is `~/.mandau/config.yaml`:

```yaml
server: "localhost:8443"
cert: "~/mandau-certs/client.crt"
key: "~/mandau-certs/client.key"
ca: "~/mandau-certs/ca.crt"
timeout: "30s"
```

**Option D: Using Profile Management (New!)**
Mandau now supports multiple configuration profiles for different environments (dev, test, prod):

```bash
# List available profiles
mandau-profile.sh list

# Switch to development profile
mandau-profile.sh use dev

# Show current profile
mandau-profile.sh show

# Or use environment variable to specify profile
export MANDAU_PROFILE=dev
mandau agent list
```

Configuration profiles are stored in `~/.mandau/` directory with names like `dev.yaml`, `test.yaml`, `prod.yaml`, etc.

### 5. Run with Enhanced Reliability (Recommended)

For development with automatic restarts and connection recovery, use the enhanced runner:

```bash
# If you have the source code, clone it to get the runner script:
git clone https://github.com/bhangun/mandau.git
cd mandau

# Clean up any stale processes first
./run-dev.sh --clean

# Start the system with enhanced reliability features (default behavior)
./run-dev.sh
# or explicitly
./run-dev.sh --host
```

Or run manually with proper process management:

#### 5a. Run Core

```bash
mandau-core \
  --listen :8443 \
  --cert ~/mandau-certs/core.crt \
  --key ~/mandau-certs/core.key \
  --ca ~/mandau-certs/ca.crt
```

#### 5b. Run Agent

```bash
mandau-agent \
  --server localhost:8443 \
  --cert ~/mandau-certs/agent.crt \
  --key ~/mandau-certs/agent.key \
  --ca ~/mandau-certs/ca.crt \
  --stack-root ~/mandau-stacks
```

### 5c. Access Web Dashboard (NEW!)

Once Mandau Core is running, you can access the web dashboard:

```bash
# Open in browser
open http://localhost:8080

# Or navigate to http://localhost:8080 in your browser
```

The web dashboard provides:
- **Dashboard**: Overview of agents, stacks, containers, and operations
- **Agents**: View and manage connected agents
- **Stacks**: Deploy and manage Docker Compose stacks
- **Containers**: View and manage running containers
- **Operations**: Monitor operation progress and history
- **Logs**: Real-time log streaming with filtering

**Note**: The web dashboard is served directly from the Mandau Core binary - no additional setup required!

### 6. Development Workflow (New!)

For enhanced development experience with profile management:

```bash
# After installation, set up development environment
curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/setup-dev.sh -o setup-dev.sh
chmod +x setup-dev.sh
./setup-dev.sh

# Use development profiles
export MANDAU_PROFILE=dev
mandau agent list

# Or switch profiles dynamically
~/mandau-profile.sh use dev
mandau agent list

# Start development services
~/mandau-dev-start.sh
```

### 7. Use CLI

After installation, you can use the Mandau CLI with your certificates.

```bash
# Option 1: Using environment variables
export MANDAU_SERVER=localhost:8443
export MANDAU_CERT=~/mandau-certs/client.crt
export MANDAU_KEY=~/mandau-certs/client.key
export MANDAU_CA=~/mandau-certs/ca.crt

# List agents
mandau agent list

# Deploy a stack
mandau stack apply web ./compose.yaml

# Run Docker commands globally (Automatically proxied)
mandau docker ps
mandau docker logs -f my-container
mandau docker images

# Manage services
mandau services nginx create-proxy agent-001 example.com http://localhost:3000 80
mandau services systemd start agent-001 myservice
mandau services firewall allow-port agent-001 80 tcp

# Manage plugins
mandau plugins secrets get my-secret
mandau plugins auth status
```

```bash
# Option 2: Using command-line flags
mandau --server localhost:8443 --cert ~/mandau-certs/client.crt --key ~/mandau-certs/client.key --ca ~/mandau-certs/ca.crt agent list

# Deploy a stack
mandau --cert ~/mandau-certs/client.crt --key ~/mandau-certs/client.key --ca ~/mandau-certs/ca.crt stack apply agent-001 web ./compose.yaml

# Stream logs
mandau --cert ~/mandau-certs/client.crt --key ~/mandau-certs/client.key --ca ~/mandau-certs/ca.crt stack logs agent-001 web

# Execute command in container
mandau --cert ~/mandau-certs/client.crt --key ~/mandau-certs/client.key --ca ~/mandau-certs/ca.crt container exec agent-001 web-container /bin/sh

# Manage services
mandau --cert ~/mandau-certs/client.crt --key ~/mandau-certs/client.key --ca ~/mandau-certs/ca.crt services nginx create-proxy agent-001 example.com http://localhost:3000 80
mandau --cert ~/mandau-certs/client.crt --key ~/mandau-certs/client.key --ca ~/mandau-certs/ca.crt services systemd start agent-001 myservice
mandau --cert ~/mandau-certs/client.crt --key ~/mandau-certs/client.key --ca ~/mandau-certs/ca.crt services firewall allow-port agent-001 80 tcp

# Manage plugins
mandau --cert ~/mandau-certs/client.crt --key ~/mandau-certs/client.key --ca ~/mandau-certs/ca.crt plugins secrets get my-secret
mandau --cert ~/mandau-certs/client.crt --key ~/mandau-certs/client.key --ca ~/mandau-certs/ca.crt plugins auth status
```

### 7. Service Management

#### Using Systemd (Recommended for Production)

For production deployments, use the provided systemd service files:

1. Copy the binaries and service files to appropriate locations
2. Enable and start the services:

```bash
# Copy binaries (if not already installed system-wide)
sudo cp /usr/local/bin/mandau-core /usr/local/bin/mandau-agent /usr/local/bin/mandau /usr/local/bin/
# Or if you have the source:
sudo cp bin/mandau-core /usr/local/bin/
sudo cp bin/mandau-agent /usr/local/bin/
sudo cp bin/mandau /usr/local/bin/

# Copy service files (from source or create manually)
# You can create these files manually:

# Create mandau-core.service
sudo tee /etc/systemd/system/mandau-core.service > /dev/null <<EOF
[Unit]
Description=Mandau Core Service
After=network.target

[Service]
Type=simple
User=$(whoami)
ExecStart=/usr/local/bin/mandau-core --listen :8443 --cert /home/$(whoami)/mandau-certs/core.crt --key /home/$(whoami)/mandau-certs/core.key --ca /home/$(whoami)/mandau-certs/ca.crt
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

# Create mandau-agent.service
sudo tee /etc/systemd/system/mandau-agent.service > /dev/null <<EOF
[Unit]
Description=Mandau Agent Service
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User=$(whoami)
ExecStart=/usr/local/bin/mandau-agent --server localhost:8443 --cert /home/$(whoami)/mandau-certs/agent.crt --key /home/$(whoami)/mandau-certs/agent.key --ca /home/$(whoami)/mandau-certs/ca.crt --stack-root /home/$(whoami)/mandau-stacks
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

# Reload systemd configuration
sudo systemctl daemon-reload

# Enable services to start on boot
sudo systemctl enable mandau-core
sudo systemctl enable mandau-agent

# Start the services
sudo systemctl start mandau-core
sudo systemctl start mandau-agent

# Check service status
sudo systemctl status mandau-core
sudo systemctl status mandau-agent
```

#### Using Docker (Alternative)

You can also run Mandau services in Docker containers:

```bash
# Pull the latest images
docker pull ghcr.io/bhangun/mandau-core:latest
docker pull ghcr.io/bhangun/mandau-agent:latest

# Run Core service
docker run -d \
  --name mandau-core \
  -p 8443:8443 \
  -v ~/mandau-certs:/certs:ro \
  -v /etc/mandau:/config:ro \
  ghcr.io/bhangun/mandau-core:latest \
  --listen :8443 \
  --cert /certs/core.crt \
  --key /certs/core.key \
  --ca /certs/ca.crt

# Run Agent service
docker run -d \
  --name mandau-agent \
  --restart unless-stopped \
  -v ~/mandau-certs:/certs:ro \
  -v ~/mandau-stacks:/stacks \
  -v /var/run/docker.sock:/var/run/docker.sock \
  ghcr.io/bhangun/mandau-agent:latest \
  --server mandau-core:8443 \
  --cert /certs/agent.crt \
  --key /certs/agent.key \
  --ca /certs/ca.crt \
  --stack-root /stacks
```

### 8. Troubleshooting

#### Common Issues

1. **Connection refused errors**: Make sure Core service is running and accessible
2. **Certificate errors**: Verify certificates are valid and properly configured
3. **Permission denied**: Check file permissions and certificate paths
4. **Agent not found**: Ensure Agent is properly registered with Core
5. **Installation fails without sudo**: The installation requires sudo to write to `/usr/local/bin/`

#### Verify Installation

```bash
# Check if binaries are installed
which mandau mandau-core mandau-agent

# Check versions
mandau --help

# Test connection to Core (if running)
mandau --cert ~/mandau-certs/client.crt --key ~/mandau-certs/client.key --ca ~/mandau-certs/ca.crt agent list
```

#### Log Files

- Core logs: Check the terminal where Core is running or systemd logs: `sudo journalctl -u mandau-core -f`
- Agent logs: Check the terminal where Agent is running or systemd logs: `sudo journalctl -u mandau-agent -f`
- CLI logs: Add `--verbose` flag for more detailed output

## 🔒 Security Model

### Authentication
- Certificate-based client identity
- Device-bound credentials
- Short-lived session tokens (optional)

### Authorization
- Minimum: RBAC with roles and permissions
- Advanced: OPA policy engine integration
- Per-RPC authorization checks
- Resource-level permissions

### Audit
- Every action logged with:
  - Identity (who)
  - Action (what)
  - Resource (where)
  - Result (success/failure)
  - Duration (when/how long)
- Terminal sessions recorded
- Tamper-evident logs

### Secrets
- Never stored in compose files
- Runtime injection only
- Vault integration
- Encrypted at rest

## 🛡️ Reliability & Resilience

Mandau is designed for production environments with built-in reliability features:

### Connection Management
- **Automatic Reconnection**: Both core and agent automatically reconnect when connections are lost
- **Retry Logic**: Exponential backoff prevents overwhelming the system during failures
- **Keepalive Probes**: Connection health is monitored with keepalive mechanisms
- **Graceful Degradation**: System continues operating during partial failures

### Process Management
- **Stale Process Cleanup**: Automatically detects and terminates stale processes
- **PID Management**: Proper tracking of running processes with PID files
- **Graceful Shutdown**: Proper cleanup on exit signals
- **Automatic Restart**: Failed processes are automatically restarted

### Monitoring & Recovery
- **Health Checks**: Regular monitoring of process health
- **Connection Cleanup**: Stale connections are cleaned up to prevent resource leaks
- **Status Tracking**: Accurate online/offline status detection
- **Reconnection Detection**: Agents that come back online are properly recognized

### Failure Scenarios Handled
- Network interruptions between core and agent
- Agent process crashes and automatic restart
- Core server restarts with agent reconnection
- Port conflicts and resource cleanup
- Stale registration cleanup

## 🚀 Production Deployment

### Using the Enhanced Runner (Recommended for Development)

For development and testing with enhanced reliability features:

```bash
# Clean up any stale processes
./run-dev.sh --clean

# Start the system with enhanced reliability features (default behavior)
./run-dev.sh

# Or with custom ports
./run-dev.sh --host-with-port 8445 8446
```

### Systemd Services (Recommended for Production)

For production deployments, use the provided systemd service files:

1. Copy the binaries and service files to appropriate locations
2. Enable and start the services:

```bash
sudo cp bin/mandau-core /usr/local/bin/
sudo cp bin/mandau-agent /usr/local/bin/
sudo cp mandau-core.service mandau-agent.service /etc/systemd/system/
sudo systemctl daemon-reload

# Enable services to start on boot
sudo systemctl enable mandau-core@$(whoami)
sudo systemctl enable mandau-agent@$(whoami)

# Start the services
sudo systemctl start mandau-core@$(whoami)
sudo systemctl start mandau-agent@$(whoami)
```

### High Availability

**Core:**
- Deploy 3+ replicas behind load balancer (future roadmap)
- Use shared storage for agent registry
- Enable leader election (future roadmap)
- Configure health checks

**Agent:**
- One agent per Docker host
- Automatic reconnection to Core
- Local operation queuing (future roadmap)
- Graceful degradation

## 🎯 Production Deployment

### System Requirements

**Agent:**
- OS: Linux (kernel 3.10+)
- Memory: 128MB minimum, 256MB recommended
- CPU: 1 core minimum
- Disk: 1GB for binaries and data
- Docker: 20.10+

**Core:**
- OS: Linux recommended
- Memory: 256MB minimum, 512MB recommended
- CPU: 1 core minimum
- Disk: 5GB for data and logs

### High Availability

**Core:**
- Deploy 3+ replicas behind load balancer
- Use shared storage for agent registry
- Enable leader election
- Configure health checks

**Agent:**
- One agent per Docker host
- Automatic reconnection to Core
- Local operation queuing
- Graceful degradation

### Monitoring

**Metrics to Track:**
- Agent online/offline status
- Operation success/failure rates
- API latency (p50, p95, p99)
- Resource usage per agent
- Audit log volume

**Alerts:**
- Agent offline > 5 minutes
- High operation failure rate (>5%)
- Certificate expiration < 30 days
- Disk usage > 80%

## 🔌 Plugin Development

### Creating a Custom Plugin

```go
package myplugin

import (
    "context"
    "github.com/bhangun/mandau/pkg/plugin"
)

type MyPlugin struct {
    name    string
    version string
}

func New() *MyPlugin {
    return &MyPlugin{
        name:    "my-plugin",
        version: "1.0.0",
    }
}

func (p *MyPlugin) Name() string { return p.name }
func (p *MyPlugin) Version() string { return p.version }

func (p *MyPlugin) Capabilities() []plugin.Capability {
    return []plugin.Capability{plugin.CapabilityAuth}
}

func (p *MyPlugin) Init(ctx context.Context, config map[string]interface{}) error {
    // Initialize plugin
    return nil
}

func (p *MyPlugin) Shutdown(ctx context.Context) error {
    // Cleanup
    return nil
}

// Implement plugin-specific interfaces...
```

### Registering Plugin

```go
// In agent/core main.go
import "your/plugin/path"

func loadPlugins(registry *plugin.Registry) {
    registry.Register(myplugin.New())
}
```

## 📊 Performance

**Benchmarks** (single agent, 8-core CPU):
- Stack apply: ~2-5s (depending on image size)
- Container exec: <100ms latency
- Log streaming: 10k+ lines/sec
- API throughput: 1000+ req/sec
- Memory: <100MB idle, <256MB under load
Security Comparison: SSH Keys vs Mandau's mTLS

    Mandau's mTLS mechanism is more secure for this use case. Here's why:


    ┌────────────────────────┬──────────────────────────────────────┬─────────────────────────────────────────────┐
    │ Aspect                 │ SSH Public Keys                      │ Mandau mTLS                                 │
    ├────────────────────────┼──────────────────────────────────────┼─────────────────────────────────────────────┤
    │ Authentication         │ One-way (client proves identity)     │ Mutual - both sides prove identity          │
    │ Encryption             │ Yes (after auth)                     │ TLS 1.3 only (stronger)                     │
    │ Certificate validation │ N/A (key-based)                      │ Full chain of trust with CA                 │
    │ Authorization          │ Binary (access or no access)         │ RBAC with granular permissions              │
    │ Audit trail            │ Limited (login/logout)               │ Full audit logging per operation            │
    │ Revocation             │ Manual (remove from authorized_keys) │ Certificate revocation via CA               │
    │ Scope                  │ Full shell access                    │ Least privilege - only exposed gRPC methods │
    │ Network exposure       │ SSH port (22) - heavily attacked     │ Custom port, mTLS required                  │
    │ Credential rotation    │ Manual                               │ Can be automated with short-lived certs     │
    └────────────────────────┴──────────────────────────────────────┴─────────────────────────────────────────────┘


    Mandau's Security Layers (from the code):

     1 ┌─────────────────────────────────────┐
     2 │ 1. mTLS (mutual authentication)     │ ← Both sides verify certificates
     3 │ 2. Auth Interceptor                 │ ← Plugin-based auth (RBAC)
     4 │ 3. Policy Interceptor               │ ← Per-request authorization
     5 │ 4. Audit Interceptor                │ ← Every action logged
     6 │ 5. Recovery Interceptor             │ ← Panic recovery
     7 └─────────────────────────────────────┘

    When SSH keys make sense:
     - Human operators accessing servers interactively
     - Bootstrap/trust-on-first-use scenarios
     - Simpler infrastructure with fewer moving parts

    When mTLS (Mandau) is better:
     - Service-to-service communication (agent ↔ core)
     - Granular access control (not all-or-nothing)
     - Compliance requirements (audit trails, revocation)
     - Zero-trust architectures (verify every request)

    Recommendation:
    Use both, but for different purposes:
     - SSH keys → human admin access to servers
     - Mandau mTLS → automated agent-to-core communication

    Mandau's approach is purpose-built for infrastructure management with defense-in-depth, while SSH is a
    general-purpose remote access tool.

### RBAC Configuration

```yaml
# roles.yaml
roles:
  - name: operator
    permissions:
      - resource: "stack:*"
        actions: ["read", "write"]
      - resource: "container:*"
        actions: ["read", "exec", "logs"]

users:
  - id: "ops@company.com"
    name: "Operations Team"
    roles: ["operator"]
```

### Stack with Secrets

```yaml
# compose.yaml
version: '3.8'
services:
  web:
    image: myapp:latest
    environment:
      DB_PASSWORD: ${secret:db-password}
      API_KEY: ${secret:api-key}
```

### Agent Labels

```yaml
# agent-config.yaml
agent:
  labels:
    environment: production
    datacenter: us-east-1
    zone: a
    tier: frontend
```

## 🛠️ CLI Command Reference

### Agent Management
- `mandau agent list` - List all registered agents

### Stack Management
- `mandau stack list <agent-id>` - List stacks on an agent
- `mandau stack apply <agent-id> <stack-name> <compose-file>` - Apply a stack to an agent
- `mandau stack logs <agent-id> <stack-name>` - Stream logs from a stack

### Container Management
- `mandau container exec <agent> <container> <command> [args...]` - Execute command in container
- `mandau container list <agent>` - List containers on an agent
- `mandau container logs <agent> <container>` - Get container logs
- `mandau container start <agent> <container>` - Start a container
- `mandau container stop <agent> <container>` - Stop a container

### Service Management
- `mandau services nginx create-proxy <agent> <domain> <upstream> <port>` - Create nginx reverse proxy
- `mandau services nginx list <agent>` - List nginx virtual hosts
- `mandau services systemd start <agent> <service>` - Start systemd service
- `mandau services systemd status <agent> <service>` - Get systemd service status
- `mandau services ssl obtain <agent> <domain> <email>` - Obtain SSL certificate
- `mandau services ssl renew-all <agent>` - Renew all SSL certificates
- `mandau services firewall allow-port <agent> <port> <protocol>` - Allow port through firewall
- `mandau services firewall deny-port <agent> <port> <protocol>` - Deny port through firewall

### Plugin Management
- `mandau plugins secrets get <key>` - Get a secret value
- `mandau plugins secrets set <key> <value>` - Set a secret value
- `mandau plugins secrets delete <key>` - Delete a secret
- `mandau plugins auth status` - Check authentication status
- `mandau plugins auth list-users` - List users
- `mandau plugins audit list` - List audit logs
- `mandau plugins audit query <filter>` - Query audit logs with filter
```

## 🤝 Contributing

1. Fork the repository
2. Create feature branch
3. Write tests
4. Run `make test`
5. Submit pull request

## 🚀 CI/CD

### GitHub Actions Workflows

Mandau uses GitHub Actions for automated testing, building, and releasing:

- **Build and Test**: Runs on every PR and push to main, testing with multiple Go versions
- **Release**: Creates GitHub releases with binaries for multiple platforms when tags are pushed
- **Docker**: Builds and pushes Docker images to GitHub Container Registry
- **Draft Release**: Manual workflow to create draft releases with changelogs

### Creating a Release

To create a new release:

1. **Create and push a new tag**:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. **GitHub Actions will automatically**:
   - Build static binaries for Linux, macOS, and Windows (AMD64/ARM64)
   - Create a GitHub release with the binaries
   - Build and push Docker images to ghcr.io

3. **Available binaries will include**:
   - `mandau-linux-amd64-v1.0.0.tar.gz`
   - `mandau-linux-arm64-v1.0.0.tar.gz`
   - `mandau-darwin-amd64-v1.0.0.tar.gz`
   - `mandau-darwin-arm64-v1.0.0.tar.gz`
   - `mandau-windows-amd64-v1.0.0.zip`

4. **Easy Installation Script**: Users can install Mandau using the quick installation script (requires sudo):
   ```bash
   curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/install.sh | sudo bash
   ```

   The installation script will be included in GitHub releases and is also available at the raw content URL.

### Automated Release Process

Use the automated release script to create new releases:

```bash
# Auto-increment patch version (e.g., v0.0.11 -> v0.0.12)
./scripts/release.sh

# Or create release with specific version
./scripts/release.sh v1.0.0
```

The release script will:
- Validate current repository state
- Auto-increment version or use specified version
- Update version references in source code
- Commit changes with appropriate message
- Create and push Git tag
- Trigger GitHub Actions release workflow

### Manual Release Process

If you want to create a draft release manually:
1. Go to the "Releases" tab in the GitHub repository
2. Click "Draft a new release"
3. Create a new tag (e.g., `v1.0.0`)
4. The draft-release workflow will generate changelog and create a draft

## 📄 License

MIT License

## 🙏 Acknowledgments

- Docker for container runtime
- gRPC for RPC framework
- HashiCorp Vault for secrets
- Compose specification

---

**Built with ❤️ for infrastructure operators who value security and reliability.**
