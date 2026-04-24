# Mandau CLI Reference Guide

Complete reference for all Mandau CLI commands with examples and use cases.

## Table of Contents

- [Getting Started](#getting-started)
- [Connection Management](#connection-management)
- [Stack Operations](#stack-operations)
- [Docker Commands](#docker-commands)
- [System Monitoring](#system-monitoring)
- [Service Management](#service-management)
- [Shell Access](#shell-access)
- [Filesystem Operations](#filesystem-operations)
- [Plugin Management](#plugin-management)
- [Certificate Management](#certificate-management)
- [Quick Reference Card](#quick-reference-card)

---

## Getting Started

### Installation

```bash
# Quick install (Linux/macOS)
curl -fsSL https://raw.githubusercontent.com/bhangun/mandau/main/scripts/install.sh | sudo bash

# Windows PowerShell
Invoke-WebRequest -Uri "https://github.com/bhangun/mandau/releases/latest/download/install.sh" -OutFile "install.sh"
bash install.sh
```

### First-Time Setup

```bash
# 1. Generate certificates
mandau cert gen

# 2. Connect to your server
mandau connect <server-ip>

# 3. Verify connection
mandau agent list
```

### Configuration

Mandau uses `~/.mandau/config.yaml` for configuration:

```yaml
server:
  listen_addr: "localhost:3443"
  tls:
    cert_path: "~/.mandau/certs/client.crt"
    key_path: "~/.mandau/certs/client.key"
    ca_path: "~/.mandau/certs/ca.crt"
```

**Auto-discovery:** Certificates in `~/.mandau/certs/` are automatically discovered.

---

## Connection Management

### mandau connect

Connect to a Mandau Core server and sync certificates.

```bash
# Connect to server
mandau connect 192.168.1.100

# Connect with custom port
mandau connect 192.168.1.100:9443
```

### mandau agent

Manage remote agents.

```bash
# List all agents
mandau agent list

# Output:
# ID                   HOSTNAME                       STATUS     LAST SEEN
# agent-insanserver    insanserver                    online     2026-04-16 04:12:50
```

---

## Stack Operations

### mandau apply (Recommended)

Deploy Docker Compose files with intelligent defaults.

```bash
# Deploy stack (defaults to 'up')
mandau apply docker-compose.yaml

# Deploy with flags
mandau apply docker-compose.yaml up -d

# Bring stack down
mandau apply docker-compose.yaml down

# Stop stack (without removing)
mandau apply docker-compose.yaml stop

# Start stopped stack
mandau apply docker-compose.yaml start

# Restart stack
mandau apply docker-compose.yaml restart

# View stack containers
mandau apply docker-compose.yaml ps

# View stack logs
mandau apply docker-compose.yaml logs -f

# Pull images only
mandau apply docker-compose.yaml pull

# Build images
mandau apply docker-compose.yaml build

# Deploy to specific agent
mandau -a agent-001 apply docker-compose.yaml
```

**Supported Actions:**
- `up` - Create and start containers (default)
- `down` - Stop and remove containers, networks, images
- `start` - Start existing containers
- `stop` - Stop running containers
- `restart` - Restart containers
- `pause` - Pause all processes
- `unpause` - Unpause all processes
- `ps` - List containers
- `logs` - Show logs
- `pull` - Pull images
- `build` - Build images
- `create` - Create without startinging
- `kill` - Force stop containers

### mandau stack

Explicit stack management with agent specification.

```bash
# List stacks on agent
mandau stack list agent-001

# Deploy stack
mandau stack up agent-001 mystack ./docker-compose.yaml

# Deploy with flags
mandau stack up agent-001 mystack ./docker-compose.yaml -d

# Remove stack
mandau stack down agent-001 mystack ./docker-compose.yaml

# Start/Stop
mandau stack start agent-001 mystack ./docker-compose.yaml
mandau stack stop agent-001 mystack ./docker-compose.yaml

# Restart
mandau stack restart agent-001 mystack ./docker-compose.yaml

# View containers
mandau stack ps agent-001 mystack ./docker-compose.yaml

# Stream logs
mandau stack logs agent-001 mystack ./docker-compose.yaml -f

# Pull/Build
mandau stack pull agent-001 mystack ./docker-compose.yaml
mandau stack build agent-001 mystack ./docker-compose.yaml
```

---

## Docker Commands

Full Docker command wrapper with 25+ subcommands.

### Container Management

```bash
# List containers
mandau docker ps
mandau docker list

# Stop containers
mandau docker stop container1 container2
mandau docker stop -t 30 mycontainer

# Start containers
mandau docker start container1 container2

# Restart containers
mandau docker restart container1
mandau docker restart -t 10 container1

# Pause/Unpause
mandau docker pause container1
mandau docker unpause container1

# Remove containers
mandau docker rm container1
mandau docker rm -f container1
mandau docker rm -v container1

# Kill containers
mandau docker kill container1
mandau docker kill --signal=SIGTERM container1
```

### Container Inspection

```bash
# View logs
mandau docker logs container1
mandau docker logs -f container1
mandau docker logs --tail 100 container1
mandau docker logs --since 2024-01-01 container1

# Inspect containers
mandau docker inspect container1
mandau docker inspect --format='{{.State.Status}}' container1

# Execute commands
mandau docker exec container1 ls
mandau docker exec -it container1 /bin/bash
mandau docker exec -e VAR=value container1 env

# View stats
mandau docker stats
mandau docker stats container1
mandau docker stats --no-stream
```

### Image Management

```bash
# List images
mandau docker images
mandau docker images -a
mandau docker images --filter "dangling=true"

# Pull images
mandau docker pull nginx
mandau docker pull nginx:latest
mandau docker pull myregistry.com/myimage:v1.0

# Push images
mandau docker push myimage
mandau docker push myregistry.com/myimage:v1.0

# Build images
mandau docker build .
mandau docker build -t myimage:v1.0 .
mandau docker build -f Dockerfile.prod .
```

### Network Management

```bash
# List networks
mandau docker network ls

# Create network
mandau docker network create mynetwork

# Inspect network
mandau docker network inspect mynetwork

# Remove network
mandau docker network rm mynetwork
```

### Volume Management

```bash
# List volumes
mandau docker volume ls

# Create volume
mandau docker volume create myvolume

# Inspect volume
mandau docker volume inspect myvolume

# Remove volume
mandau docker volume rm myvolume
```

### System Information

```bash
# Docker version
mandau docker version

# System info
mandau docker info

# Remove unused data
mandau docker system prune
mandau docker container prune
mandau docker image prune
```

---

## System Monitoring

### Quick Commands (Root Level)

Shortcuts for common system checks:

```bash
# Process list
mandau ps
mandau ps aux
mandau ps -ef

# Disk usage
mandau df
mandau df -hi

# Memory usage
mandau free
mandau free -m

# System uptime
mandau uptime
```

### mandau system

Comprehensive system monitoring commands.

```bash
# Comprehensive system info
mandau system info
mandau system info agent-001

# Process management
mandau system ps                    # Default: ps aux
mandau system ps aux
mandau system ps -ef

# Disk usage
mandau system df                    # Default: df -h
mandau system df -hi
mandau system du /var/log           # Directory usage
mandau system du / -h --max-depth=1

# Memory
mandau system free                  # Default: free -h
mandau system free -m

# Uptime
mandau system uptime

# Interactive tools
mandau system top                   # Interactive process viewer
mandau system htop                  # Requires htop installed

# User activity
mandau system who                   # Logged-in users
mandau system last                  # Recent logins
mandau system last -20

# Network
mandau system netstat               # Default: -tulpn
mandau system netstat -tulpn

# Log viewing
mandau system logs /var/log/syslog
mandau system logs /var/log/syslog -n 100
mandau system logs /var/log/syslog -f  # Interactive follow mode
```

**Examples:**

```bash
# Full system overview
mandau system info

# Output:
# 📊 System Information for agent: agent-insanserver
# 
# 🖥️  System Overview:
# Hostname: insanserver
# OS: Ubuntu 24.04.2 LTS
# Kernel: 6.11.0-26-generic
# Architecture: x86_64
# 
# 💻 CPU Information:
# CPU: Intel(R) Xeon(R) CPU E3-1240 v5 @ 3.50GHz
# Cores: 8
# 
# 🧠 Memory Usage:
#                total        used        free      shared  buff/cache   available
# Mem:            23Gi       5.9Gi       675Mi       112Mi        17Gi        17Gi
# Swap:          8.0Gi        41Mi       8.0Gi
# ...
```

---

## Service Management

### Nginx Management

```bash
# List nginx configurations
mandau services nginx list agent-001

# Create proxy
mandau services nginx create-proxy agent-001 example.com http://localhost:3000 80

# Reload nginx
mandau services nginx reload agent-001
```

### Systemd Service Management

```bash
# Start service
mandau services systemd start agent-001 myservice

# Stop service
mandau services systemd stop agent-001 myservice

# Restart service
mandau services systemd restart agent-001 myservice

# Check status
mandau services systemd status agent-001 myservice
```

### SSL Certificate Management

```bash
# Obtain SSL certificate
mandau services ssl obtain agent-001 example.com admin@example.com

# Renew all certificates
mandau services ssl renew-all agent-001
```

### Firewall Management

```bash
# Allow port
mandau services firewall allow-port agent-001 80 tcp

# Deny port
mandau services firewall deny-port agent-001 8080 tcp

# List rules
mandau services firewall list agent-001
```

### Cron Management

```bash
# Add cron job
mandau services cron add agent-001 backup-job "0 2 * * *" "/usr/local/bin/backup.sh"

# List cron jobs
mandau services cron list agent-001
```

---

## Shell Access

### mandau shell

Open interactive shell on remote agent (like SSH but via mTLS).

```bash
# Open shell on default agent
mandau shell

# Open shell on specific agent
mandau shell agent-001
```

**Features:**
- ✅ Automatic terminal resize (SIGWINCH handling)
- ✅ Full TTY support
- ✅ Color output preserved
- ✅ Ctrl+C handling

**Exit:** Type `exit` or press `Ctrl+D`

---

## Filesystem Operations

### mandau fs

Remote filesystem management.

```bash
# List files
mandau fs ls /var/log
mandau fs ls /var/log --long

# View file
mandau fs cat /etc/hosts

# Copy to local
mandau fs fetch /var/log/syslog ./local-syslog

# Copy to remote
mandau fs cp ./config.yaml /etc/myapp/config.yaml

# Create directory
mandau fs mkdir /var/log/myapp

# Remove file
mandau fs rm /tmp/old-file.log
```

---

## Plugin Management

### Authentication

```bash
# Check auth status
mandau plugins auth status

# List users
mandau plugins auth list-users
```

### Secrets Management

```bash
# Get secret
mandau plugins secrets get my-secret

# Set secret
mandau plugins secrets set my-secret "my-value"

# Delete secret
mandau plugins secrets delete my-secret
```

### Audit Log

```bash
# List audit logs
mandau plugins audit list

# Query logs
mandau plugins audit query "agent:agent-001"
```

---

## Certificate Management

### mandau cert

Certificate generation and management.

```bash
# Generate certificates
mandau cert gen

# View certificate info
mandau cert info
```

---

## Quick Reference Card

### Most Common Commands

```bash
# Connect & Check
mandau connect <server>
mandau agent list

# Deploy Stack
mandau apply docker-compose.yaml
mandau apply docker-compose.yaml up -d
mandau apply docker-compose.yaml down

# Monitor
mandau ps
mandau df
mandau free
mandau uptime
mandau system info

# Docker
mandau docker ps
mandau docker logs -f container
mandau docker exec -it container bash

# Shell
mandau shell

# Services
mandau services nginx reload agent-001
mandau services systemd restart agent-001 myservice

# Files
mandau fs ls /var/log
mandau fs cat /etc/hosts
```

### Global Flags

```bash
-a, --agent string             Target agent ID
--server string                Core server address (default: localhost:3443)
--cert string                  Client certificate path
--key string                   Client key path
--ca string                    CA certificate path
-h, --help                     Help for any command
```

### Environment Variables

```bash
MANDAU_SERVER=localhost:3443
MANDAU_CERT=~/.mandau/certs/client.crt
MANDAU_KEY=~/.mandau/certs/client.key
MANDAU_CA=~/.mandau/certs/ca.crt
MANDAU_AGENT=agent-001  # Set default agent
```

---

## Tips & Best Practices

### 1. Use Auto-Discovery

Let Mandau auto-discover certificates and default agent:

```bash
# Generate once
mandau cert gen

# Use without flags
mandau agent list
mandau docker ps
```

### 2. Stack Deployments

Use `mandau apply` for quick deployments:

```bash
# Simple and clean
mandau apply docker-compose.yaml
mandau apply docker-compose.yaml down
```

### 3. System Checks

Quick health checks:

```bash
# Full overview
mandau system info

# Quick checks
mandau uptime
mandau df -h
mandau free -h
```

### 4. Interactive Sessions

For interactive tools, use shell:

```bash
mandau shell
# Then run: top, htop, vim, etc.
```

### 5. Log Streaming

Follow logs in real-time:

```bash
# Container logs
mandau docker logs -f container

# Stack logs
mandau stack logs agent-001 mystack ./docker-compose.yaml -f

# System logs
mandau system logs /var/log/syslog -f
```

---

## Troubleshooting

### Connection Issues

```bash
# Check if server is running
mandau connect <server>

# Verify certificates
mandau cert info

# List agents
mandau agent list
```

### Common Errors

**"tls: bad certificate"**
```bash
# Regenerate certificates
mandau cert gen

# Restart services
sudo systemctl restart mandau-core mandau-agent
```

**"no agent specified"**
```bash
# Set default agent
mandau -a agent-001 docker ps

# Or set env
export MANDAU_AGENT=agent-001
```

**"stream error: EOF"**
This is normal after successful operations. The command completed successfully.

---

## See Also

- [Quick Start Guide](guide/QUICKSTART.md)
- [Certificate Management](guide/CERTIFICATE_MANAGEMENT.md)
- [Installation Guide](guide/INSTALLATION_GUIDE.md)
- [Configuration Reference](../docs/development/CONFIGURATION.md)
