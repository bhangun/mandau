# 🗡️ Mandau Configuration Guide

## Overview

Mandau supports configuration through YAML files, with optional command-line flag overrides. The configuration system provides a flexible way to customize the behavior of both the core server and agents.

## Configuration File Locations

- **Core Configuration**: `config/core/config.yaml` (default) or specified with `--config` flag
- **Agent Configuration**: `config/agent/config.yaml` (default) or specified with `--config` flag

The configuration path can also be overridden using the `MANDAU_CONFIG_PATH` environment variable.

## Core Configuration

### Example `config/core/config.yaml`

```yaml
server:
  listen_addr: ":8443"
  tls:
    cert_path: "certs/core.crt"
    key_path: "certs/core.key"
    ca_path: "certs/ca.crt"
    min_version: "TLS1.3"
    server_name: "mandau-core"

plugins:
  enabled:
    rbac-auth: true
    file-audit: true
  configs:
    rbac-auth:
      roles: |
        roles:
          - name: admin
            permissions:
              - resource: "*"
                actions: ["*"]
          - name: operator
            permissions:
              - resource: "stack:*"
                actions: ["read", "write", "delete"]
              - resource: "container:*"
                actions: ["read", "exec", "logs"]
        users:
          - id: "admin@example.com"
            name: "Administrator"
            roles: ["admin"]

agent_management:
  heartbeat_interval: "30s"
  offline_timeout: "90s"
  auto_deregister: false

plugin_dir: "/usr/lib/mandau/plugins"
```

### Core Configuration Fields

- `server.listen_addr`: Address and port for the core server to listen on
- `server.tls.cert_path`: Path to the server certificate file
- `server.tls.key_path`: Path to the server private key file
- `server.tls.ca_path`: Path to the CA certificate file
- `server.tls.min_version`: Minimum TLS version (default: "TLS1.3")
- `server.tls.server_name`: Server name for certificate verification
- `plugins.enabled`: Map of plugin names to boolean values indicating if they should be loaded
- `plugins.configs`: Map of plugin-specific configurations
- `agent_management.heartbeat_interval`: How often agents should send heartbeats (duration string)
- `agent_management.offline_timeout`: How long to wait before marking an agent as offline (duration string)
- `agent_management.auto_deregister`: Whether to automatically remove offline agents
- `plugin_dir`: Directory where plugin binaries are located

## Agent Configuration

### Example `config/agent/config.yaml`

```yaml
agent:
  id: ""
  hostname: ""
  labels:
    environment: "production"
    datacenter: "dc1"
    zone: "us-east-1a"

server:
  listen_addr: ":8444"
  tls:
    cert_path: "certs/agent.crt"
    key_path: "certs/agent.key"
    ca_path: "certs/ca.crt"
    min_version: "TLS1.3"
    server_name: "mandau-agent"

server_connection:
  core_addr: "localhost:8443"
  tls:
    cert_path: "certs/agent.crt"
    key_path: "certs/agent.key"
    ca_path: "certs/ca.crt"
    min_version: "TLS1.3"
    server_name: "mandau-core"

docker:
  socket: "/var/run/docker.sock"
  api_version: "1.41"

stacks:
  root_dir: "./stacks"
  max_concurrent_operations: 5

plugins:
  enabled:
    rbac-auth: true
  configs:
    rbac-auth:
      roles: |
        roles:
          - name: admin
            permissions:
              - resource: "*"
                actions: ["*"]
        users:
          - id: "mandau-core"
            name: "Core Server"
            roles: ["admin"]

security:
  exec_timeout: "1h"
  log_retention: "30d"
  terminal_recording: true
```

### Agent Configuration Fields

- `agent.id`: Unique identifier for the agent (auto-generated if empty)
- `agent.hostname`: Hostname of the agent machine (auto-detected if empty)
- `agent.labels`: Key-value pairs for agent labeling and organization
- `server.listen_addr`: Address and port for the agent server to listen on
- `server.tls.cert_path`: Path to the agent certificate file
- `server.tls.key_path`: Path to the agent private key file
- `server.tls.ca_path`: Path to the CA certificate file
- `server.tls.min_version`: Minimum TLS version (default: "TLS1.3")
- `server.tls.server_name`: Server name for certificate verification
- `server_connection.core_addr`: Address of the core server to connect to
- `server_connection.tls`: TLS configuration for connecting to the core server
- `docker.socket`: Path to the Docker socket
- `docker.api_version`: Docker API version to use
- `stacks.root_dir`: Directory where stack files are stored
- `stacks.max_concurrent_operations`: Maximum number of concurrent stack operations
- `plugins.enabled`: Map of plugin names to boolean values indicating if they should be loaded
- `plugins.configs`: Map of plugin-specific configurations
- `security.exec_timeout`: Maximum time for container exec operations
- `security.log_retention`: How long to retain logs
- `security.terminal_recording`: Whether to record terminal sessions

## Command-Line Flag Precedence

Command-line flags take precedence over configuration file values:

```bash
# Use config file but override listen address
./bin/mandau-core --config config/core/config.yaml --listen :9000

# Use config file but override certificate paths
./bin/mandau-agent --config config/agent/config.yaml --cert /custom/agent.crt --key /custom/agent.key
```

## Environment Variables

- `MANDAU_CONFIG_PATH`: Override the default configuration file path

## Configuration Validation

The system validates configuration files at startup and will report errors for:
- Missing required certificate files
- Invalid TLS configuration
- Invalid duration strings
- Invalid port numbers

## Development vs Production

For development, you can use the default configuration files in the repository which use local paths and development settings.

For production, you should:
- Use absolute paths for certificates and data directories
- Set appropriate security settings
- Configure proper TLS certificates
- Adjust timeout and resource limits as needed