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

## 🏗️ Architecture

```
┌──────────────────────┐
│   UI Clients         │
│  - Flutter Desktop   │
│  - CLI               │
│  - Web (optional)    │
└──────────┬───────────┘
           │
           │ mTLS/gRPC
           ▼
┌──────────────────────┐
│   Mandau Core        │
│  - Agent registry    │
│  - Auth & RBAC       │
│  - Audit logging     │
│  - Policy engine     │
└──────────┬───────────┘
           │
           │ mTLS/gRPC
           ▼
┌──────────────────────┐
│   Mandau Agent       │
│  - Docker control    │
│  - Stack management  │
│  - File system       │
│  - Container exec    │
└──────────────────────┘
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

## 🚀 Quick Start

### 1. Build

```bash
make build
```

### 2. Generate Certificates

```bash
make certs
```

### 3. Run with Enhanced Reliability (Recommended)

For development with automatic restarts and connection recovery, use the enhanced runner:

```bash
# Clean up any stale processes first
./run-dev.sh --clean

# Start the system with enhanced reliability features (default behavior)
./run-dev.sh
# or explicitly
./run-dev.sh --host
```

Or run manually with proper process management:

### 3a. Run Core

```bash
./bin/mandau-core \
  --listen :8443 \
  --cert ./certs/core.crt \
  --key ./certs/core.key \
  --ca ./certs/ca.crt
```

### 3b. Run Agent

```bash
./bin/mandau-agent \
  --server localhost:8443 \
  --cert ./certs/agent.crt \
  --key ./certs/agent.key \
  --ca ./certs/ca.crt \
  --stack-root ./stacks
```

### 4. Use CLI

```bash
# Option 1: Using environment variables
export MANDAU_SERVER=localhost:8443
export MANDAU_CERT=./certs/client.crt
export MANDAU_KEY=./certs/client.key
export MANDAU_CA=./certs/ca.crt

# List agents
./bin/mandau agent list

# Deploy a stack
./bin/mandau stack apply agent-001 web ./compose.yaml

# Stream logs
./bin/mandau stack logs agent-001 web

# Execute command in container
./bin/mandau container exec agent-001 web-container /bin/sh

# Manage services
./bin/mandau services nginx create-proxy agent-001 example.com http://localhost:3000 80
./bin/mandau services systemd start agent-001 myservice
./bin/mandau services firewall allow-port agent-001 80 tcp

# Manage plugins
./bin/mandau plugins secrets get my-secret
./bin/mandau plugins auth status
```

```bash
# Option 2: Using command-line flags
./bin/mandau --server localhost:8443 --cert ./certs/client.crt --key ./certs/client.key --ca ./certs/ca.crt agent list

# Deploy a stack
./bin/mandau --cert ./certs/client.crt --key ./certs/client.key --ca ./certs/ca.crt stack apply agent-001 web ./compose.yaml

# Stream logs
./bin/mandau --cert ./certs/client.crt --key ./certs/client.key --ca ./certs/ca.crt stack logs agent-001 web

# Execute command in container
./bin/mandau --cert ./certs/client.crt --key ./certs/client.key --ca ./certs/ca.crt container exec agent-001 web-container /bin/sh

# Manage services
./bin/mandau --cert ./certs/client.crt --key ./certs/client.key --ca ./certs/ca.crt services nginx create-proxy agent-001 example.com http://localhost:3000 80
./bin/mandau --cert ./certs/client.crt --key ./certs/client.key --ca ./certs/ca.crt services systemd start agent-001 myservice
./bin/mandau --cert ./certs/client.crt --key ./certs/client.key --ca ./certs/ca.crt services firewall allow-port agent-001 80 tcp

# Manage plugins
./bin/mandau --cert ./certs/client.crt --key ./certs/client.key --ca ./certs/ca.crt plugins secrets get my-secret
./bin/mandau --cert ./certs/client.crt --key ./certs/client.key --ca ./certs/ca.crt plugins auth status
```

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

## 🛣️ Roadmap

**Phase 1: Core (Current)**
- ✅ Agent & Core implementation
- ✅ Stack management
- ✅ Plugin system
- ✅ Security interceptors
- ✅ Audit logging

**Phase 2: Advanced Features**
- [ ] Multi-architecture support (ARM64)
- [ ] Image registry management
- [ ] Volume management
- [ ] Network management
- [ ] Backup/restore operations
- [ ] Rolling updates

**Phase 3: UI & Integrations**
- [ ] Flutter desktop application
- [ ] Web UI (optional)
- [ ] Prometheus metrics exporter
- [ ] Grafana dashboards
- [ ] Slack/Discord notifications
- [ ] Git-based stack sync

**Phase 4: Enterprise**
- [ ] Multi-tenancy
- [ ] Cost tracking
- [ ] Compliance reporting
- [ ] Advanced policy engine
- [ ] SSO integration (OIDC)

## 📝 Configuration Examples

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

### Manual Release Trigger

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
